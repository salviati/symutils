// dups(1) Find duplicate files with the same name, using a locate database. Can remove the dups, or convert them to links pointing to a chosen "origin" file.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	. "symutils/common"
	"symutils/locate"
)

const (
	DBFILES = "/var/lib/mlocate/mlocate.db"
)

// TODO(utkan): Ignore pattern and extension.
var (
	dbFiles    = flag.String("d", DBFILES, "List of : separated database files. dups will try to determine the format automatically.")
	existing   = flag.Bool("e", false, "List only existing files.")
	ignoreCase = flag.Bool("i", false, "Ignore case.")
	root       = flag.String("root", "/", "Only files under root will be searched.")
	accessable = flag.Bool("a", true, "List only (read-)accessable files (disabling this option requires RO access to all given DB files)")
	nworkers   = flag.Uint("nworkers", 1, "The number of parallel workers searching in one database")

	_minSize  = flag.Uint64("min", 0, "Set the minumum file size (see the unit option), smaller files will be discarded.")
	matchSize = flag.Bool("s", true, "Compare file sizes.")

	// var ncompare = flag.Uint("n", 0, "Compare first n bytes before declaring files identical")

	showVersion = flag.Bool("V", false, "Display version and licensing information, and quit.")
	showHelp    = flag.Bool("h", false, "Display help and quit")

	unit     = flag.String("unit", "B", "B for bytes, K for KiB, M for MiB, G for GiB, T for TiB")
	yesToAll = flag.Bool("Y", false, "Assume yes to all y/n questions (they appear before making changes in the filesystem)")
	action   = flag.String("action", "none", "What to do with duplicates? Valid choices are none (nothing), rm (remove), ln (link back to origin).")
	
	verbose  = flag.Uint("v", 0, "Verbosity 0: errors only, 1: errors and warnings, 2: errors, warning, log")
)

var db *locate.DB

const (
	pkg, version, author, about, usage string = "dups", VERSION, "Utkan Güngördü",
		"dups(1) Find duplicate files with the same name, using a locate database. Can remove the dups, or convert them to links pointing to a chosen \"origin\" file.",
		"dups [options]"
)

type filesize int64

func (n filesize) String() string {
	return fmt.Sprint(int64(n) / multiplier)
}

var (
	multiplier = int64(1)
	minSize    = filesize(0)
)

func okay(format string, va ...interface{}) bool {
	if *yesToAll {
		return true
	}
	return Queryf(format, va...)
}

func init() {
	flag.Parse()

	if *showHelp {
		PrintHelp(pkg, version, about, usage)
		os.Exit(0)
	}
	
	if *verbose < LogLevelMin || *verbose > LogLevelMax {
		log.Fatal("Verbosity parameter should be between", LogLevelMin, "and", LogLevelMax)
	}
	LogLevel = LogLevelType(int(*verbose))

	mulmap := map[string]int64{"B": 1, "K": 1024, "M": 1024 * 1024, "G": 1024 * 1024 * 1024, "T": 1024 * 1024 * 1024 * 1024}

	var ok bool
	if multiplier, ok = mulmap[*unit]; !ok {
		log.Fatal("Invalid unit", *unit)
	}
	minSize = filesize(int64(*_minSize) * multiplier)

	options := locate.Options{
		IgnoreCase: *ignoreCase,
		Basename:   false,
		StripPath:  false,
		Existing:   *existing, // We handle this manually, after getting the list of matches.
		Accessable: *accessable,
		Symlink:    false,
		HashMap:    true,
		NWorkers:   *nworkers,
		Root:       *root,
	}

	var err error
	db, err = locate.NewDB(strings.Split(*dbFiles, ":"), &options)
	if err != nil {
		log.Fatal(err)
	}
}

func rm(path string) error {
	if okay("Really remove the file?: %s", path) {
		return os.Remove(path)
	}
	Logln("Removed file: ", path)
	return nil
}

func rmAndLink(path, newtarget string) error {
	if err := rm(path); err != nil {
		return err
	}

	if okay("Okay to create the symlink?: %s -> %s", path, newtarget) {
		return os.Symlink(newtarget, path)
	}
	return nil
}

func handleDups(paths []string) {
	if len(paths) < 2 {
		return
	}
	fmt.Println()
	orig, cancel := Choose("Which of these should be considered as the origin?", paths)
	if cancel {
		Logln("User cancel")
		return
	}

	switch *action {
	case "rm":
		for i := 0; i < len(paths); i++ {
			if i == orig {
				continue
			}
			if err := rm(paths[i]); err != nil {
				Warnln(err)
			}
		}
	case "ln":
		for i := 0; i < len(paths); i++ {
			if i == orig {
				continue
			}
			if err := rmAndLink(paths[i], paths[orig]); err != nil {
				Warnln(err)
			}
		}
	default:

	}
}

func main() {
	dups := db.Duplicates()
	Logln(len(dups), "files don't have unique names")

	for _, paths := range dups {
		if *matchSize == false {
			handleDups(paths)
			continue
		}

		sizelist := make(map[int64][]string)
		// FIXME(utkan): Use db.LocateHashMap(path) to do the job.

		for _, path := range paths {
			fi, err := os.Lstat(path)
			if err != nil {
				Warnln(err)
				continue
			}
			if fi.Mode()&os.ModeType != 0 {
				continue
			}

			size := fi.Size()

			if size < int64(minSize) {
				continue
			}

			if _, ok := sizelist[size]; !ok {
				sizelist[size] = make([]string, 0)
			}
			sizelist[size] = append(sizelist[size], path)
		}

		for size, paths := range sizelist {
			if len(paths) <= 1 {
				continue
			}

			for _, path := range paths {
				fmt.Println("[", size, "B,", filesize(size), *unit, "]", path)
			}

			handleDups(paths)
		}
	}
}
