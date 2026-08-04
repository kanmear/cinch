.PHONY: build test setup-hooks version-show version-major release
build:
	go build -o bin/cinch .

test: build
	go vet ./...
	go test ./...

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
