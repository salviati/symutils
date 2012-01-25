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

// xlocate(1) is a feature-rich, parallel alternative to locate.
// Common options are (mostly) compatible with GNU locate.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"symutils/fuzzy"
	"symutils/locate"
	"text/template"
	"time"
)

const (
	DBFILES = "/var/lib/mlocate/mlocate.db"
)

var stripPath = flag.Bool("b", false, "Match only the basename part of files, stripping the path.")
var countEntries = flag.Bool("c", false, "Write out the number of matching entries and quit.")
var dbFiles = flag.String("d", DBFILES, "List of : separated database files. xlocate will try to determine the format automatically.")
var existing = flag.Bool("e", false, "List only existing files.")
var follow = flag.Bool("f", false, "Follow symlinks when checking for existence.")
var ignoreCase = flag.Bool("i", false, "Ignore case.")
var showHelp = flag.Bool("h", false, "Display help and quit")
var limit = flag.Uint("l", 0, "Limit the number of listed entries, zero means no limit.")

var accessable = flag.Bool("a", true, "List only (read-)accessable files (disabling this option requires RO access to all given DB files)")

var levenshteinParams = flag.String("levenshtein", "", "Levenshtein parameters. Parameter format is ThresholdLevensteinDistance,DelCost,InsCost,SubsCost all integers")
var searchMethod = flag.String("m", "hashmap,substring",
	"Comma separated list of search methods: hashmap (exact matches [except for -x and -i options], very fast. Requires a hash-map initialization on first usage.), substring (using strings.Contains), wildcard (using filepath.Match), regexp, levenshtein (fuzzy search, see -levenshtein option as well). Search will be repeated using the next method if the current method gives 0 hits.")
var nworkers = flag.Uint("nworkers", 1, "The number of parallel workers searching in one database")
var stripExtension = flag.Bool("E", false, "Ignore file extension. (For definition of extension, see Go's package documentation on filepath.Ext)")
var basenameMustMatch = flag.Bool("B", false, "Basename must match (this's slightly different than the GNU Locate's -b option).")

var symlinkCandidates = flag.Bool("s", true, "List symlinks") //FIXME: What about S in GNU locate?
var showVersion = flag.Bool("V", false, "Display version and licensing information, and quit.")
var httpAddr = flag.String("http", "", "HTTP service address (eg. ':9188')")
var templateString = flag.String("template", "{N}. <a href=\"file://{{Path}}\">{{Base}}</a><br>", "Template for HTTP results")

var db *locate.DB
var tpl *template.Template

const (
	pkg, version, author, about, usage string = "xlocate", VERSION, "Utkan Güngördü",
		"xlocate(1) A feature-richer, parallel alternative to locate(1).",
		"xlocate [options] pattern"
)

func init() {
	flag.Parse()

	daemonMode := *httpAddr != ""

	if *showHelp || (!daemonMode && flag.NArg() == 0) {
		printHelp(pkg, version, about, usage)
		os.Exit(0)
	}

	var fuzzyCost fuzzy.LevenshteinCost
	var fuzzyThreshold int

	if *levenshteinParams != "" {
		n, err := fmt.Sscanf(*levenshteinParams, "%d,%d,%d,%d", &fuzzyThreshold, &fuzzyCost.Del, &fuzzyCost.Ins, &fuzzyCost.Subs)
		if err != nil {
			log.Fatal(err)
		}
		if n != 4 {
			vprintf(ERR, "Invalid number of fields for fuzzy search parameter.\n")
		}
	}

	tpl = template.Must(template.New("result").Parse(*templateString+"\n"))

	options := locate.Options{
		IgnoreCase:           *ignoreCase,
		MaxMatches:           *limit,
		StripExtension:       *stripExtension,
		Basename:             *basenameMustMatch,
		StripPath:            *stripPath,
		Existing:             *existing, // We handle this manually, after getting the list of matches.
		Accessable:           *accessable,
		Symlink:              *symlinkCandidates,
		HashMap:              strings.Contains(*searchMethod, "hashmap"),
		LevenshteinCost:      fuzzyCost,
		LevenshteinThreshold: fuzzyThreshold,
		NWorkers:             *nworkers,
	}

	var err error
	t0 := time.Now()
	db, err = locate.NewDB(strings.Split(*dbFiles, ":"), &options)
	t1 := time.Now()
	vprintln(LOG, "Loaded", *dbFiles, "in", float64(t1.Sub(t0))/1e9, "seconds")
	if err != nil {
		log.Fatal(err)
	}
}

type Config struct {
	StripPath         bool
	CountEntries      bool
	DBFiles           string
	Existing          bool
	Follow            bool
	IgnoreCase        bool
	Limit             uint
	Accessable        bool
	LevenshteinParams string
	SearchMethod      string
	StripExtension    bool
	BasenameMustMatch bool
	SymlinkCandidates bool
	HttpAddr          string
	TemplateString    string
}

func printConfig(w io.Writer) {
	t := template.Must(template.ParseFiles("config.html"))
	c := &Config{
		StripPath:         *stripPath,
		CountEntries:      *countEntries,
		DBFiles:           *dbFiles,
		Existing:          *existing,
		Follow:            *follow,
		IgnoreCase:        *ignoreCase,
		Limit:             *limit,
		Accessable:        *accessable,
		LevenshteinParams: *levenshteinParams,
		SearchMethod:      *searchMethod,
		StripExtension:    *stripExtension,
		BasenameMustMatch: *basenameMustMatch,
		SymlinkCandidates: *symlinkCandidates,
		HttpAddr:          *httpAddr,
		TemplateString:    *templateString,
	}
	t.Execute(w, c)
}

type Match struct {
	Base, Path string
	N          int
}

func handler(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Path[1:]

	if pattern == "" {
		printConfig(w)
		return
	}

	nmatches := 0
	for _, method := range strings.Split(*searchMethod, ",") {
		var err error
		ch := make(chan string)
		go func() { err = locate.Locate(db, method, pattern, ch) }()

		for p := range ch {
			nmatches++
			m := &Match{Path: p, Base: filepath.Base(p), N: nmatches}
			tpl.Execute(w, m)
		}

		if err != nil {
			fmt.Println(w, err)
			return
		}
		if nmatches > 0 {
			break
		}
	}
}

func serveHTTP(addr string) {
	http.HandleFunc("/", handler)
	http.ListenAndServe(addr, nil)
}

func main() {
	if *httpAddr != "" {
		serveHTTP(*httpAddr)
		return
	}

	if flag.NArg() == 0 {
		printHelp(pkg, version, about, usage)
		os.Exit(0)
	}

	nmatches := 0
	for _, method := range strings.Split(*searchMethod, ",") {
		var err error
		ch := make(chan string)
		go func() { err = locate.Locate(db, method, flag.Arg(0), ch) }()

		for m := range ch {
			nmatches++
			fmt.Println(m)
		}

		if err != nil {
			log.Fatal(err)
		}
		if nmatches > 0 {
			break
		}
	}
}
