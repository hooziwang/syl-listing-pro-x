.PHONY: default test build clean

DEFAULT_GOAL := default

default: test build

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/syl-listing-pro-x .

clean:
	rm -rf bin
