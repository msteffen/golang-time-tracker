package watcher

import (
	"context"
	"fmt"
	"os"
	p "path"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// watchMask is the mask passed to all InotifyAddWatch() syscalls
	watchMask = unix.IN_CREATE | unix.IN_DELETE | unix.IN_MODIFY |
		unix.IN_MOVED_FROM | unix.IN_MOVED_TO | unix.IN_IGNORED

	// inotifyBufSz is the minimum buffer size required to read a unix inotify
	// event. Per 'man 7 inotify':
	//  Specifying a buffer of size 'sizeof(struct inotify_event) + NAME_MAX + 1'
	//  will be sufficient to read at least one event.
	inotifyBufSz = unix.SizeofInotifyEvent + unix.NAME_MAX + 1
)

// WatchEventType is the type associated with each WatchEvent returned by a
// watcher
type WatchEventType uint8

// The definition of each of the WatchEventTypes
const (
	Create WatchEventType = iota
	Delete
	Modify
	Ignore
)

// A WatchEvent is passed to a callback by a watcher to indicate some
// filesystem event
type WatchEvent struct {
	Type  WatchEventType
	IsDir bool
	Path  string
}

func (e WatchEvent) String() string {
	var verb string
	switch e.Type {
	case Create:
		verb = "Create"
	case Delete:
		verb = "Delete"
	case Modify:
		verb = "Modify"
	}
	var prettyPath string
	if e.IsDir {
		prettyPath = e.Path + "/"
	} else {
		prettyPath = e.Path
	}
	return verb + " \"" + prettyPath + "\""
}

// Watch begins watching all directories under 'dir', calling 'cb' with every
// event
func Watch(ctx context.Context, path string, cb func(WatchEvent) error) (retErr error) {
	path = p.Clean(path)
	pathInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("could not Stat() watch target %q: %v", path, err)
	}

	// Init inotify file descriptor
	fd, err := unix.InotifyInit()
	if err != nil {
		return err
	}
	defer func() {
		if err := unix.Close(fd); err != nil && retErr == nil {
			retErr = err
		}
	}()
	w := &fileWatcher{
		rootDir:     path,
		watchFD:     fd,
		wdToPath:    make(map[int]string),
		watchedDirs: make(map[string]struct{}),
		cb:          cb,
		doneCtx:     ctx,
	}
	// w.add(path) may call w.cb several times and set up several watches before
	// we get to run(), but shouldn't call w.cb on 'path'
	if err := w.add(WatchEvent{
		Type:  Create,
		Path:  path,
		IsDir: pathInfo.IsDir(),
	}); err != nil {
		return err
	}
	return w.run() // wait for further events
}

// fileWatcher watches a directory. N.B. that watcher is *not* a concurrent data
// structure, for simplicity. It reads from 'watchFD' until a new event comes
// in, and then it does not read from 'watchFD' again until the event has been
// processed (including traversing any subdirectories, etc--see below)
//
// Re. traversing new subdirectories (per 'man 7 watch'):
//  If monitoring an entire directory subtree, and a new subdirectory is created
//  in that tree or an existing directory is renamed into that tree, be aware
//  that by the time you create a watch for the new subdirectory, new files (and
//  subdirectories) may already exist inside the subdirectory. Therefore, you
//  may want to scan the contents of the subdirectory immediately after adding
//  the watch (and, if desired, recursively add watches for any subdirectories
//  that it contains).
type fileWatcher struct {
	// rootDir is root directory being watched
	rootDir string

	// watchFD is the unix file descriptor where this fileWatcher reads watch
	// events
	watchFD int

	// wdToPath maps watch descriptors to path (used to interpret new inotify
	// events generated by the linux kernel)
	wdToPath map[int]string

	// watchedDirs contains the set of watched directories (i.e. its keys are all
	// the values of 'wdToPath'. If the values were watch descriptors, it would be
	// an inverted index)
	watchedDirs map[string]struct{}

	// cb is the callback that is given each WatchEvent
	cb func(WatchEvent) error

	// doneCtx is used to cancel the watch (which is the only way for Watch() to
	// return without an error)
	doneCtx context.Context
}

