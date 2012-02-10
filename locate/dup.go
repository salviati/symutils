package locate

func (db *DB) Duplicates() (pathlist PathList) {
	db.hasmapLock.Lock()
	defer db.hasmapLock.Unlock()
	
	pathlist = make(PathList)

	for basename, paths := range db.basenames {
		if len(paths) > 1 { pathlist[basename] = paths }
	}
	return
}
