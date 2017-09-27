package watchd

// WatchExistsErr is an error returned by Watch indicating that a requested
// directory or one of its parents is already watched
type WatchExistsErr struct {
	dir string
}

func (e *WatchExistsErr) Error() string {
	return "watch already exists for " + e.dir
}

