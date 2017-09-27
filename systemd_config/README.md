I've found this tool most useful when I'm running the watch daemon as a systemd
service. This way, I don't have to worry about losing credit for work because
the OOM killer killed my watch daemon or some such.

Fortunately systemd supports user-level services[1], and fortunately, they are
easy to create. Simply copy `golang-time-tracker.service` in this directory to
`${HOME}/.config/systemd/user/golang-time-tracker.service` and then run:

```
$ systemctl --user start golang-time-tracker
$ systemctl --user enable golang-time-tracker # always run on startup
```

[1] https://wiki.archlinux.org/index.php/Systemd/User
