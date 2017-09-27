# golang-time-tracker

Little app for tracking how much work I do in a day

You can start the server with:
```
t serve &
```

You can start watching 'dir' for writes with:
```
t watch dir &
```

You can see the time you've spend working with:
```
$ t
```
or
```
$ t week
```

Finally, you can query the server manually with:
```
curl \
  --unix-socket ${HOME}/.time-tracker/sock \
  http://unix/${HOME}/.time-tracker/sock/intervals
```

# Design

Time-tracker has three parts:
1. A CLI (completely stateless)
2. A local watcher daemon/API server (state = set of inotify FDs)
3. A SQL database (set of writes, and set of watched directories)

The CLI (1) communicates with the watcher daemon (2) via HTTP over a local
socket. Depending on the nature of the call, the watcher daemon can:
- Begin watching a local directory (by writing it to the DB as a directory
  that needs to be watched, and then installing inotify watches on it and its
  subdirectories)
  - When writes do occur, they're written to the SQL database
- Report "work intervals" (i.e. periods of time in which writes were happening)
