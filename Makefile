.PHONY: build test fixtures setup-hooks version-show version-major release
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

# Versioning (see ops/git-conventions.md — minor/patch/hotfix bump
# automatically via .githooks/post-commit on dev/main squash-merges; major is
# manual-only).
setup-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/commit-msg .githooks/post-commit

version-show: build
	@./bin/cinch version show

version-major: build
	@./bin/cinch version bump major

release:
	@./scripts/release.sh
