DEBUG=false
OUTPUT_NAME=generated_template_assets.go
# PKG=$(shell basename "${PWD}")
PKG=watchd

go-bindata:
	which go-bindata || go get -u github.com/go-bindata/go-bindata/...

$(OUTPUT_NAME): go-bindata
	rm -f $(OUTPUT_NAME)
	echo $(PKG)
ifeq '$(DEBUG)' 'true'
	@# If you want to change the output, just use -pkg and -o to change the package and destination file
	go-bindata -debug -pkg $(PKG) -o $(OUTPUT_NAME) ./templates/*
else
	go-bindata -pkg $(PKG) -o $(OUTPUT_NAME) ./templates/*
endif

.PHONY: go-bindata $(OUTPUT_NAME)
