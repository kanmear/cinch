# harness-template v3
# go-cli stack of the harness template (one Go command-line tool: main.go at the repo root, no
# server, released as binaries by goreleaser). Seeded once, then the repo's own.
# Repo extras beyond the template's contract: install-bin.

# ===== PARAMETERS: the only block to edit when adopting the template =====
# Repo and binary name; `make build` writes $(BIN_DIR)/$(NAME).
NAME := cinch
# Where `make build` puts the binary. `bin` (gitignore it) keeps a build out of the way; point it
# at a directory on PATH (BIN_DIR := $(HOME)/.local/bin) to make a build also the install.
BIN_DIR ?= bin
# There are no ports, DEPLOY_HOST or ENV_FILE here: a CLI is delivered as a tagged release that
# goreleaser builds in CI. Add PORT_BASE and a require-free-ports guard (see the go stack) only if
# the repo grows a server or a fake service that `make dev` has to start.
# The Go variable `-X` stamps the version into. The template's default is `var version` in
# package main; cinch's is the exported `var Version` in main.go. `.goreleaser.yml` hard-codes the
# same variable in its `-X` (goreleaser cannot read make variables); change both together.
VERSION_VAR ?= main.Version
# ===== end parameters =====

.DEFAULT_GOAL := help

.PHONY: help install install-bin dev run down check test format build merge release ship deploy logs version version-major prune require-cinch

# GUI git clients and non-login shells (including cinch's generated git hooks)
# don't source shell rc files, so mise's shims and cinch's install location can be missing
# from PATH. Prepend them for every recipe, not just the hook.
export PATH := $(HOME)/.local/share/mise/shims:$(HOME)/.local/bin:$(HOME)/go/bin:$(PATH)

CINCH ?= cinch

# Runs a command quietly, printing one ✓/✗ line (full output with V=1);
# see scripts/step.sh. Used by the git-flow targets.
STEP := ./scripts/step.sh

# The repo's one version (scripts/version.sh), stamped into the binary as $(VERSION_VAR)
# (`var Version = "dev"` in main.go; goreleaser stamps the same variable in CI, so
# .goreleaser.yml names it too).
VERSION := $(shell ./scripts/version.sh show 2>/dev/null)
LDFLAGS := -X $(VERSION_VAR)=$(VERSION)

# tmux session a repo-local dev helper may start (a fake server, a log tail); `make down`
# stops it. `make dev` itself is one foreground process.
DEV_SESSION := $(NAME)-dev

require-cinch:
	@command -v $(CINCH) >/dev/null 2>&1 || { echo "cinch is not installed or not on PATH (looked for '$(CINCH)')."; echo "Install cinch (the version cinch.yml requires), then re-run: make $(MAKECMDGOALS)"; exit 1; }

help:
	@echo "=== $(NAME) ==="
	@echo "  make install       - go mod download, .env from .env.example if there is one, cinch init (idempotent)"
	@echo "  make dev           - Run the CLI in the foreground (go run . \$$(ARGS)), same as make run"
	@echo "  make down          - Stop the dev tmux session's processes and kill it, if there is one"
	@echo "  make check         - Static and read-only: go vet, gofmt"
	@echo "  make test          - The Go test suite (TESTFLAGS='-run X -count=50' passes flags to go test)"
	@echo "  make format        - gofmt -w"
	@echo "  make build         - The $(NAME) binary into $(BIN_DIR) (version-stamped; BIN_DIR=... to move it)"
	@echo "  make merge         - From a topic branch: squash onto main as 'merge: <branch>' (the hook bumps the version)"
	@echo "  make release       - Tag v<version> on main, push main + tag to origin"
	@echo "  make ship          - release; the tag push runs goreleaser in CI and publishes the GitHub release"
	@echo "  make deploy        - Not applicable (no server): use make ship"
	@echo "  make logs          - Not applicable (nothing keeps running)"
	@echo "  make version       - Print the version ($(VERSION))"
	@echo "  make version-major - Bump the major by hand"
	@echo "  make prune [confirm=1] - List (or delete) topic branches whose squash landed on main"
	@echo ""
	@echo "More:"
	@echo "  make run ARGS=...  - go run . \$$(ARGS), e.g. make run ARGS='version' or ARGS='check'"
	@echo "  make install-bin   - go install the version-stamped binary onto PATH (how the owner installs cinch; mise shims it)"
	@echo "  V=1 shows every step's full output (logs: .git/make-logs/)"

