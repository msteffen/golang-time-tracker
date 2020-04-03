## 3/25/2020
- Coming back to this project after a bit of a hiatus 
- I think I'm working on the vizualization piece of this change right now. 
- Next will be the new data model
- [ ] Do I want to serve most endpoints (e.g. /tick) on a socket, while serving the viz endpoint on a port? It might make client initialization easier...


--
## 1/2/2020 - 1/5/2020
- [x] Import D3 library to template
- [x] Replace 5 divs with 5 ~~canvases~~ svg elements
- [ ] Draw an arc that doesn't depend on anything in each canvas
- [ ] Bind interval data to each arc
- [ ] Draw intervals for real
- [ ] Get interval updates every second
- [ ] **Optional** Graph on the bottom showing last month?

---
## 12/31/2019
I'm getting an error after switching everything from socket files to a port. TestGetIntervalsBoundary is failing with UNIQUE constraint error. I know I've seen this before because this exact problem is described in one of my comments:
```
// even though 'tmpDir' is a tempdir, it's shared by all tests currently
// running. If we don't remove the existing tmp directory for invocation of
// StartTestServer, then go test ... -count=N won't work, as all runs of a
// given test will share the same DB
```
However, I'm deleting the directory containing the DB file between each test invocation (as verified by adjacent logging)

