package locate

// TODO(salviati): Instead of merging databases treat them separately using goroutines (?)

import (
	"io/ioutil"
	"encoding/binary"
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"exec"
	"sync"
)

// Represents a database file.
type DB struct {
	dbFilenames []string // Databases
	files       []string // Union of files listed in db files
	options     Options

	basenames map[string][]string // A map of file basenames -> path of files with that basename
	hasmapLock sync.Mutex
}

func (db *DB) bakeBasenames() os.Error {
	db.hasmapLock.Lock()
	defer db.hasmapLock.Unlock()

	db.basenames = make(map[string][]string)
	for _, f := range db.files {
		base := filepath.Base(f)
		ix := bakeName(base, &db.options)
		db.basenames[ix] = append(db.basenames[ix], f)
	}

	return nil
}

// readDBs calls readDB for all database files, concatenates s/and sorts// the output.
// Result is stored in DB's files field.
func (db *DB) readDBs() os.Error {
	if len(db.dbFilenames) == 1 {
		nametab, err := db.readDB(db.dbFilenames[0])
		if err != nil { return err }
		db.files = nametab
		return nil
	}

	fmap := make(map[string]bool)
	for _, dbf := range db.dbFilenames {
		nametab, err := db.readDB(dbf)
		for _, f := range nametab {
			fmap[f] = true
		}
		if err != nil { return err } // FIXME(salviati): We can move on to the next file instead of giving up
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

func setgid() os.Error {
	exe, err := exec.LookPath(os.Args[0])
	if err != nil { return err }

	st, err := os.Stat(exe)
	if err != nil { return err }

	e := syscall.Setgid(st.Gid)
	if e != 0 { return &os.PathError{"setgid", exe, os.Errno(e)} }
	return nil
}

func (db *DB) readDB(filename string) (r []string, err os.Error) {
	var fb []byte
	
	func() {
		gid := syscall.Getgid()
		if setgid() != nil { defer syscall.Setgid(gid) }
		fb, err = ioutil.ReadFile(filename)
	}()

	if err != nil { return }

	if bytes.Compare(fb[0:8], []byte("\x00mlocate")) == 0 {
		return db.readMlocateDB(fb)
	}

	return
}

func (db *DB) readMlocateDB(fb []byte) (nametab []string, e os.Error) {
	/*
		8 bytes magic
		4 bytes configuration block size (BE)
		1 byte file format version (0)
		1 byte visibility flag (0 or 1)
		2 bytes padding
		NIL terminated path name of the root
	*/

	if bytes.Compare(fb[0:8], []byte("\x00mlocate")) != 0 {
		e = os.NewError("Not an mlocate database file")
		return
	}

	blocksize := 0
	binary.Read(bytes.NewBuffer(fb[8:12]), binary.BigEndian, &blocksize)

	dbversion := fb[12]
	if dbversion != 0 {
		e = os.NewError("Invalid database version")
		return
	}

	//visibility := fb[13] == 0
	rootpath, rem := nextCstr(fb[16:])

	stampsize := 16

	rem = rem[blocksize+stampsize:]
	entry := rem

	// Get the entries themselves
	// FIXME(salviati): Something's terribly slowing things down here.
	rem = entry
	dirNameNow := true
	curDir := rootpath

	nametab = make([]string, 0)
	for {
		var name string
		name, rem = nextCstr(rem)
		if dirNameNow {
			curDir = name
			dirNameNow = false
			nametab = append(nametab, curDir)
		} else {
			nametab = append(nametab, curDir + "/" + name)
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

// BUG(salviati): Let the caller of NewDB know whether Accessable is in effect or not.

// NewDB reads filenames in given databases and stores the union in a newly created DB.
// If everything goes fine, new DB is returned.
func NewDB(dbFilenames []string, options *Options) (db *DB, err os.Error) {
	db = &DB{dbFilenames: dbFilenames}
	db.options = *options

	// BUG(salviati): Accessable option should not require RO access to _all_ DB files.
	if db.options.Accessable { // FIXME(salviati): No R_OK(=4) in syscall package!
		for _, dbFilename := range dbFilenames {
			e := syscall.Access(dbFilename, 4)
			if e != 0 {
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
		/* err = db.bakeBasenames()
		if err != nil {
			return db, err
		} */
		go func() {
			err = db.bakeBasenames()
			if err != nil {
				db.options.HashMap = false
			}
		}()
	}

	return db, nil
}
