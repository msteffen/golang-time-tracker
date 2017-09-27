package watcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	p "path"
	"strings"
	"testing"
	"time"

	"github.com/msteffen/golang-time-tracker/pkg/check"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// set by TestMain
var testDir string

var testRand = rand.New(rand.NewSource(7))

type paths map[string]struct{}

func (d paths) String() string {
	var buf bytes.Buffer
	var sep string
	for k := range d {
		buf.WriteString(sep)
		buf.WriteString(k)
		sep = ", "
	}
	return buf.String()
}

// ConfirmEq returns 'nil' if 'd' and 'other' are equal, or an error otherwise
func (d paths) ConfirmEq(other paths) error {
	if len(other) != len(d) {
		// slight abuse of error, but this only really used by tests
		return fmt.Errorf("expected 'paths' of size %d, but was %d:\n%v\n%v",
			len(d), len(other), d, other)
	}
	for k := range d {
		if _, ok := other[k]; !ok {
			return fmt.Errorf("expected %q, but didn't find it:\n%v", k, other)
		}
	}
	return nil // success
}

func (d paths) Copy() paths {
	var result paths = make(map[string]struct{})
	for s := range d {
		result[s] = struct{}{}
	}
	return result
}

func (d paths) Add(path string) {
	for path = p.Clean(path); path != ""; path = p.Dir(path) {
		if _, ok := d[path]; ok {
			break // all other parents have been added
		}
		d[path] = struct{}{}
	}
}

func (d paths) Rm(path string) {
	for f := range d {
		if strings.HasPrefix(f, path) {
			delete(d, f)
		}
	}
}

func (d paths) Pick(except ...string) string {
	exceptM := make(map[string]struct{})
	for _, e := range except {
		exceptM[e] = struct{}{}
	}

	i := testRand.Int31n(int32(len(d) - len(exceptM)))
	var f string
	for f = range d {
		if _, ok := exceptM[f]; ok {
			continue
		}
		if i--; i < 0 { // i < 0 => i == 0 at the start (i \in [0, n) )
			break
		}
	}
	return f
}

// NewTestDir generates a randomly named testdir, creates it, and returns it
// (along with the path to a pre-created child of that dir in the second
// argument, for convenience)
func NewTestDir(t testing.TB) (string, string) {
	u := uuid.New()
	dirName := fmt.Sprintf("%s-%s", t.Name(), base64.RawURLEncoding.EncodeToString(u[:]))
	dirPath := p.Join(testDir, dirName)
	MkdirT(t, dirPath, 0755)
	MkdirT(t, p.Join(dirPath, "child"), 0755)
	return dirPath, p.Join(dirPath, "child")
}

func MkdirT(t testing.TB, name string, perms os.FileMode) {
	check.T(t, check.Nil(os.Mkdir(name, perms)))
}