**Hypothesis**: When the test starts, there's existing data in the database<br/>
**Experiment**: Print existing database entries on test startup (I'm sure I have a code snippet somewhere for doing that) (in SEPI4)<br/>
**Result**: Disproved hypothesis. I open the new DB file and query it and there's nothing in there<br/>

**Hypothesis**: When the test starts and initializes a client, the new client is talking to the old server goro<br/>
**Experiment**: Explicitly shutdown the HTTP server created by StartTestServer (by calling `s.httpServer.Shutdown()`) at the end of each test<br/>
**Result**: Failed to disprove hypothesis. Once I added this line, subsequent tests started passing<br/>

**Question**: Why wasn't I seeing an error when attempting to start an HTTP server while the port was still bound by the old HTTP server?<br/>
**Answer**: I was, I was just discarding it. I was doing `go httpServer.ListenAndServe()`; once I checked the result for errors and called `t.Fatalf()` if `err != nil`, then I saw the `address already in use` error<br/>

**Question**: Do I want to do this for all tests? Is there some way I can cause the old goro to shut down when the new test runs?<br/>
**Answer**: Just have StartTestServer register the most recent httpServer that it's created, and shut that down (if any) on each call<br/>

**Question**: Why doesn't deleting the file cause the old goro to crash?<br/>
**Answer**: I think because of the linux behavior in which an unlinked file remains on disk until its last open file descriptor is closed.<br/>

---
## 12/26/2019
#### TODO:
- [ ] Finish HTML homepage rendering of time spent
  - **Why**: I want an HTML home page showing the time I've spent that shows up in new tabs
- [ ] Change DB representation so that ticks are packed into intervals
  - **Why** The DB is already kind of fat after not using this for so long, so I think this will help a lot with size
- [ ] Change DB representation so that project names are stored in a DB w/ a unique key
  - **Why** Again, I think this will shrink the size of the DB
  - **Note**: Five base64 digits allows for a billion projects. Hopefully plenty. Though, there's actually no reason to limit this.
- [x] Switch from socket back to HTTP server (or launch both. But I want my time-tracker homepage)
  - **Why**: I want an HTML home page showing the time I've spent that shows up in new tabs, and I have to serve that page from an endpoint that my browser can read
  - **Note**: Check if my browser can read HTML from a socket. I might be able to strike this
- [x] Create a new endpoint that returns some kind of HTML to your browser
  - **Why**: This will be easier than wiring up d3.js, but is a necessary first step
---
## 7/23/2019

- Figured out TestMaxWatches (had to do with a preexisting DB bug that only arose in certain circumstances)
  - (12/26/2019) Updating this waay after the fact, but I think it was a classic case of spawning a goroutine in a for loop (the syncWatches() 'create' case) but closing over a loop variable in the goroutine so that it got updated before the goro got scheduled and then it watched the wrong thing
- Currently, the restart test that I just added doesn't seem real to me. When I run it, I see a bunch of log messages for writes that happen after the restart, but no corresponding log messages for writes before the restart. Are the writes really being picked up??

---
## 7/21/2019
Issue: TestMaxWatches is failing. Says EndGap is 900 when it should be 300. To me this strongly suggests that the writes being made to the test directory aren't being received
- When TestMaxWatches is setting up N (currently 8) test directories, the failure always seems to occur w/ the 8th


**Hypothesis**: watch isn't being established on 8th directory at all<br/>
**Experiment**: logging<br/>
**Result**: <span style="color:DarkRed">Disproved -- watch library receives req for dir #8, and when I query the /watches endpoing before running the failing test, I see dir #8 show up</span><br/>

**Hypothesis**: watch is recieved but is not set up in time for the test<br/>
**Experiment**: Longer sleep? factor out locking portion of /watch into separate method so that it only returns once the inotify watch is established?<br/>
**Result**: <span style="color:DarkRed">disproved hypthesis (partially): waiting longer between establishing the watch and issuing the writes didn't fix the issue</span><br/>

**Hypothesis**: Some DB is preventing the writes from being persisted. In the test logs, I saw:<br/>
```
time="2019-07-22T09:34:59-07:00" level=info msg="/watch &client.WatchRequest{Label:\"test\", Dir:\"/dev/shm/time-tracker-test-814828970/TestMaxWatches-6\"} -> <nil>"
>>> event "Create \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-6/file-81\""
...
time="2019-07-22T09:35:19-07:00" level=info msg="/watch &client.WatchRequest{Label:\"test\", Dir:\"/dev/shm/time-tracker-test-814828970/TestMaxWatches-7\"} -> <nil>"
>>> event "Create \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-7/file-84\""
time="2019-07-22T09:35:29-07:00" level=info msg="observed \"Create \\\"/dev/shm/time-tracker-test-814828970/TestMaxWatches-7/file-85\\\"\""
...
time="2019-07-22T09:35:40-07:00" level=info msg="/watch &client.WatchRequest{Label:\"test\", Dir:\"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8\"} -> <nil>"
# TWO create events
>>> event "Create \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-87\""
>>> event "Create \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-87\""
time="2019-07-22T09:35:49-07:00" level=error msg="error recording write at 2017-07-01 11:27:27 -0700 PDT for \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-1\" (will attempt to roll back): UNIQUE constraint failed: ticks.time"
# One failed!
time="2019-07-22T09:35:49-07:00" level=error msg="error rolling back txn: cannot rollback - no transaction is active"
# Rollback got executed twice as well

## Two events registered again
>>> event "Create \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-88\""
time="2019-07-22T09:35:50-07:00" level=info msg="observed \"Create \\\"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-88\\\"\""
>>> event "Create \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-88\""
time="2019-07-22T09:35:50-07:00" level=info msg="observed \"Create \\\"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-88\\\"\""

time="2019-07-22T09:35:52-07:00" level=info msg="watch on \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8\" (labeled \"test\") registered a change"
# How 
time="2019-07-22T09:35:52-07:00" level=error msg="error recording write at 2017-07-01 11:32:27 -0700 PDT for \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8\" (will attempt to roll back): cannot start a transaction within a transaction"
time="2019-07-22T09:35:52-07:00" level=error msg="error rolling back txn: cannot rollback - no transaction is active"
time="2019-07-22T09:35:52-07:00" level=info msg="watch on \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-1\" (labeled \"test\") registered a change"
time="2019-07-22T09:35:52-07:00" level=error msg="error recording write at 2017-07-01 11:32:27 -0700 PDT for \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-1\" (will attempt to roll back): cannot start a transaction within a transaction"
time="2019-07-22T09:35:52-07:00" level=error msg="error rolling back txn: cannot rollback - no transaction is active"
# Double rollback again

>>> event "Create \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-89\""
time="2019-07-22T09:35:53-07:00" level=info msg="observed \"Create \\\"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-89\\\"\""
>>> event "Create \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-89\""
time="2019-07-22T09:35:53-07:00" level=info msg="observed \"Create \\\"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8/file-89\\\"\""
time="2019-07-22T09:35:55-07:00" level=info msg="watch on \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8\" (labeled \"test\") registered a change"
time="2019-07-22T09:35:55-07:00" level=error msg="error recording write at 2017-07-01 11:37:27 -0700 PDT for \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-8\" (will attempt to roll back): cannot start a transaction within a transaction"
time="2019-07-22T09:35:55-07:00" level=info msg="watch on \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-1\" (labeled \"test\") registered a change"
time="2019-07-22T09:35:55-07:00" level=error msg="error rolling back txn: cannot rollback - no transaction is active"
time="2019-07-22T09:35:55-07:00" level=error msg="error recording write at 2017-07-01 11:37:27 -0700 PDT for \"/dev/shm/time-tracker-test-814828970/TestMaxWatches-1\" (will attempt to roll back): cannot start a transaction within a transaction"
time="2019-07-22T09:35:55-07:00" level=error msg="error rolling back txn: cannot rollback - no transaction is active"
time="2019-07-22T09:35:57-07:00" level=info msg="/intervals <- [1498892400, 1498978800]"
time="2019-07-22T09:35:57-07:00" level=info msg="/intervals [1498892400, 1498978800] -> (9 intervals + end gap, <nil>)"
time="2019-07-22T09:35:57-07:00" level=info msg=/watches
time="2019-07-22T09:35:57-07:00" level=info msg="/watches -> (8 watches, <nil>)"
```


---
## 7/16/2019
- [x] 't week' should print {H}h{M}m, and leave out seconds
- [x] 't week' should be the default value of 't'
- [x] 't week' should have a horizontal line between today and the previous days

---
## 7/15/2019
- I want changing the API for /intervals in several ways:
  - I want to add label markers to intervals indicating where the interval
    changed from one project to another (this seems simpler and cleaner to me
    than messing around with multiple "connected" intervals.
  - I want to change GetIntervalsResponse to include information on the
    interval that's in-progress (if any). This will be a separate field, and
    include information on when it would end if it were extended to server.now
  - I want to change the server such that intervals less than
    minEventLength aren't included in the result, unless the intervals are
    in-progress.
    - This means that Collector needs a notion of "current time" (ultimately
      from APIServer.Clock—see note below)
  - I want maxEventGap and minIntervalLength to be semi-configurable (at
    least in tests)
- These will mean changing a ton of tests.
  - I have a few tests in `interval_test.go`, which only test the collector.go
    library, but since collector.go should probably contain a backwards
    reference to an APIServer, I think I'll write tests in interval_test.go
    until I'm confident that the new behavior has been implemented correctly,
    and then move all of the tests (and all of the associated setting of
    Collector{.now,.minIntervalLength,.maxEventGap}) into the main
    api_test.go, and set the corresponding fields of APIServer there.
- **Abandoned** Rather than proactively extending the last interval and then
  setting 'EndGap', I think want to mostly do that kind of thing client-side.
  The client (e.g. bar.go) should detect when the last interval is close to
  the current time (i.e. the interval is "in-progress"), and manage the
  blinking and such.
  - The problem with this though is that then both the server and client need
    a notion of "now" (the server for deciding whether to discard short
    intervals, and the client for deciding how to render the intervals it
    gets).
  - Will I get sensible behavior if server and client don't have a shared
    clock? Will I need to have a client-side synthetic clock that I use all
    the places that currently use a server-side synthetic clock?
  - On reflection, I think I should do everything on the server. I'll start
    doing more on the client if it makes sense, but I'll have go back to the
    plan of having intervals include an "ExtendedEnd" field if theserver
    thinks they're in-progress.

---
## 7/13/2019
- Just finished refactoring `watch_daemon/api_impl.go` to keep watches in DB. Need
  to get that building & passing tests
- Update: tests now pass. Work is the HEAD commit of the 'wip' branch. Full of
  printfs and shit, though. Get that cleaned up and into master
