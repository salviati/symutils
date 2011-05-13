/*
   Copyright (c) Utkan Güngördü <utkan@freeconsole.org>

   This program is free software; you can redistribute it and/or modify
   it under the terms of the GNU General Public License as
   published by the Free Software Foundation; either version 3 or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of

   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the

   GNU General Public License for more details


   You should have received a copy of the GNU General Public
   License along with this program; if not, write to the
   Free Software Foundation, Inc.,
   51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
*/

/*
  TODO(salviati):
	* Revise log levels for vprintf() calls
	* Implement ignoreChars option
*/

/*
	symfix(1) finds and (somewhat interactively) repairs broken symlinks.

	symfix [options] file1/dir1 [file2/path2 ...]

	Note: filename variable refers to file's full path in the code.
*/
package main

import (
	"flag"
	"fmt"
	"strings"
	"os"
	"path/filepath"
	"log"
	"time"
	"syscall"
	"regexp"
	"symutils/fuzzy"
	"symutils/locate"
)

const (
	DBFILES = "/var/lib/mlocate/mlocate.db"
)

var automatedMode = flag.Bool("A", false, "Automated mode: do not proceed if there's no certain way of fixing the symlink")
var basenameMustMatch = flag.Bool("B", false, "Basename for candidates must precisely match input's")
var stripPath = flag.Bool("b", true, "Match only the basename part of files, stripping the path")
var deleteDeadLinks = flag.Bool("d", false, "Delete dead links (i.e., links with no matches)")
var existing = flag.Bool("e", false, "Check existence of target candidates before listing (handy for slocate users)")
var showHelp = flag.Bool("h", false, "Display help and quit")
var recurse = flag.Bool("r", false, "Recurse into directories")
var symlinkCandidates = flag.Bool("s", false, "Include symlinks in possible target list")
var showVersion = flag.Bool("version", false, "Show version and license info and quit")
var stripExtension = flag.Bool("x", false, "Strip extension of the file when doing the search")
var ignoreCase = flag.Bool("i", false, "Ignore case")
var limit = flag.Uint("l", 0, "Limit the number of listed entries, zero means no limit.")
var yesToAll = flag.Bool("Y", false, "Assume yes to all y/n questions (they appear before making changes in the filesystem)")
var dbPath = flag.String("D", DBFILES, "List of database file paths, separator character is : under Unix, see path/filepath/ListSeparator for other OSes.")
var levenshteinParams = flag.String("levenshtein", "", "Levenshtein parameters. Parameter format is ThresholdLevensteinDistance,DelCost,InsCost,SubsCost all integers")
var renameSymlink = flag.Bool("rename", false, "If the symlink filename does not match with the target file's name, rename it to match it with target. Must be used with -names option.")
var matchNames = flag.Bool("names", false, "Consider symlinks with a name that does not match with it's target as broken")
var showSummary = flag.Bool("summary", false, "Show a summary with misc info")
var ignoreChars = flag.String("ignore", "", "Ignore the given set of characters in file names")
var filter = flag.String("filter", "", `Filter search results using regexp.MatchString. Separate filters with a newline (\n). If the first character of the filter is !, those that match with the regexp are _not_ listed.`)

var searchMethod = flag.String("m", "hashmap",
	"Comma separated list of search methods: hashmap (exact matches [except for -x and -i options], very fast. Requires a hash-map initialization on first usage.), substring (using strings.Contains), wildcard (using path.Match), regexp (using sre2), levenshtein (fuzzy search, see -levenshtein option as well). Search will be repeated using the next method if the current method gives 0 hits.")
var nworkers = flag.Uint("nworkers", 1, "The number of parallel workers searching in one database")

var repaired, deleted, skipped, dead int
var db *locate.DB
var fileNames []string

var missingTargets map[string]bool
var brokenLinks map[string]string

var filterIn []*regexp.Regexp  // Only entries that match all these filters will be listed
var filterOut []*regexp.Regexp // Only entries that do not match any of these filters will be listed

