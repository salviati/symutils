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
  TODO(utkan):
	* Revise log levels for Printf() calls
	* Implement ignoreChars option
*/

/*
	symfix(1) finds and (somewhat interactively) repairs broken symlinks.

	symfix [options] file1/dir1 [file2/path2 ...]

	Note: filename variable refers to file's full path in the code.
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	. "symutils/common"
	"symutils/fuzzy"
	"symutils/locate"
	"time"
)

const (
	DBFILES = "/var/lib/mlocate/mlocate.db"
)

var (
	automatedMode     = flag.Bool("A", false, "Automated mode: do not proceed if there's no certain way of fixing the symlink")
	basenameMustMatch = flag.Bool("B", false, "Basename for candidates must precisely match input's")
	stripPath         = flag.Bool("b", true, "Match only the basename part of files, stripping the path")
	deleteDeadLinks   = flag.Bool("d", false, "Delete dead links (i.e., links with no matches)")
	existing          = flag.Bool("e", false, "Check existence of target candidates before listing (handy for slocate users)")
	showHelp          = flag.Bool("h", false, "Display help and quit")
	recurse           = flag.Bool("r", false, "Recurse into directories")
	symlinkCandidates = flag.Bool("s", false, "Include symlinks in possible target list")
	showVersion       = flag.Bool("version", false, "Show version and license info and quit")
	stripExtension    = flag.Bool("x", false, "Strip extension of the file when doing the search")
	ignoreCase        = flag.Bool("i", false, "Ignore case")
	limit             = flag.Uint("l", 0, "Limit the number of listed entries, zero means no limit.")
	root              = flag.String("root", "/", "Only files under root will be searched.")
	yesToAll          = flag.Bool("Y", false, "Assume yes to all y/n questions (they appear before making changes in the filesystem)")
	dbPath            = flag.String("D", DBFILES, "List of database file paths, separator character is : under Unix, see path/filepath/ListSeparator for other OSes.")
	levenshteinParams = flag.String("levenshtein", "", "Levenshtein parameters. Parameter format is ThresholdLevensteinDistance,DelCost,InsCost,SubsCost all integers")
	renameSymlink     = flag.Bool("rename", false, "If the symlink filename does not match with the target file's name, rename it to match it with target. Must be used with -names option.")
	matchNames        = flag.Bool("names", false, "Consider symlinks with a name that does not match with it's target as broken")
	showSummary       = flag.Bool("summary", false, "Show a summary with misc info")
	ignoreChars       = flag.String("ignore", "", "Ignore the given set of characters in file names")
	filter            = flag.String("filter", "", `Filter search results using regexp.MatchString. Separate filters with a newline (\n). If the first character of the filter is !, those that match with the regexp are _not_ listed.`)
	verbose           = flag.Uint("v", 0, "Verbosity 0: errors only, 1: errors and warnings, 2: errors, warning, log")
	replaceFile       = flag.String("replace", "", "Name of the file containing replacement rules. To be documented here, for now see replace.go for details.")

	searchMethod = flag.String("m", "hashmap",
		"Comma separated list of search methods: hashmap (exact matches [except for -x and -i options], very fast. Requires a hash-map initialization on first usage.), substring (using strings.Contains), wildcard (using path.Match), regexp, levenshtein (fuzzy search, see -levenshtein option as well). Search will be repeated using the next method if the current method gives 0 hits.")
	nworkers = flag.Uint("nworkers", 1, "The number of parallel workers searching in one database")
)

var (
	repaired, deleted, skipped, dead int
	db                               *locate.DB
	fileNames                        []string

	missingTargets map[string]bool
	brokenLinks    map[string]string

	filterIn  []*regexp.Regexp // Entries that match all these filters will be listed
	filterOut []*regexp.Regexp // Entries that do not match any of these filters will be listed
	
	replacer *Replacer
)

var (
	ErrUserCancel = errors.New("user cancel")
	ErrCircular   = errors.New("circular symlink")
)

const (
	pkg, version, author, about, usage string = "symfix", VERSION, "Utkan Güngördü",
		"symfix(1) finds and (somewhat interactively) repairs broken symlinks.\nIt uses locate(1) to look up broken symlinks.",
		"symfix [options] file1/dir1 [file2/path2 ...]"
)

func okay(format string, va ...interface{}) bool {
	if *yesToAll {
		return true
	}
	return Queryf(format, va...)
}

/* Unlinks the file name. If the target is not an empty string,
   also creates the symlink name (or if renameSymlink option is
   used, filepath.Base(target)) pointing to it.
   Depending on the options, the function may expect user-interaction
   to confirm the action. */
