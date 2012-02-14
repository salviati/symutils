/*
   Copyright (c) Doğan Çeçen <dogan@cecen.info>, Utkan Güngördü
   <utkan@freeconsole.org>

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

// TODO(utkan): Handle relative symlinks
// TODO(utkan): Replicate rename-as-basename-only functionality.
// BUG(utkan): wildcard matching matches only against basenames (filepath.Match)

// replsym(1) finds symlinks pointing to a target, or targets described
// by a pattern, and replaces them with a given, new target.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	. "symutils/common"
)

var (
	target          = flag.String("t", "", "Replacement target for matched symlinks.")
	pattern         = flag.String("p", "", "Pattern for symlink targets for replacement.")
	matchMethod     = flag.String("m", "exact", "Matching method, can be wildcard, substring, regexp or exact)")
	caseInsensitive = flag.Bool("i", false, "Case insensitive matching")
	recurse         = flag.Bool("r", false, "Recurse into subdirectories")
	rename          = flag.Bool("R", false, "Rename symlinks as target's basename")
	showVersion     = flag.Bool("V", false, "Show version and license info and quit")
	showHelp        = flag.Bool("h", false, "Display help and quit")

	verbose = flag.Uint("v", 0, "Verbosity 0: errors only, 1: errors and warnings, 2: errors, warning, log")
)

const (
	pkg, version, author, about, usage string = "replsym", VERSION, "Doğan Çeçen, Utkan Güngördü",
		"replsym finds symlinks pointing to a target (or targets) described" +
			"by a pattern, and replaces them with a given new target." +
			"If no targets for replacement are given, replsym just prints out" +
			"the matching symlinks, thus replicating the behavior of the" +
			"former lssym(1) tool of symutils.",
		"replsym -p target_pattern [-t new_target] [-m match_type -i -v -r] symlink1/dir1 ... symlinkN/dirN"
)

var match func(pattern, filename string) bool

func imatch(pattern, filename string) bool {
	if *caseInsensitive {
		pattern = strings.ToLower(pattern)
		filename = strings.ToLower(filename)
	}
	return match(pattern, filename)
}

func WalkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		Errorf("%v\n", err)
		return err
	}

	if info.IsDir() {
		if *recurse == false {
			return filepath.SkipDir
		}
		return nil
	}

	issym := info.Mode()&os.ModeSymlink != 0

	if !issym {
		return nil
	}

	oldtarget, err := os.Readlink(path)
	if err != nil {
		log.Fatal(err)
	}

	if imatch(*pattern, oldtarget) {
		Logf("%s -> %s matches the pattern %s\n", path, oldtarget, *pattern)

		if *target == "" {
			fmt.Println(MakeAbsolute(path, ""))
			return nil
		}

		newname := path
		if *rename {
			newname = filepath.Base(*target)
			dir, _ := filepath.Split(path)
			newname = filepath.Join(dir, newname)
		}

		Logf("%s -> %s is being replaced by  %s -> %s\n", path, oldtarget, newname, *target)

		replace(newname, path, *target)
	}

	return nil
}

func replace(newname, oldname, target string) {
	err := os.Remove(oldname)
	if err != nil {
		log.Fatal(err)
	}

	err = os.Symlink(target, newname)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func init() {
	flag.Parse()

	SetLogLevel(*verbose)

	if *showHelp {
		PrintHelp(pkg, version, about, usage)
		os.Exit(0)
	}

	if *showVersion {
		PrintVersion(pkg, version, author)
		os.Exit(0)
	}

	if *pattern == "" {
		PrintVersion(pkg, version, author)
		os.Exit(0)
	}

	switch *matchMethod {
	case "exact":
		match = func(pattern, filename string) bool {
			return filename == pattern
		}

	case "regexp":
		re := regexp.MustCompile(*pattern)
		match = func(pattern, filename string) bool {
			return re.MatchString(filename)
		}

	case "wildcard":
		match = func(pattern, filename string) bool {
			_, name := filepath.Split(filename)
			matched, _ := filepath.Match(pattern, name)
			return matched
		}
	case "substring":
		match = func(pattern, filename string) bool {
			return strings.Contains(filename, pattern)
		}
	default:
		PrintVersion(pkg, version, author)
		os.Exit(0)
	}
}

func main() {
	for _, dir := range flag.Args() {
		if err := filepath.Walk(dir, WalkFunc); err != nil {
			log.Fatal(err)
		}
	}
}
