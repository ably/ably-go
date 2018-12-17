all: vet build test

vet:
	go vet ./ably ./scripts

build:
	go build ./...

test: submodules
	ABLY_PROTOCOL=application/json go test -p 1 -race -v ./...
	ABLY_PROTOCOL=application/x-msgpack go test -p 1 -race -v ./...

submodules:
	git submodule update --init

.PHONY: all vet build test submodules
