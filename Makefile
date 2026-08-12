.PHONY: build install test
build:
	go build -o bin/cinch .

# The generated hook shims exec a bare `cinch`, so the binary has to be on
# PATH for self-enforcement to run at all — this is how it gets there.
install:
	go install .

test: build
	go vet ./...
	go test ./...
