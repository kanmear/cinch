.PHONY: build install setup-hooks version-show version-major release push-release test catch-matrix cost-curve
VERSION ?= $(shell scripts/update-version.sh show 2>/dev/null || echo 0.1.0)
LDFLAGS := -X main.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/cinch .
	go vet ./...

test:
	go test ./...

catch-matrix:
	go run ./bench/catchmatrix

# The cost curve times the current tree against BENCH_BASELINE, the last
# commit before the marker scan skipped binary files (CINCH-009).
BENCH_BASELINE ?= 74caaf0
BENCH_BASELINE_VERSION ?= 0.9.0

cost-curve:
	@d=$$(mktemp -d) && trap 'rm -rf "$$d"' EXIT && \
	mkdir "$$d/base" && git archive $(BENCH_BASELINE) | tar -x -C "$$d/base" && \
	(cd "$$d/base" && go build -ldflags "-X main.Version=$(BENCH_BASELINE_VERSION)" -o "$$d/cinch-base" .) && \
	go build -ldflags "$(LDFLAGS)" -o "$$d/cinch-current" . && \
	go run ./bench/costcurve -cinch "$(BENCH_BASELINE_VERSION)=$$d/cinch-base,$(VERSION)=$$d/cinch-current" $(BENCH_ARGS)

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