// run add()s any un-scanned directories, and if none are left, reads
// w's watchFD in a loop, and responds to events (by adding new subdirs to 'w'
// and calling the watch callback).
//
// By not calling unix.Read() until all new directories are scanned, we prevent
// a race between:
// 1. setting up the unix watch, receiving the watch descriptor wd from
//    unix.InotifyAddWatch, and adding wd to w.wdToPath
// 2. reading watch events from w.watchFd for wd
func (w *fileWatcher) run() error {
	var end int
	buf := make([]byte, inotifyBufSz*10)
	// linux API modifies this slice in-place (but we don't care)
	watchFD := []unix.PollFd{{
		Fd:     int32(w.watchFD),
		Events: unix.POLLIN | unix.POLLPRI,
	}}
	timeout := -1 // poll blocks until watchFD is ready or there's a signal
	if w.doneCtx != nil {
		timeout = 1000 // poll returns after 1000ms = 1s to check w.doneCtx again
	}
	for {
		if w.doneCtx != nil {
			select {
			case <-w.doneCtx.Done():
				return nil // doneCtx has been cancelled
			default:
			}
		}

		// Poll watchFD
		n, err := unix.Poll(watchFD, timeout)
		if err != nil {
			return fmt.Errorf("select() error: %v", err)
		}
		if n > 0 {
			// Only read events were requested, so data must be available to read
			n, err := unix.Read(w.watchFD, buf[end:])
			if err != nil {
				return fmt.Errorf("Error reading watch FD: %v", err)
			}
			end += n

			// Consume events from buf, if possible
			end, err = w.forEachEvent(buf[:end], w.applyWatchEvent)
			if err != nil {
				return err
			}
		}
	}
}