const (
	pkg, version, author, about, usage string = "symfix", VERSION, "Utkan Güngördü",
		"symfix(1) finds and (somewhat interactively) repairs broken symlinks.\nIt uses locate(1) to look up broken symlinks.",
		"symfix [options] file1/dir1 [file2/path2 ...]"
)

/* Unlinks the file name. If the target is not an empty string,
   also creates the symlink name (or if renameSymlink option is
   used, filepath.Base(target)) pointing to it.
   Depending on the options, the function may expect user-interaction
   to confirm the action. */
func relink(name, target string) (e int) {
	newname := name
	if *renameSymlink {
		newname = filepath.Base(name)
	}

	if target == newname {
		vprintf(WARN, "Symlink shouldn't be pointing to itself!\n")
		return -1
	}

	if (target == "" && false == ynQuestion("Really unlink the file?: %s", name)) ||
		false == ynQuestion("Really relink the file?: %s -> %s", newname, target) {
		return 3
	}

	e = syscall.Unlink(name)
	if e != 0 {
		vprintf(WARN, "%v\n", os.PathError{"unlink", name, os.Errno(e)})
		return
	}

	if target == "" {
		vprintf(INFO, "unlinked %v\n", name)
		deleted++
		return
	}

	e = syscall.Symlink(target, newname)
	if e != 0 {
		vprintf(WARN, "%v\n", os.PathError{"symlink", newname, os.Errno(e)})
	}
	vprintf(INFO, "created symlink: %v -> %v\n", newname, target)
	repaired++
	return 0
}

// Filters a give file name.
func filterResult(name string) bool {
	vprintf(LOG, "Testing %s against filters", name)
	for _, f := range filterIn {
		if f.MatchString(name) == false {
			return false
		}
	}

	for _, f := range filterOut {
		if f.MatchString(name) == true {
			return false
		}
	}

	vprintf(LOG, "%s successfully passed through all filters", name)
	return true
}

func mylocate(db *locate.DB, pattern string) (matches []string, err os.Error) {
	for _, method := range strings.Split(*searchMethod, ",", -1) {
		t0 := time.Nanoseconds()
		matches, err = locate.LocateAll(db, method, pattern)
		t1 := time.Nanoseconds()
		vprintf(LOG, "locate %s using %s %.3f seconds-->\n", pattern, method, float64(t1-t0)/1e9)
		if len(matches) > 0 { return matches, err }
	}
	return matches, err
}

/* Fixes a given single symlink.
   Returns error code. */
func symfix(filename string) (e int) {
	ok, dst, _ := linkAlive(filename, *matchNames)
	if ok {
		vprintf(LOG, "%v -> %v\n", filename, dst)
		return 0
	}

	vprintf(INFO, "%v -> %v (broken)\n", filename, dst)

	// locate does not store trailing / for directories, so we gotta stip it off as well.
	if dst[len(dst)-1] == '/' {
		dst = dst[0 : len(dst)-1]
	}
	_, pattern := filepath.Split(dst)

	matches, err := mylocate(db, pattern)

	if err != nil {
		vprintf(ERR, "%v\n", err)
	}

	for _, f := range matches {
		vprintf(LOG, "Found candidate: %v\n", f)
	}

	if len(matches) == 0 {
		vprintf(INFO, "No matches for: %v\n", filename)
	}

	if len(matches) == 0 {
		if *showSummary {
			missingTargets[dst] = true
			brokenLinks[filename] = dst
		}

		dead++
		if *deleteDeadLinks {
			return relink(filename, "")
		}
		return 0
	}

	if len(matches) == 1 {
		return relink(filename, matches[0])
	}

	// we have more than 1 match
	if *automatedMode {
		vprintf(INFO, "Automated mode, skipping results\n")
		skipped++
		return 0
	}

	choice := getInteractiveChoice(matches)
	if choice == -1 {
		return 3
	} //user cancel
	return relink(filename, matches[choice])

	return 0
}

