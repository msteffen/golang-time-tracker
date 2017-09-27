test:
	cd ./pkg/watcher && make test # watcher lib
	cd ./watch_daemon && make test # http server/daemon
	go test -v ./pkg/escape
	go test -v ./cli/t
	go test -v ./pkg/check

clean-test-data:
	rm -rf /dev/shm/time-tracker-test-*

.PHONY: test
