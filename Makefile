test:
	cd ./pkg/watcher && make test # watcher lib
	cd ./watch_daemon && make test # http server/daemon
	go test -v ./pkg/escape -count 1
	go test -v ./cli/t -count 1
	go test -v ./pkg/check -count 1

clean-test-data:
	rm -rf /dev/shm/time-tracker-test-*

.PHONY: test
