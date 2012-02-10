package locate

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"os"
)

func stripExtension(name string) string {
	_, b := filepath.Split(name)
	ext := filepath.Ext(b)
	if ext != "" {
		name = name[0 : len(name)-len(ext)]
	}
	return name
}

// Filters given file name accordingly
// FIXME(utkan): Do not screw the pattern here, write a new function bakePattern instead
func bakeName(name string, options *Options) string {
	//name = filepath.Clean(name)

	if options.IgnoreCase {
		name = strings.ToLower(name)
	}

	if options.StripExtension {
		name = stripExtension(name)
	}

	if options.StripPath {
		name = filepath.Base(name)
	}
	return name
}

/* func bakePattern(name string, options *Options) string {

} */

func nextCstr(b []byte) (cstr string, rest []byte) {
	split := bytes.SplitN(b, []byte("\x00"), 2)
	return string(split[0]), split[1]
}

func escape(s string, echars string) string {
	for i := 0; i < len(echars); i++ { //FIXME: clean up this mess
		c := echars[i : i+1]
		r := regexp.MustCompile(`\` + c)
		s = r.ReplaceAllString(s, `\`+c)
	}

	return s
}

func existing(f string) (exists, issym bool, err error) {
	fi, err := os.Lstat(f)
	if err != nil && err != syscall.ENOENT {
		return
	}
	exists = err != os.ENOENT
	issym = fi.Mode() & os.ModeSymlink != 0
	return
}

// Checks whether the file can be considered a match according to given options
// Existing option requires the file to exist
// Symlink option allows the file to be a symlink
func fileOkay(f string, options *Options) (bool, error) {
	fi, err := os.Lstat(f)

	if err != nil && err != os.ENOENT {
		return false, err
	}

	if options.Existing && (err == os.ENOENT) {
		return false, nil
	} // Drop dead files...

	if options.Accessable { // FIXME(utkan): No R_OK(=4) in syscall package!
		err = syscall.Access(f, 4)
		if err != nil {
			return false, nil
		}
	}

	issym := fi.Mode() & os.ModeSymlink != 0
	if options.Symlink == false && issym {
		return false, nil
	} // ...and symlinks, if necessary.
	return true, nil
}

func matchOkay(f string, options *Options) (ok bool, err error) {

	if !options.Existing && options.Symlink {
		return true, nil //If everything's welcomed, no need to check
	}

	ok, err = fileOkay(f, options)
	if err != nil {
		return ok, err //FIXME(utkan):
	}
	return
}
