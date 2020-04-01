DEBUG=false
OUTPUT_NAME=generated_assets.go
# PKG=$(shell basename "${PWD}")
PKG=watchd

go-bindata:
	which go-bindata || go get -u github.com/go-bindata/go-bindata/...

$(OUTPUT_NAME): go-bindata
	rm -f $(OUTPUT_NAME)
	echo $(PKG)
ifeq '$(DEBUG)' 'true'
	@# If you want to change the output, just use -pkg and -o to change the package and destination file
	go-bindata -debug -pkg $(PKG) -o $(OUTPUT_NAME) ./assets/*
else
	go-bindata -pkg $(PKG) -o $(OUTPUT_NAME) ./assets/*
endif

test-interactive: $(OUTPUT_NAME)
	INTERACTIVE=true go test -v . -run TestInteractive -count 1

.PHONY: \
	test-interactive \
	go-bindata \
	$(OUTPUT_NAME)