// add adds a subdirectory of w's original dir to the underlying linux inotify
// instance. 'dir' MUST be a directory, and callers may need to run Stat(dir)
// to confirm that it's a directory before passing it to add()
//
// Two notes:
// - There are several system calls made in this function which could emit
//   NotExist errors; in all cases, 'path's existence is confirmed before
//   add(path) is called, so it always means the failing system call raced with
//   a delete and we'll get a delete event as soon as we read watchFD.  Because
//   of this, many system calls check os.IsNotExists(err) and drop the error if
//   so
// - Callers must ensure that 'path's suffix is '/' iff 'path' is a directory
func (w *fileWatcher) add(e WatchEvent) (err error) {
	if e.Type != Create {
		panic(fmt.Sprintf("add(%s) called with non-Create event", e))
	}

	// Create watch and note the path associated with this watch descriptor
	wd, err := unix.InotifyAddWatch(w.watchFD, e.Path, watchMask)
	if err != nil {
		if os.IsNotExist(err) && e.Path != w.rootDir {
			return nil
		}
		return fmt.Errorf("could not add watch: %v", err)
	}
	w.wdToPath[wd] = e.Path
	w.watchedDirs[e.Path] = struct{}{}

	// Watch is now in place, but children of 'e.Path' may have been added while
	// watch was being created, so scan the current contents of 'e.Path' and add any
	// existing subdirs to 'w'
	f, err := os.Open(e.Path)
	if err != nil {
		if os.IsNotExist(err) && e.Path != w.rootDir {
			return nil // dir was deleted in the interim--event will appear later
		}
		return fmt.Errorf("could not open directory %q: %v", e.Path, err)
	}
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("could not close() watched dir: %v", closeErr)
		}
	}()
	childInfos, err := f.Readdir(0)
	if err != nil {
		if os.IsNotExist(err) && e.Path != w.rootDir {
			return nil
		}
		return fmt.Errorf("could not read contents of directory %q: %v", e.Path, err)
	}
	for _, childInfo := range childInfos {
		child := childInfo.Name()
		// heuristic: avoid .git directories
		if strings.Contains(child, ".git") {
			continue
		}

		// heuristic: avoid golang vendor directories, since I typically use this
		// with go projects (omits "vendor" as well as "vendor.X")
		if strings.HasPrefix(child, "vendor") {
			continue // avoid golang vendor directories
		}
		childPath := p.Join(e.Path, child)

		// generate synthetic Create event for child and apply it (calls w.cb)
		if err := w.applyWatchEvent(WatchEvent{
			Type:  Create,
			IsDir: childInfo.IsDir(),
			Path:  childPath,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *fileWatcher) forEachEvent(buf []byte, cb func(WatchEvent) error) (end int, err error) {
	var start int
	// set 'end' in deferred function--avoid doing this before every return
	defer func() {
		copy(buf, buf[start:])
		end = len(buf) - start
	}()
	for start < len(buf) {
		// check if buffer contains full Inotify event
		// TODO(msteffen): may not need this, due to "short read" check in
		// readNextEventsFromFDInto (n < SizeofInotifyEvent)
		if start+unix.SizeofInotifyEvent > len(buf) {
			break
		}
		structEnd := start + unix.SizeofInotifyEvent

		// extract inotify event struct (minus name)
		event := (*unix.InotifyEvent)(unsafe.Pointer(&buf[start]))
		// Check if buffer contains entire name
		if structEnd+int(event.Len) > len(buf) {
			break
		}

		// extract name
		// Per man 7 inotify:
		//  The name field is present only when an event is returned for a file
		//  inside a watched directory; it identifies the filename within to the
		//  watched directory. This filename is null-terminated, and may include
		//  further null bytes ('\0') to align subsequent reads to a suitable
		//  address boundary.
		// Note that when deleting watched directories themselves, 'name' is empty
		var name string
		for r := int(event.Len); r > 0; r-- {
			if buf[structEnd+r-1] != 0 {
				name = string(buf[structEnd : structEnd+r])
				break
			}
		}
		we, err := w.toWatchEvent(event, name)
		if err != nil {
			return 0, err
		}

		// advance 'start'
		start += unix.SizeofInotifyEvent + int(event.Len)

		// process 'we'
		if we.Type == Ignore {
			if we.Path == w.rootDir {
				return 0, fmt.Errorf("watch root %q has been deleted", w.rootDir)
			}
			// ignore other IN_IGNORED events (rely on IN_DELETE from parent)
			continue
		}
		// Heuristic: ignore vim writes to backup files
		if strings.HasSuffix(we.Path, ".swp") || strings.HasSuffix(we.Path, ".swo") {
			continue
		}
		if err := cb(we); err != nil {
			return 0, err
		}
	}
	return 0, nil
}

// w.modelMu must be locked before calling this
func (w *fileWatcher) applyWatchEvent(we WatchEvent) (err error) {
	// 1a. if we.Type == Modify, just call w.cb immediately, there's no extra
	// watch handling that needs to be done
	if we.Type == Modify {
		return w.cb(we)
	}

	// 1b. if we.Type == Delete, just check if it's watched, and if so, removed
	// it from watched dirs (no OS calls are needed)
	if we.Type == Delete {
		if _, isWatched := w.watchedDirs[we.Path]; isWatched {
			delete(w.watchedDirs, we.Path)
		}
		return w.cb(we)
	}

	// 2. Strip out duplicate Create() events (can happen if foo/ and foo/bar/ are
	// created in rapid succession:
	// - watch is added for foo/
	// - foo/bar/ is created after InotifyAddWatch() returns but before
	//   list(foo/) is called
	// - event is generated for foo/bar/ but foo/bar/ is also added recursively
	//   inside add(foo/)
	if _, isWatched := w.watchedDirs[we.Path]; we.Type == Create && isWatched {
		return nil
	}

	// call w.cb on 'path'
	if err := w.cb(we); err != nil {
		return err
	}

	// recursively descend into 'path' if needed
	if we.IsDir {
		return w.add(we)
	}
	return nil
}

func (w *fileWatcher) toWatchEvent(e *unix.InotifyEvent, name string) (WatchEvent, error) {
	result := WatchEvent{
		IsDir: e.Mask&unix.IN_ISDIR > 0,
	}
	switch {
	case e.Mask&unix.IN_CREATE > 0, e.Mask&unix.IN_MOVED_TO > 0:
		result.Type = Create
	case e.Mask&unix.IN_DELETE > 0, e.Mask&unix.IN_MOVED_FROM > 0:
		result.Type = Delete
	case e.Mask&unix.IN_MODIFY > 0:
		result.Type = Modify
	case e.Mask&unix.IN_IGNORED > 0:
		result.Type = Ignore
	}
	parentPath, ok := w.wdToPath[int(e.Wd)]
	if !ok {
		return WatchEvent{}, fmt.Errorf("event for unrecognized watch descriptor %d", e.Wd)
	}
	result.Path = p.Clean(p.Join(parentPath, name))
	return result, nil
}

// Render returns a human-readable string corresponding to 'e'
func (w *fileWatcher) Render(e *unix.InotifyEvent, name string) string {
	parentPath, ok := w.wdToPath[int(e.Wd)]
	if !ok {
		parentPath = "<unknown parent>"
	}

	var eType string
	switch {
	case e.Mask&unix.IN_CREATE > 0:
		eType = "Create"
	case e.Mask&unix.IN_DELETE > 0:
		eType = "Delete"
	case e.Mask&unix.IN_MODIFY > 0:
		eType = "Modify"
	case e.Mask&unix.IN_MOVED_FROM > 0:
		eType = "Move from"
	case e.Mask&unix.IN_MOVED_TO > 0:
		eType = "Move to"
	case e.Mask&unix.IN_DELETE_SELF > 0:
		eType = "Delete watched dir"
	case e.Mask&unix.IN_MOVE_SELF > 0:
		eType = "Move watched dir"
	}

	path := p.Clean(p.Join(parentPath, name))
	result := fmt.Sprintf("Event type: %s %s", eType, path)
	if e.Mask&unix.IN_ISDIR > 0 {
		result += " (dir)\n"
	} else {
		result += " (file)\n"
	}
	return result
}
