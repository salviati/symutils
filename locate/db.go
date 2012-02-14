package locate

// TODO(utkan): Instead of merging databases treat them separately using goroutines (?)

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"log"
)

// A list paths, indexed by a string.
type PathList map[string][]string

// Represents a database file.
type DB struct {
	dbFilenames []string // Databases
	files       []string // Union of files listed in db files
	options     Options

	basenames  PathList // A map of file basenames -> path of files with that basename
	hasmapLock sync.Mutex
}

func (db *DB) bakeBasenames() error {
	db.hasmapLock.Lock()
	defer db.hasmapLock.Unlock()

	db.basenames = make(PathList)
	for _, f := range db.files {
		base := filepath.Base(f)
		ix := bakeName(base, &db.options)
		db.basenames[ix] = append(db.basenames[ix], f)
	}

	return nil
}

// readDBs calls readDB for all database files, concatenates s/and sorts// the output.
// Result is stored in DB's files field.
func (db *DB) readDBs() error {
	if len(db.dbFilenames) == 1 {
		nametab, err := db.readDB(db.dbFilenames[0])
		if err != nil {
			return err
		}
		db.files = nametab
		return nil
	}

	fmap := make(map[string]bool)
	for _, dbf := range db.dbFilenames {
		nametab, err := db.readDB(dbf)
		for _, f := range nametab {
			fmap[f] = true
		}
		if err != nil {
			return err
		} // FIXME(utkan): We can move on to the next file instead of giving up
	}

	files := make([]string, len(fmap))
	i := 0
	for f, _ := range fmap {
		files[i] = f
		i++
	}
	//sort.SortStrings(files)
	db.files = files
	return nil
}

func setgid() error {
	exe, err := exec.LookPath(os.Args[0])
	if err != nil {
		return err
	}

	st, err := os.Stat(exe)
	if err != nil {
		return err
	}

	gid := int(st.Sys().(*syscall.Stat_t).Gid)

	return syscall.Setgid(gid)
}

func (db *DB) readDB(filename string) (r []string, err error) {
	var fb []byte

	func() {
		gid := syscall.Getgid()
		if setgid() != nil {
			defer syscall.Setgid(gid)
		}
		fb, err = ioutil.ReadFile(filename)
	}()

	if err != nil {
		return
	}

	if bytes.Compare(fb[0:8], []byte("\x00mlocate")) == 0 {
		return db.readMlocateDB(fb)
	}

	return
}

func (db *DB) readMlocateDB(fb []byte) (nametab []string, e error) {
	/*
		8 bytes magic
		4 bytes configuration block size (BE)
		1 byte file format version (0)
		1 byte visibility flag (0 or 1)
		2 bytes padding
		NIL terminated path name of the root
	*/

	if bytes.Compare(fb[0:8], []byte("\x00mlocate")) != 0 {
		e = errors.New("Not an mlocate database file")
		return
	}

	blocksize := int32(0)
	binary.Read(bytes.NewBuffer(fb[8:12]), binary.BigEndian, &blocksize)

	dbversion := fb[12]
	if dbversion != 0 {
		e = errors.New("Invalid database version")
		return
	}

	//visibility := fb[13] == 0
	rootpath, rem := nextCstr(fb[16:])

	stampsize := 16

	rem = rem[int(blocksize)+stampsize:]
	entry := rem

	// Get the entries themselves
	// FIXME(utkan): Something's terribly slowing things down here.
	rem = entry
	dirNameNow := true
	curDir := rootpath

	nametab = make([]string, 0)
	// ok becomes true when we reached the requested db.option.Root directory,
	// and we start adding files so forth.
	alwaysOk := filepath.HasPrefix(rootpath, db.options.Root)
	log.Println("alw", alwaysOk)
	log.Println("rootpath",rootpath)
	for ok := false; ; {
		var name string
		name, rem = nextCstr(rem)
		if dirNameNow {
			if !alwaysOk {
				ok = filepath.HasPrefix(name, db.options.Root) // FIXME(utkan): This is too inefficient.
			}
			curDir = name
			log.Println(curDir)
			dirNameNow = false
			if alwaysOk || ok {
				nametab = append(nametab, curDir)
			}
		} else {
			if alwaysOk || ok {
				nametab = append(nametab, curDir+"/"+name)
			}
		}
		ftype := rem[0]
		rem = rem[1:]

		if len(rem) == 0 {
			break
		}

		if ftype == 2 { // 0: non-directory file, 1: sub-directory, 2: end of directory
			rem = rem[stampsize:]
			dirNameNow = true
		}
	}

	return nametab, nil
}

// BUG(utkan): Let the caller of NewDB know whether Accessable is in effect or not.

// NewDB reads filenames in given databases and stores the union in a newly created DB.
// If everything goes fine, new DB is returned.
func NewDB(dbFilenames []string, options *Options) (db *DB, err error) {
	db = &DB{dbFilenames: dbFilenames}
	db.options = *options

	db.options.Root = filepath.Clean(db.options.Root)
	if db.options.Root == "" {
		db.options.Root = "/"
	}

	// BUG(utkan): Accessable option should not require RO access to _all_ DB files.
	if db.options.Accessable { // FIXME(utkan): No R_OK(=4) in syscall package!
		for _, dbFilename := range dbFilenames {
			if syscall.Access(dbFilename, 4) != nil {
				db.options.Accessable = false
				break
			}
		}
	}

	err = db.readDBs()
	if err != nil {
		return nil, err
	}

	if db.options.HashMap {
		err = db.bakeBasenames()
		if err != nil {
			return db, err
		}
		/*go func() {
			err = db.bakeBasenames()
			if err != nil {
				db.options.HashMap = false
			}
		}()*/
	}

	return db, nil
}