func relink(name, target string) error {
	newname := name
	if *renameSymlink {
		newname = filepath.Base(name)
	}

	if target == newname {
		Warnf("Symlink shouldn't be pointing to itself!\n")
		return ErrCircular
	}

	if (target == "" && false == okay("Really unlink the file?: %s", name)) ||
		false == okay("Really relink the file?: %s -> %s", newname, target) {
		return ErrUserCancel
	}

	err := os.Remove(name)
	if err != nil {
		Warnf("%v\n", nil)
		return nil
	}

	if target == "" {
		Logf("unlinked %v\n", name)
		deleted++
		return nil
	}

	err = os.Symlink(target, newname)
	if err != nil {
		Warnf("%v\n", err)
	}
	Printf(LOG, "created symlink: %v -> %v\n", newname, target)
	repaired++
	return nil
}

// Check filename against given filters.
func filterResult(filename string) bool {
	Logf("Testing %s against filters", filename)
	for _, f := range filterIn {
		if f.MatchString(filename) == false {
			return false
		}
	}

	for _, f := range filterOut {
		if f.MatchString(filename) == true {
			return false
		}
	}

	Logf("%s successfully passed through all filters", filename)
	return true
}

func mylocate(db *locate.DB, pattern string) (matches []string, err error) {
	for _, method := range strings.Split(*searchMethod, ",") {
		t0 := time.Now()
		matches, err = locate.LocateAll(db, method, pattern)
		t1 := time.Now()
		Logf("locate %s using %s %.3f seconds-->\n", pattern, method, float64(t1.Sub(t0))/1e9)
		if len(matches) > 0 {
			return matches, err
		}
	}
	return matches, err
}

/* Fixes a given single symlink.
   Returns error code. */
func symfix(path string) error {
	ok, dst, _ := LinkAlive(path, *matchNames)
	if ok {
		Logf("%v -> %v\n", path, dst)
		return nil
	}

	Logf("%v -> %v (broken)\n", path, dst)

	// locate does not store trailing / for directories, so we gotta strip it off as well.
	if dst[len(dst)-1] == '/' {
		dst = dst[0 : len(dst)-1]
	}
	_, pattern := filepath.Split(dst)

	// look up possible targets using locate
	matches, err := mylocate(db, pattern)

	if err != nil {
		Errorf("%v\n", err)
	}

	for _, f := range matches {
		Logf("Found candidate: %v\n", f)
	}

	if len(matches) == 0 {
		Logf("No matches for: %v\n", path)
		if replacer != nil {
			Logf("Using replacement rules to get targets\n", path)
			matches = replacer.Replace(path)
		}
	}

	if len(matches) == 0 {
		if *showSummary {
			missingTargets[dst] = true
			brokenLinks[path] = dst
		}

		dead++
		if *deleteDeadLinks {
			return relink(path, "")
		}
		return nil
	}

	if len(matches) == 1 {
		return relink(path, matches[0])
	}

	// we have more than 1 match
	if *automatedMode {
		Logf("Automated mode, skipping results\n")
		skipped++
		return nil
	}

	choice, cancel := Choose("(Fixing: " + path + " -> " + dst + ")\nWhich one seems to be the correct target?", matches)
	if cancel {
		return ErrUserCancel
	} //user cancel
	return relink(path, matches[choice])

	return nil
}

func WalkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		Warnf("%v\n", err)
		return err
	}

	if info.IsDir() {
		if *recurse == false {
			return filepath.SkipDir
		}
		return nil
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if err := symfix(path); err == ErrUserCancel {
			skipped++
		}
	}

	return nil
}

/* Repair (recursively) symlink(s) */
func symfixr(filename string) {
	_, err := os.Stat(filename)
	if err != nil {
		Warnf("%v\n", err)
		return
	}

	filepath.Walk(filename, WalkFunc)
}

func init() {
	var err error
	

	missingTargets = make(map[string]bool)
	brokenLinks = make(map[string]string)

	flag.Parse()

	SetLogLevel(*verbose)

	if *showVersion {
		PrintVersion(pkg, version, author)
		os.Exit(0)
	}
	if *showHelp || flag.NArg() == 0 {
		PrintHelp(pkg, version, about, usage)
		os.Exit(0)
	}

	Logf("It's recommended that you update your database files by updatedb(8) prior to execution.\n")

	if *filter != "" {
		filters := strings.Split(*filter, "\n")
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
		if err != nil {
			log.Fatal(err)
		}
		if n != 4 {
			Errorf("Invalid number of fields for fuzzy search parameter.\n")
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
		Root:                 *root,
	}
	
	if *replaceFile != "" {
		replacer, err = NewReplacer(*replaceFile)
		if err != nil {
			log.Fatal(err)
		}
		Logf("Read replacement rules:")
		for _, r := range replacer.rules {
			Logln("*", r)
		}
	}

	Logf("Reading databases...\n")
	timeStart := time.Now()
	db, err = locate.NewDB(filepath.SplitList(*dbPath), &options)
	timeEnd := time.Now()
	Logf("Done. Took %.2f seconds.\n", float64(timeEnd.Sub(timeStart))/1e9)
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
