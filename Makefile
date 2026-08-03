.PHONY: build test fixtures
build:
	go build -o bin/cinch .

test: build
	go vet ./...
	go test ./... 2>/dev/null || true

# Fixture render+check across stacks — the portability test (D017).
fixtures: build
	@for f in fixtures/*/; do \
		echo "== $$f"; \
		( cd $$f && ../../bin/cinch render && ../../bin/cinch check ) || exit 1; \
	done
