GOPATH ?= $(shell go env GOPATH)

# Ensure GOPATH is set before running build process.
ifeq "$(GOPATH)" ""
  $(error Please set GOPATH before running `make`)
endif

.PHONY: build
build: clean
	go build -o bin *.go

.PHONY: build-x86
build-x86: clean
	GOOS=linux GOARCH=amd64 go build -o x86-bin *.go

.PHONY: run
run: build
	./bin

.PHONY: clean
clean:
	rm -f bin
	rm -rf polaris
