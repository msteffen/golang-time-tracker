DEBUG=false

bindata:
	which bindata || { \
	  go get github.com/jteeuwen/go-bindata && \
	  cd $${GOPATH}/src/github.com/jteeuwen/go-bindata/go-bindata && \
	  go install .; \
	}

bindata.go: bindata
	rm bindata.go
ifeq '$(DEBUG)' 'true'
	@# If you want to change the output, just use -pkg and -o to change the package and destination file
	go-bindata -debug -pkg webui ./today.html.template
else
	go-bindata -pkg webui ./today.html.template
endif

.PHONY: bindata bindata.go
