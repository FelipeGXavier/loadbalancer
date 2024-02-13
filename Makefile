SHELL = /bin/bash
export PATH := $(shell yarn global bin):$(PATH)

default: lint test

test:
	go test -v ./...

lint:
	golangci-lint run --verbose