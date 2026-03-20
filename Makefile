.PHONY: generate build install test clean

VERSION ?= dev
LDFLAGS := -s -w -X github.com/sofq/confluence-cli/cmd.Version=$(VERSION)

generate:
	go run ./gen/...

build:
	go build -ldflags "$(LDFLAGS)" -o cf .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

clean:
	rm -f cf
	rm -f cmd/generated/*.go