install: require-cinch
	go mod download
	@if [ ! -f .env ] && [ -f .env.example ]; then cp .env.example .env; echo "Created .env from .env.example"; fi
	$(CINCH) init

# Dev loop: make run ARGS='version'. Make pastes $(ARGS) into the shell line, so inner quotes
# survive.
# A bare `cinch` prints the synopsis and exits 2, so ARGS defaults to `help` to keep `make dev`
# (= run) exiting 0.
ARGS ?= help

run:
	go run -ldflags "$(LDFLAGS)" . $(ARGS)

# The CLI is one foreground process: dev is run.
dev: run

# Stops the dev session, silent when there is none.
# C-c goes to every pane first: SIGINT reaches the pane's whole foreground process group, so
# `go run`'s child binary or Air's backend stops too, where the SIGHUP of `kill-session` alone
# leaves it running and holding its port. The session ends by itself once its panes exit; after
# 5 s it is killed anyway. `=` makes tmux match the session name exactly, not as a prefix.
down:
	@if command -v tmux >/dev/null 2>&1 && tmux has-session -t '=$(DEV_SESSION)' 2>/dev/null; then \
		for p in $$(tmux list-panes -s -t '=$(DEV_SESSION)' -F '#{pane_id}'); do tmux send-keys -t "$$p" C-c; done; \
		i=0; while [ $$i -lt 50 ] && tmux has-session -t '=$(DEV_SESSION)' 2>/dev/null; do sleep 0.1; i=$$((i + 1)); done; \
		tmux kill-session -t '=$(DEV_SESSION)' 2>/dev/null; \
		echo "Stopped tmux session '$(DEV_SESSION)'"; \
	fi

# Static and read-only: no tests, no file rewrites. gofmt takes only tracked .go files. Add
# repo-local static checks as extra lines here.
check:
	go vet ./...
	@unformatted=$$(gofmt -l $$(git ls-files '*.go')); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed (run 'make format'):"; echo "$$unformatted"; \
		exit 1; \
	fi

# TESTFLAGS passes extra flags to go test; a later -count wins over the default, e.g.
# make test TESTFLAGS='-run TestName -count=200'.
TESTFLAGS ?=

test:
	go test -count=1 $(TESTFLAGS) ./...

format:
	gofmt -w $$(git ls-files '*.go')

# Same flags as .goreleaser.yml (static, trimmed, symbols stripped), so a local build behaves
# like the released one.
build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(BIN_DIR)/$(NAME) .

# How the owner installs cinch: the version-stamped binary lands in $(go env GOBIN) or
# ~/go/bin, which the mise shims put on PATH. A go install, so no BIN_DIR.
install-bin:
	go install -ldflags "$(LDFLAGS)" .

# There is no server to deploy to; delivery is the release, which `make ship` runs.
deploy:
	@echo "deploy: not applicable to a CLI; use 'make ship' (release; the tag push runs goreleaser)"; exit 2

# A CLI is not a running service, so there is nothing to tail.
logs:
	@echo "logs: not applicable to a CLI (nothing keeps running); run it with 'make run ARGS=...'"; exit 2

# release, which is the delivery: the tag push triggers .github/workflows/release.yml, and
# goreleaser builds the binaries and publishes the GitHub release. Separate recipe lines so make
# stops at the first failure. Keep the echo lines apart from the $(MAKE) lines: only lines that
# contain $(MAKE) run under `make -n`, and they must stay free of side effects.
ship:
	@echo "==> release"
	@$(MAKE) --no-print-directory V=$(V) release
	@echo "==> shipped: goreleaser builds the release in CI (.github/workflows/release.yml)"

# ===== Git flow (trunk on main; see .docs/git-conventions.md) =====

