package main

/*
 * Various filesystem related functions used by the main program.
 * */

import (
	"os"
	"path/filepath"
	"syscall"
)

/* Check for existence of a file, without following a symlink */
func fileExists(fname string) bool {
	var stat syscall.Stat_t
	err := syscall.Lstat(fname, &stat)

	if err == nil {
		return true
	}
	if err == syscall.ENOENT {
		return false
	}

	vprintf(WARN, "%v\n", err)
	return false
}

/* Checks whether the link is dead or not.
   Returns the alive state, link's target (if alive) and error. */
func linkAlive(fname string, matchNames bool) (isAlive bool, dst string, err error) {
	dst, err = os.Readlink(fname)
	if err != nil {
		vprintf(INFO, "%v\n", err)
		isAlive = false
		return
	}

	if dst[0] != '/' { //relative link
		dirname, _ := filepath.Split(fname)
		dst = filepath.Join(dirname, dst)
	}

	if matchNames {
		if filepath.Base(dst) != filepath.Base(fname) {
			isAlive = false
			return
		}
	}

	isAlive = fileExists(dst)
	return
}

/* If filename isn't absolute, try joining with dirname,
   if it still is not absolute, try tacking working dir
   Return the Clean()ed path */
func makeAbsolute(filename string, dirname string) string {
	if !filepath.IsAbs(filename) { //relative link
		filename = filepath.Join(dirname, filename)
	}
	if !filepath.IsAbs(filename) {
		wd, err := os.Getwd()
		if err != nil {
			vprintf(ERR, "%s\n", err)
			return filename
		}
		filename = filepath.Join(wd, filename)
	}

	return filepath.Clean(filename)
}
