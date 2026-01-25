.PHONY: all build test lint install clean check

all: check build

build:
	go build -o orbital ./cmd/orbital

test:
	go test -race ./...

lint:
	golangci-lint run ./...

check: lint test

install:
	go install ./cmd/orbital

clean:
	rm -f orbital coverage.out
