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

// Package locate implements reading and searching in ?locate databases.
package locate

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"symutils/fuzzy"
)

// BUG(utkan): Cannot change IgnoreCase after creating DB.

// BUG(utkan): Currently recognizes mlocate database files only.

// BUG(utkan): IgnoreChars has no effect for now.

// TODO(utkan): Implement a function to report the DB type.

func (db *DB) locate(pattern string, ch chan string, match func(n string, h string) bool) (e error) {
	n := uint(0)
	nworkers := uint(0)

	worker := func(files []string, wch chan string, match func(n string, h string) bool) {
		defer func() {
			nworkers--
			if nworkers == 0 {
				close(wch)
			}
		}()

		for _, f := range files {
			haystack := bakeName(f, &db.options)

			if match(pattern, haystack) == false {
				continue
			}

			if db.options.Basename {
				if filepath.Base(f) != filepath.Base(pattern) {
					continue
				}
			}
			ok, err := matchOkay(f, &db.options)
			if err != nil {
				e = err
				return
			}
			if ok {
				wch <- f

				n++
			}
			if db.options.MaxMatches > 0 && n >= db.options.MaxMatches {
				return
			} // FIXME(utkan):
		}
	}

	nblock := uint(len(db.files)) / db.options.NWorkers
	nrem := uint(len(db.files)) % db.options.NWorkers

	nworkers = db.options.NWorkers
	if nrem != 0 {
		nworkers++
	} // BUG(utkan): An extra worker in locate() might cause performance loss depending on GOMAXPROCS

	wch := make(chan string)
	for i := uint(0); i < db.options.NWorkers; i++ {
		go worker(db.files[i*nblock:(i+1)*nblock], wch, match)
	}
	if nrem > 0 {
		go worker(db.files[db.options.NWorkers*nblock:], wch, match)
	}

	for f := range wch {
		ch <- f
	}

	return
}

// Uses the built-in map to look-up files.
// If NewDB was not called with HashMap option enabled, the lookup table
// will be created on demand.
func (db *DB) LocateHashMap(pattern string, ch chan string) error {
	if len(db.basenames) == 0 {
		if err := db.bakeBasenames(); err != nil {
			return err
		}
	}

	db.hasmapLock.Lock()
	defer db.hasmapLock.Unlock()

	matches, ok := db.basenames[bakeName(filepath.Base(pattern), &db.options)]
	if !ok {
		return nil // no matches
	}

	n := uint(0)
	for _, m := range matches {
		mOK, err := matchOkay(m, &db.options)
		if err != nil {
			return err
		}

		if mOK {
			ch <- m

			n++
			if db.options.MaxMatches > 0 && n >= db.options.MaxMatches {
				return nil
			}
		}
	}

	return nil
}

// Searches for entries that mathch filename pattern fn, using filepath.Match.
// Matching entries in the database are returned via channel ch.
// LocateWildcard forces StripPath option, even when not enabled.
func (db *DB) LocateWildcard(pattern string, ch chan string) (err error) {
	pattern = bakeName(pattern, &db.options)

	// With recent changes, filepath.Match behaves like fnmatch(3) with FNM_PATHNAME
	// enabled. We thus need StripPath
	stripPath := db.options.StripPath
	db.options.StripPath = true

	match := func(n string, h string) bool {
		m, _ := filepath.Match(n, h)
		return m
	}

	err = db.locate(pattern, ch, match)
	db.options.StripPath = stripPath
	return
}

// Searches for entries that mathch filename pattern fn, using regexp.MatchString
// Matching entries in the database are returned via channel ch.
func (db *DB) LocateRegexp(pattern string, ch chan string) (err error) {
	pattern = bakeName(pattern, &db.options)

	var re *regexp.Regexp
	re, err = regexp.Compile(pattern)
	if err != nil {
		return err
	}

	match := func(n string, h string) bool {
		return re.MatchString(h)
	}

	return db.locate(pattern, ch, match)
}

// Performs a fuzzy search in the database against name, with given cost values and threshold Levenshtein distance.
// Matching entries in the database are returned via channel ch.
func (db *DB) LocateLevenshtein(name string, ch chan string) (err error) {
	name = bakeName(name, &db.options)
	_, name = filepath.Split(name) // Work only with basename

	match := func(n string, h string) bool {
		return fuzzy.Levenshtein(n, h, &db.options.LevenshteinCost) <= db.options.LevenshteinThreshold
	}
	return db.locate(name, ch, match)
}

// Locates the files with name as a substring. Uses strings.Contains.
func (db *DB) LocateSubstring(name string, ch chan string) (err error) {
	name = bakeName(name, &db.options)
	match := func(n string, h string) bool {
		return strings.Contains(h, n)
	}
	return db.locate(name, ch, match)
}

// A wrapper for the Locate.+ functions.
// Method is specified by a string, which can be one of the following:
//  "wildcard", "substring", "levenshtein", "hashmap", "regexp"
// Returns the matches through a given channel.
func Locate(db *DB, method, pattern string, ch chan string) (err error) {
	defer close(ch)

	switch method {
	case "wildcard":
		return db.LocateWildcard(pattern, ch)
	case "substring":
		return db.LocateSubstring(pattern, ch)
	case "levenshtein":
		return db.LocateLevenshtein(pattern, ch)
	case "hashmap":
		return db.LocateHashMap(pattern, ch)
	case "regexp":
		return db.LocateRegexp(pattern, ch)
	}
	return errors.New("No such search method as " + method)
}

// A wrapper for the Locate.+ functions.
// Stores results of a Locate call in a string array, returns afterwards.
func locateIntoArray(pattern string, locateFn func(pattern string, ch chan string) error) (matches []string, err error) {
	nameMap := make(map[string]struct{})
	var elem struct{}
	ch := make(chan string)
	go func() { err = locateFn(pattern, ch) }()
	for f := range ch {
		nameMap[f] = elem
	}
	nameArray := make([]string, len(nameMap))
	i := 0
	for f, _ := range nameMap {
		nameArray[i] = f
		i++
	}

	return nameArray, err
}

// A wrapper for the Locate.+ functions.
// Method is specified by a string, which can be one of the following:
//  "wildcard", "substring", "levenshtein", "hashmap", "regexp"
// Stores results of a Locate call in a string array, returns afterwards.
func LocateAll(db *DB, method, pattern string) (matches []string, err error) {
	locateFn := func(pattern string, ch chan string) error { return Locate(db, method, pattern, ch) }
	return locateIntoArray(pattern, locateFn)
}
