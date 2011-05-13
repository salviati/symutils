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
	lssym(1) looks for (common) symlinks under given directories

	lssym [options] dir1 dir2 [dir3 ... dirN]
*/

package main

import (
	"fmt"
	"flag"
	"os"
	"path/filepath"
)

const (
	pkg, version, author, about, usage string = "cosym", VERSION, "Utkan Güngördü",
		"cosym(1) looks for common symlinks under given directories",
		"cosym [options] dir1 dir2 [dir3 ... dirN]"
)

//var recurse = flag.Bool("r", false, "Recursive search") //FIXME: can mean "allow depth > 1"

var showHelp = flag.Bool("h", false, "Display help")
//var symDirFollow = flag.Bool("F", false, "Follow symlinks when recursing (does nothing in this version)") //FIXME
var showVersion = flag.Bool("version", false, "Show version and license info and quit")
var nmatchMin = flag.Int("N", 0, "Minimum number of identical symlinks (in different dirs) to be enlisted. (0 means # of given directories.)")
var includeOrdinaryFiles = flag.Bool("o", false, "Do not discard ordinary files")
var checkSymlink = flag.Bool("c", false, "Check symlinks before enlisting")

type nametab_t map[string]int

var nametabs []nametab_t
var curNametab nametab_t
var currentDir string

type walkEnt struct{}

func (*walkEnt) VisitDir(dname string, d *os.FileInfo) bool {
	currentDir = dname
	//return *recurse
	return true
}

func (*walkEnt) VisitFile(filename string, d *os.FileInfo) {
	if !d.IsSymlink() && !*includeOrdinaryFiles {
		return
	}

	if d.IsSymlink() {
		if *checkSymlink {
			ok, _, _ := linkAlive(filename, false)
			if !ok {
				return
			}
		}

		var err os.Error
		filename, err = os.Readlink(filename)

		if err != nil {
			vprintf(INFO, "%v\n", err)
			return
		}
	}

	curNametab[filename] = 1
}

func walk(dname string) {
	dir, err := os.Stat(dname)
	if err != nil {
		vprintf(ERR, "%v\n", err)
		return
	}

	if !dir.IsDirectory() {
		vprintf(ERR, "%s is not a directory\n", dname)
	}

	v := new(walkEnt)
	ech := make(chan os.Error)
	go func() { filepath.Walk(dname, v, ech); close(ech) }()
	for e := range ech {
		vprintf(WARN, "%v\n", e)
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		printVersion(pkg, version, author)
		return
	}
	if *showHelp || flag.NArg() == 0 {
		printHelp(pkg, version, about, usage)
		return
	}

	if flag.NArg() < 2 {
		printHelp(pkg, version, about, usage)
		return
	}

	nametabs := make([]nametab_t, flag.NArg())

	for i := 0; i < flag.NArg(); i++ {
		nametabs[i] = make(nametab_t)
		curNametab = nametabs[i]
		walk(flag.Arg(i))
	}

	utab := make(nametab_t)
	for _, nametab := range nametabs {
		for f, _ := range nametab {
			ncopies, _ := utab[f]
			utab[f] = ncopies + 1
		}
	}

	// let negative values have a meaning too anyway
	if *nmatchMin <= 0 {
		*nmatchMin = flag.NArg()
	}
	for f, n := range utab {
		if n >= *nmatchMin {
			fmt.Println(f)
		}
	}
}