# Run from a topic branch: squashes it onto main as `merge: <branch>`. A squash and not
# --no-ff on purpose: the post-commit hook bumps the version only for a single-parent
# commit, so make does not bump. No --no-verify either: the hooks run the checks.
# Stays on main; the topic branch is kept (`make prune` removes it once it has landed).
# Each step prints one ✓/✗ line (the hooks' noise goes to .git/make-logs/; V=1 shows it),
# then a summary: the new commit, its size and which version moved.
# A branch that adds nothing to main (already merged) is detected up front with a
# merge-tree dry run and prints one line without leaving the branch.
# step.sh runs from a temp copy because main may not have it yet (the branch that
# introduces or changes it).
merge:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Refusing to merge: working tree is not clean"; exit 1; \
	fi
	@branch=$$(git branch --show-current); \
	case "$$branch" in \
		main) echo "Refusing to merge: already on main (run it from a topic branch)"; exit 1 ;; \
		feature/*|fix/*|refactor/*|perf/*|ui/*|chore/*|docs/*) ;; \
		*) echo "Refusing to merge: '$$branch' is not a feature/*, fix/*, refactor/*, perf/*, ui/*, chore/* or docs/* branch (a hotfix is a fix/ branch)"; exit 1 ;; \
	esac; \
	merged=$$(git merge-tree --write-tree main "$$branch" 2>/dev/null | head -n 1); \
	if [ -n "$$merged" ] && [ "$$merged" = "$$(git rev-parse 'main^{tree}' 2>/dev/null)" ]; then \
		echo "  = $$branch is already merged into main"; \
		exit 0; \
	fi; \
	step=$$(mktemp) && cp scripts/step.sh "$$step" && trap 'rm -f "$$step"' EXIT; \
	sh "$$step" "checkout main" git checkout main || exit 1; \
	before=$$(./scripts/version.sh show 2>/dev/null || echo "?"); \
	if ! sh "$$step" "squash $$branch" git merge --squash "$$branch"; then \
		conflicted=$$(git diff --name-only --diff-filter=U); \
		if [ -n "$$conflicted" ]; then \
			echo "  conflicted files:"; echo "$$conflicted" | sed 's/^/    /'; \
			echo "merge: conflicts. Resolve them, 'git add' the files, then: git commit -m \"merge: $$branch\""; \
		else \
			echo "merge: the squash failed (see above); still on main"; \
		fi; \
		exit 1; \
	fi; \
	if git diff --cached --quiet; then \
		git checkout -q "$$branch"; \
		echo "  = $$branch is already merged into main"; \
		exit 0; \
	fi; \
	if ! sh "$$step" "commit (checks + version bump)" git commit -m "merge: $$branch"; then \
		echo "merge: commit failed (a pre-commit check?). The squash is still staged on main: fix it, then: git commit -m \"merge: $$branch\""; \
		exit 1; \
	fi; \
	after=$$(./scripts/version.sh show 2>/dev/null || echo "?"); \
	stat=$$(git diff --shortstat HEAD~1 HEAD | awk '{ for (i = 2; i <= NF; i++) { \
		if ($$i ~ /^file/) f = $$(i - 1); else if ($$i ~ /^insertion/) a = $$(i - 1); else if ($$i ~ /^deletion/) d = $$(i - 1) } } \
		END { printf "%d file%s, +%d −%d", f, (f == 1 ? "" : "s"), a, d }'); \
	echo "✓ $$branch → main  $$(git log -1 --format='%h "%s"')  ($$stat)"; \
	if [ "$$before" = "$$after" ]; then echo "  version $$before unchanged"; else echo "  version $$before → $$after"; fi

# Tags v<version> on main and pushes main and the tag to origin (scripts/release.sh).
release:
	@./scripts/release.sh

# One version per repo: print it, or bump the major by hand (merges bump minor/patch).
version:
	@./scripts/version.sh show

version-major:
	@./scripts/version.sh major

# Lists local topic branches whose squash already landed on main; confirm=1 deletes them.
prune:
	@./scripts/prune.sh $(if $(filter 1,$(confirm)),--confirm,)
