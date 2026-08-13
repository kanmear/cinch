.PHONY: build install test
VERSION ?= 0.1.0
LDFLAGS := -X main.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/cinch .

# The generated hook shims exec a bare `cinch`, so the binary has to be on
# PATH for self-enforcement to run at all — this is how it gets there.
install:
	go install -ldflags "$(LDFLAGS)" .

test: build
	go vet ./...
	go test ./...
