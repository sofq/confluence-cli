.PHONY: generate build install test clean lint spec-update docs-generate docs-dev docs-build docs

VERSION ?= dev
LDFLAGS := -s -w -X github.com/sofq/confluence-cli/cmd.Version=$(VERSION)
SPEC_URL := https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json

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

lint:
	golangci-lint run

spec-update:
	curl -sL "$(SPEC_URL)" -o spec/confluence-v2.json

docs-generate:
	go run ./cmd/gendocs/... website

docs-dev: docs-generate
	cd website && npx vitepress dev

docs-build: docs-generate
	cd website && npx vitepress build

docs: docs-build
