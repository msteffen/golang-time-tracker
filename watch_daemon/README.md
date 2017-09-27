# Description

api.go contains the "core" implementation of the watch daemon API. The rest of
the `watch_daemon` package mostly serves to wrap `api.go` in HTTP serving and
serialization logic
