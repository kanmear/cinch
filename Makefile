.PHONY: build install setup-hooks version-show version-major release push-release test
VERSION ?= $(shell scripts/update-version.sh show 2>/dev/null || echo 0.1.0)
LDFLAGS := -X main.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/cinch .
	go vet ./...

test:
	go test ./...

install:
	go install -ldflags "$(LDFLAGS)" .

setup-hooks: install
	go run . init

version-show:
	@scripts/update-version.sh show

version-major:
	@scripts/update-version.sh major

release:
	@scripts/release.sh

push-release:
	git push --follow-tags origin main dev