// TestBasic creates a watch (on 'dir') and then waits for two events: a
// synthetic event for 'childOne' (which is created before the watch is
// established) and a real event for 'childTwo' (which is created by a separate
// goro.
//
// childTwo cannot be created before the childOne event (because the goro that
// creates childTwo blocks on 'block', which is closed when the childOne event
// is received), and if either event is received more than once, the test will
// err, and both events perform an operation that can only be done once.
func TestBasic(t *testing.T) {
	dir, child := NewTestDir(t)
	foo := p.Join(dir, "foo")

	block := make(chan struct{})
	go func() {
		<-block // yield to calling goro
		MkdirT(t, foo, 0755)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	err := Watch(ctx, dir, func(e WatchEvent) error {
		switch e.Path {
		case child:
			close(block) // start goro making modifications
			return nil
		case foo:
			cancel() // can this be done more than once? Don't think so, but not sure
		default:
			return fmt.Errorf("unwanted event: %s", e)
		}
		return nil
	})
	check.T(t, check.Nil(err))
}

// TestErrorForInitialEvent tests that errors returned by the function argument
// to 'Watch()' are propagated back up to the caller of 'Watch', even when the
// event is a synthetic event generated for one of the initial members of the
// watched dir.
func TestErrorForInitialEvent(t *testing.T) {
	dir, child := NewTestDir(t)

	// test error for startup event
	err := Watch(nil, dir, func(e WatchEvent) error {
		if e.Path != child {
			return fmt.Errorf("unwanted event: %s", e)
		}
		return fmt.Errorf("expected error")
	})
	check.T(t,
		check.NotNil(err),
		check.Eq(err.Error(), "expected error"))
}

// TestError tests that errors returned by the function argument
// to 'Watch()' are propagated back up to the caller of 'Watch'.
func TestError(t *testing.T) {
	dir, child := NewTestDir(t)
	foo := p.Join(dir, "foo")

	block := make(chan struct{})
	go func() {
		<-block // yield to calling goro
		MkdirT(t, foo, 0755)
	}()
	err := Watch(nil, dir, func(e WatchEvent) error {
		switch e.Path {
		case child:
			close(block) // start goro making modifications
			return nil
		case foo:
			return fmt.Errorf("expected error")
		default:
			return fmt.Errorf("unwanted event: %s", e)
		}
		return nil
	})
	check.T(t,
		check.NotNil(err),
		check.Eq(err.Error(), "expected error"))
}

// TestErrorDeleteRoot deletes the dir being watched directly, and checks that
// Watch() returns an error (even if the event-handling function doesn't)
func TestErrorDeleteRoot(t *testing.T) {
	dir, child := NewTestDir(t)
	block := make(chan struct{})

	go func() {
		<-block // yield to calling goro
		// For some reason, check.T(t, check.Nil(...)) doesn't seem to work in goros
		if err := os.RemoveAll(dir); err != nil {
			panic("could not remove " + dir)
		}
	}()
	err := Watch(nil, dir, func(e WatchEvent) error {
		if e.Type == Create && e.Path == child {
			close(block) // start goro making modifications
		}
		return nil // always return nil
	})
	check.T(t,
		check.NotNil(err),
		check.Eq(
			err.Error(),
			fmt.Sprintf("watch root \"%s\" has been deleted", dir)))
}

// TestErrorNoRoot checks that watching a nonexistent directory yields an error
func TestErrorNoRoot(t *testing.T) {
	// initialize 'dir' but don't create an actual directory
	dir := p.Join(testDir, t.Name())
	err := Watch(nil, dir, func(e WatchEvent) error {
		return nil
	})
	check.T(t,
		check.NotNil(err),
		check.Eq(
			err.Error(),
			fmt.Sprintf("could not Stat() watch target \"%[1]s\": "+
				"stat %[1]s: no such file or directory", dir)))
}

// TestFuzz randomly creates and deletes directories under 'dir', and makes sure
// that the on-disk state can always be recreated from events emitted by a
// Watch()
//
// Specifically this test spawns a goro that modifies the filesystem randomly,
// and writes expected filesystem states to 'expectedStateCh'. In the main
// thread, this test reads states out of 'expectedStateCh' and events out of
// 'eventCh' and makes sure the events read from 'eventCh' eventually recreate
// every state read from 'expectedStateCh' in order
//
// Note that the writer goro needs to be blocked after every write, so that it
// doesn't modify files while the main goro is trying to recreate a previous
// state. This can cause watch events to be delivered out-of-order which can
// prevent an intermediate state from ever being reached. To accomplish this
// syncronization, after writing an expected state to 'expectedStateCh', the
// writer goro writes a dummy value to 'expectedStateCh' (which blocks) and the
// main goro doesn't read the dummy value (unblocking this goro) until the
// previous expected state has been reached.
func TestFuzz(t *testing.T) {
	dir, child := NewTestDir(t)
	block := make(chan struct{})

	// use 'paths' datatype for extra Watch() features
	var expected paths = map[string]struct{}{
		dir:   struct{}{},
		child: struct{}{},
	}
	actual := expected.Copy()
	expectedStateCh := make(chan paths)

	// Modify the filesystem, and every time we do, modify the 'expected' state
	// and write a copy to 'expectedStateCh'.
	var eg errgroup.Group
	eg.Go(func() error {
		<-block
		for i := 0; i < 100; i++ {
			if len(expected) == 2 || testRand.Int31n(2) > 0 {
				baby := p.Join(expected.Pick( /* except */ child), fmt.Sprintf("%03d", i))
				MkdirT(t, baby, 0755)
				expected.Add(baby)
			} else {
				dead := expected.Pick( /* except */ dir, child)
				check.T(t, check.Nil(os.RemoveAll(dead)))
				expected.Rm(dead)
			}
			expectedStateCh <- expected.Copy()

			// insert dummy value into channel; this line blocks until the target
			// state has been reached so the next change can be applied (or close the
			// channel if we're done)
			if i+1 < 100 {
				expectedStateCh <- nil
			} else {
				close(expectedStateCh)
			}
		}
		return nil
	})

	// Initiate Watch
	// N.B. we pass WatchEvents through a channel because a single operation above
	// may generate multiple events (e.g. "delete foo/" would generate Delete
	// events for "foo/a/", "foo/b/", and "foo/c/"), and we want to apply all
	// events generated by a single operation before comparing end states.
	eventCh := make(chan WatchEvent)
	ctx, cancel := context.WithCancel(context.Background())
	eg.Go(func() error {
		return Watch(ctx, dir, func(e WatchEvent) error {
			if e.Type == Create && e.Path == child {
				close(block) // start goro making modifications
				return nil
			}
			if e.Type != Create && e.Type != Delete {
				return fmt.Errorf("unexpected event: %s", e)
			}
			eventCh <- e
			return nil
		})
	})

	// read through expected states and try to reach each one--note that
	// filesystem modifications are blocked on each expected state being reached
	// (roughly -- real FS is typically one step ahead) so there's no way for some
	// random run of deletes to mask the fact that a particular state was never
	// reached
	target, ok := <-expectedStateCh
	for {
		// Apply pending watch events until there are none left (deleting a dir w/
		// lots of children may generate a lot of events)
		select {
		case e := <-eventCh:
			if e.Type == Create {
				actual.Add(e.Path)
			} else if e.Type == Delete {
				actual.Rm(e.Path)
			} else {
				t.Fatalf("unexpected event type: " + e.String())
			}
			continue
		case <-time.After(500 * time.Millisecond):
			// TODO ideally don't use timing. use a semaphore or some such
			break // no more events arriving
		}
		// Make sure Watch events put 'actual' into the right target state
		check.T(t, check.Eq(actual, target))

		// pull dummy state out of channel to unblock generator goro (or exit)
		if _, ok = <-expectedStateCh; !ok {
			break
		}
		// Wait for new expected state
		target, ok = <-expectedStateCh
	}
	cancel()
	check.T(t, check.Nil(eg.Wait()))
}

// TestFuzzAsync is similar to TestFuzz, but the goroutine that generates events
// doesn't wait for the gorouting that receieves events to catch up before
// continuing. Because of that, this test is more lax--it's very easy for events
// to be re-ordered (because watches take a non-trivial amount of time to
// establish) so instead of checking every state, we just confirm that every
// create and every delete is seen in some order.
//
// The main benefit of this test over TestFuzz is that it's good at confirming
// that Watch() doesn't have any race conditions with files being deleted
// shortly after being created that cause it to panic.
func TestFuzzAsync(t *testing.T) {
	dir, child := NewTestDir(t)
	block := make(chan struct{})
	var expected paths = map[string]struct{}{
		dir:   struct{}{},
		child: struct{}{},
	}
	expectedCreates, expectedDeletes := make(paths), make(paths)

	// Randomly modify 'dir' and make sure modifications are seen by Watch.
	// 'ctx' is cancelled by this goro once it's done making modifications
	ctx, cancel := context.WithCancel(context.Background())
	var eg errgroup.Group
	eg.Go(func() error {
		<-block // yield to calling goro
		for i := 0; i < 100; i++ {
			if len(expected) == 2 || testRand.Int31n(2) > 0 {
				baby := p.Join(expected.Pick( /* except */ child), fmt.Sprintf("%03d", i))
				MkdirT(t, baby, 0755)
				expected.Add(baby)
				expectedCreates.Add(baby)
			} else {
				dead := expected.Pick( /* except */ dir, child)
				check.T(t, check.Nil(os.RemoveAll(dead)))
				for path := range expected {
					if strings.HasPrefix(path, dead) {
						expectedDeletes.Add(path) // inotify generates events for children
					}
				}
				expected.Rm(dead)
			}
			// hack -- briefly yield control; if a dir is created and deleted before
			// the watcher can even scan its parent, it might not generate an event
			// and this test would break
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
		return nil
	})

	// Initiate Watch
	actualCreates, actualDeletes := make(paths), make(paths)
	eg.Go(func() error {
		return Watch(ctx, dir, func(e WatchEvent) error {
			if e.Type == Create && e.Path == child {
				close(block) // start goro making modifications
				return nil
			}
			// Apply pending watch events until we match 'expected'
			if e.Type == Create {
				actualCreates.Add(e.Path)
			} else if e.Type == Delete {
				actualDeletes.Add(e.Path)
			} else {
				t.Fatalf("unexpected event type: " + e.String())
			}
			return nil
		})
	})

	check.T(t, check.Nil(eg.Wait()))
	time.Sleep(time.Second) // hack -- wait for pending WatchEvents to be applied
	check.T(t,
		check.Eq(actualCreates, expectedCreates),
		check.Eq(actualDeletes, expectedDeletes))
}

// TestFiles is similar to other tests--it creates a watched directory, alters
// its contents, and checks that the alterations are detected by the watcher.
func TestFiles(t *testing.T) {
	dir, child := NewTestDir(t)
	block := make(chan struct{})
	expectedCreates, expectedModifications := make(paths), make(paths)

	// Create and modify several files in 'dir' and make sure modifications
	// are seen by Watch
	// 'ctx' is cancelled by this goro once it's done making changes
	ctx, cancel := context.WithCancel(context.Background())
	var eg errgroup.Group
	eg.Go(func() error {
		<-block
		for i := 0; i < 100; i++ {
			baby := p.Join(dir, fmt.Sprintf("%03d", i))
			f, err := os.Create(baby)
			check.T(t,
				check.Nil(err),
				check.Nil(f.Close()))
			expectedCreates.Add(baby)
		}
		for i := 0; i < 100; i++ {
			adult := p.Join(dir, fmt.Sprintf("%03d", i))
			f, err := os.OpenFile(adult, os.O_WRONLY|os.O_APPEND, 0)
			check.T(t, check.Nil(err))
			f.Write([]byte("x"))
			check.T(t, check.Nil(f.Close()))
			expectedModifications.Add(adult)
			// hack -- briefly yield control to give the watcher time to read and
			// process pending events
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
		return nil
	})

	// Initiate Watch
	actualCreates, actualModifications := make(paths), make(paths)
	eg.Go(func() error {
		return Watch(ctx, dir, func(e WatchEvent) error {
			if e.Type == Create && e.Path == child {
				close(block) // start goro making modifications
				return nil
			}
			// Apply pending watch events until we match 'expected'
			if e.Type == Create {
				actualCreates.Add(e.Path)
			} else if e.Type == Modify {
				actualModifications.Add(e.Path)
			} else {
				t.Fatalf("unexpected event type: " + e.String())
			}
			return nil
		})
	})

	check.T(t,
		check.Nil(eg.Wait()),
		check.Eq(actualCreates, expectedCreates),
		check.Eq(actualModifications, expectedModifications))
}

// copied from watch_daemon/api_test.go
func TestMain(m *testing.M) {
	var errCode int
	defer func() {
		// make sure test failures case the test binary to return non-zero exit code
		os.Exit(errCode)
	}()

	// create temporary directory for housing test data
	var err error
	// /dev/shm is a pre-mounted in-memory filesystem that exists by default on
	// most linux distros (incl. ubuntu on my laptop)
	testDir, err = ioutil.TempDir("/dev/shm", "time-tracker-test-")
	if err != nil {
		panic(fmt.Sprintf("could not create temporary directory for test data: %v", err))
	}
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			panic(fmt.Sprintf("could not remove temp testing directory: %v", err))
		}
	}()

	// In-memory test dir created--run tests
	errCode = m.Run()
}