type walkEnt struct{}

func (*walkEnt) VisitDir(filename string, d *os.FileInfo) bool {
	return *recurse
}

func (*walkEnt) VisitFile(filename string, d *os.FileInfo) {
	if d.IsSymlink() {
		e := symfix(filename)
		if e == 3 {
			skipped++
		}
	}
}

/* Repair (recursively) symlink(s) */
func symfixr(filename string) {
	_, err := os.Stat(filename)
	if err != nil {
		vprintf(WARN, "%v\n", err)
		return
	}

	v := new(walkEnt)
	ech := make(chan os.Error)
	go func() { filepath.Walk(filename, v, ech); close(ech) }()
	for e := range ech {
		vprintf(WARN, "%v\n", e)
	}
}

func init() {
	missingTargets = make(map[string]bool)
	brokenLinks = make(map[string]string)

	flag.Parse()

	if *showVersion {
		printVersion(pkg, version, author)
		os.Exit(0)
	}
	if *showHelp || flag.NArg() == 0 {
		printHelp(pkg, version, about, usage)
		os.Exit(0)
	}

	vprintf(INFO, "It's recommended that you update your database files by updatedb(8) prior to execution.\n")


	if *filter != "" {
		filters := strings.Split(*filter, "\n", -1)
		filterIn = make([]*regexp.Regexp, 0, len(filters))
		filterOut = make([]*regexp.Regexp, 0, len(filters))

		for _, f := range filters {
			if f[0] == '!' {
				filterOut = append(filterOut, regexp.MustCompile(f[1:]))
			} else {
				filterIn = append(filterIn, regexp.MustCompile(f))
			}
		}
	}

	var fuzzyCost fuzzy.LevenshteinCost
	var fuzzyThreshold int

	if *levenshteinParams != "" {
		n, err := fmt.Sscanf(*levenshteinParams, "%d,%d,%d,%d", &fuzzyThreshold, &fuzzyCost.Del, &fuzzyCost.Ins, &fuzzyCost.Subs)
		if err != nil { log.Fatal(err) }
		if n != 4 {
			vprintf(ERR, "Invalid number of fields for fuzzy search parameter.\n")
		}
	}

	options := locate.Options{
		IgnoreCase:           *ignoreCase,
		MaxMatches:           *limit,
		StripExtension:       *stripExtension,
		Basename:             *basenameMustMatch,
		StripPath:            *stripPath,
		Existing:             *existing, // We handle this manually, after getting the list of matches.
		Symlink:              *symlinkCandidates,
		HashMap:              strings.Contains(*searchMethod, "hashmap"),
		LevenshteinCost:      fuzzyCost,
		LevenshteinThreshold: fuzzyThreshold,
		NWorkers:             *nworkers,
	}

	var err os.Error
	vprintf(INFO, "Reading databases...\n")
	timeStart := time.Nanoseconds()
	db, err = locate.NewDB(filepath.SplitList(*dbPath), &options)
	timeEnd := time.Nanoseconds()
	vprintf(INFO, "Done. Took %.2f seconds.\n", float64(timeEnd-timeStart)/1e9)
	if err != nil {
		log.Fatal(err)
	}
}

func summary() {
	sprintf := func(format string, va ...interface{}) { fmt.Fprintf(os.Stderr, "[SUMMARY] "+format, va...) }
	sprintf("Repaired: %d, deleted: %d, skipped: %d, dead: %d\n", repaired, deleted, skipped, dead)
	sprintf("List of missing targets (%d items):\n", len(missingTargets))
	for target, _ := range missingTargets {
		sprintf("%s\n", target)
	}

	sprintf("List of broken links (%d items):\n", len(brokenLinks))
	for name, dst := range brokenLinks {
		sprintf("%s -> %s\n", name, dst)
	}
}

func main() {
	for _, f := range flag.Args() {
		symfixr(f)
	}

	if *showSummary {
		summary()
	}
}
