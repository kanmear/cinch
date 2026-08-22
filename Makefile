.PHONY: build install
VERSION ?= 0.1.0
LDFLAGS := -X main.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/cinch .
	go vet ./...

install:
	go install -ldflags "$(LDFLAGS)" .
