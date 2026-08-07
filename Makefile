.PHONY: build test
build:
	go build -o bin/cinch .

test: build
	go vet ./...
	go test ./...
	./bin/cinch check
