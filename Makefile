all: vet build test

vet:
	go get golang.org/x/tools/cmd/vet
	find . -type f -name '*.go' -not -path './Godeps/*' | xargs -L 1 go vet -x

build:
	go build ./...

test: submodules
	ABLY_PROTOCOL=application/json go test -p $(GOMAXPROCS) -race -v ./...
	ABLY_PROTOCOL=application/x-msgpack go test -p $(GOMAXPROCS) -race -v ./...

submodules:
	git submodule update --init

.PHONY: all vet build test submodules
