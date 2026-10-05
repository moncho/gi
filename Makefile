.DEFAULT_GOAL := help

# ── CPU throttling ──────────────────────────────────────────────────────
# Every recipe (builds, tests, the dev server) runs niced and pinned to a
# CPU subset so the machine stays usable and runs are reproducible. Tune per
# invocation, e.g. `make test CPU_SET=0-3 CPU_PROCS=4` or `CPU_NICE=0`.
#   CPU_NICE   nice level for every recipe
#   CPU_SET    taskset CPU list every recipe is pinned to (skipped where
#              taskset is unavailable, e.g. macOS, which has no CPU pinning)
#   CPU_PROCS  GOMAXPROCS and Go build parallelism (-p)
# Make itself is .NOTPARALLEL, and test* targets run one Go package at a
# time (-p=1), so test suites always execute sequentially.
CPU_NICE ?= 10
CPU_SET ?= 0-1
CPU_PROCS ?= 2
.NOTPARALLEL:
# Arguments go in SHELL itself, not .SHELLFLAGS: make 3.81 (macOS's
# /usr/bin/make) ignores .SHELLFLAGS and runs `$(SHELL) -c '<recipe>'`.
TASKSET := $(shell command -v taskset 2>/dev/null)
SHELL := /usr/bin/env nice -n $(CPU_NICE) $(if $(TASKSET),$(TASKSET) -c $(CPU_SET)) bash
export GOMAXPROCS := $(CPU_PROCS)
export GOFLAGS += -p=$(CPU_PROCS)
test%: export GOFLAGS := $(filter-out -p=%,$(GOFLAGS)) -p=1

# ── Build cache ─────────────────────────────────────────────────────────
# The Go build cache lives on disk, never on tmpfs: /tmp is RAM here, there
# is no swap, and a pinned multi-GB cache starves the page cache (the VM's
# memory balloon already takes a large share). It is trimmed back to empty
# whenever it grows past GO_CACHE_MAX_MB; checked once per make invocation.
GO_CACHE_MAX_MB ?= 1500
export GOCACHE := $(or $(GI_GOCACHE),$(HOME)/.cache/go-build)
# Go's per-build work directories (compile/link temporaries, often 0.5-1 GB
# each) default to $TMPDIR, which is RAM here; keep them on disk too.
export GOTMPDIR := $(or $(GI_GOTMPDIR),$(HOME)/.cache/go-tmp)
$(shell mkdir -p "$(GOTMPDIR)")
ifneq ($(filter-out help status logs stop,$(or $(MAKECMDGOALS),help)),)
_GO_CACHE_MB := $(shell du -sm "$(GOCACHE)" 2>/dev/null | cut -f1)
ifneq ($(_GO_CACHE_MB),)
ifeq ($(shell [ $(_GO_CACHE_MB) -gt $(GO_CACHE_MAX_MB) ] && echo over),over)
$(info Go build cache $(_GO_CACHE_MB) MB > $(GO_CACHE_MAX_MB) MB: trimming)
_ := $(shell GOCACHE="$(GOCACHE)" $(or $(GO),go) clean -cache)
endif
endif
endif

# The race detector needs a ThreadSanitizer-compatible address layout; some
# kernels (e.g. 39/42-bit arm64 VMs) lack it. Probe once and drop -race there.
ifndef RACE
RACE := $(shell f=/tmp/gi-race-probe; [ -f $$f.ok ] && cat $$f.ok || { printf 'package main\nfunc main(){}\n' > $$f.go; if timeout 60 $(GO) run -race $$f.go >/dev/null 2>&1; then echo -race; fi | tee $$f.ok; })
endif

# ── Tool commands ───────────────────────────────────────────────────────

GO ?= go
BUN ?= bun
# Web front-end: owned in rcarmo/fixtures-vibes (ui/classic), consumed only through this submodule.
GI_UI := references/fixtures-vibes/ui/classic
PLAYWRIGHT ?= scripts/run-playwright.sh

# ── Runtime defaults ────────────────────────────────────────────────────

PORT ?= 8090
BIND ?= 0.0.0.0
LISTEN ?=
MODEL ?= github-copilot/gpt-5-mini
WORKSPACE ?= /workspace

# ── Local paths ─────────────────────────────────────────────────────────

RUN_DIR ?= .gi-run
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/gi
DB ?= $(RUN_DIR)/gi.db
LOG ?= $(RUN_DIR)/gi.log
PID ?= $(RUN_DIR)/gi.pid

TEST_PORT ?= 19090
TEST_DIR ?= .gi-test
TEST_DB ?= $(TEST_DIR)/gi.db
TEST_LOG ?= $(TEST_DIR)/gi.log
TEST_PID ?= $(TEST_DIR)/gi.pid
TEST_WORKSPACE ?= $(TEST_DIR)/workspace
TEST_RESULTS ?= test-results
TUI_TEST_DIR ?= .gi-tui-test

# ── Derived arguments and data ──────────────────────────────────────────

SERVER_LISTEN_ARGS = -web $(if $(LISTEN),-listen $(LISTEN),-bind $(BIND) -port $(PORT))
SERVER_RUN_ARGS = $(SERVER_LISTEN_ARGS) -model $(MODEL) -db $(DB) -workspace $(WORKSPACE)
SERVER_DAEMON_ARGS = $(SERVER_LISTEN_ARGS) -model $(MODEL) -db $(abspath $(DB)) -workspace $(WORKSPACE) -log-file $(abspath $(LOG)) -pid-file $(abspath $(PID))
SERVER_STATUS_ADDR = $(if $(LISTEN),$(LISTEN),$(BIND):$(PORT))
TEST_SERVER_ARGS = -web -bind 127.0.0.1 -port $(TEST_PORT) -model test-model -db $(abspath $(TEST_DB)) -workspace $(abspath $(TEST_WORKSPACE)) -log-file $(abspath $(TEST_LOG)) -pid-file $(abspath $(TEST_PID))
TEST_PICLAW_CONFIG_JSON = {"assistant":{"assistantName":"Gi Test"},"user":{"userName":"Test User"}}
TEST_ENABLED_MODELS ?= ["test-model"]
TEST_PI_SETTINGS_JSON = {"defaultProvider":"test","defaultModel":"test-model","defaultThinkingLevel":"low","enabledModels":$(TEST_ENABLED_MODELS),"agents":{"list":[{"id":"web","name":"Gi Test","default":true,"model":"test-model"}]}}

# ── Helper macros ───────────────────────────────────────────────────────

define require-command
	@command -v $(1) >/dev/null || { echo "$(2)"; exit 1; }
endef

# ── Mandatory test profiling ────────────────────────────────────────────
TEST_PKGS ?= ./...
TEST_RUN ?=
TEST_PROFILE_DIR ?= $(HOME)/.cache/gi-test-profile
export GI_TEST_PROFILE_DIR := $(TEST_PROFILE_DIR)
export GO
TESTPROFILE := $(abspath $(BIN_DIR)/testprofile)
# Indirection keeps make -n from executing the profiling wrapper as a
# recursive-make recipe and recording a dry run as a passing baseline.
PROFILE_MAKE := $(MAKE)

$(TESTPROFILE): $(wildcard scripts/testprofile/*.go)
	mkdir -p $(BIN_DIR)
	$(GO) build -o $@ ./scripts/testprofile

# Wrap the whole lifecycle, not just the test command. The recursive make
# retains CPU limits and command-line overrides; nested suites execute once.
_PROFILE_GOALS := $(filter-out test-instance-start test-instance-stop test-web-regression-list,$(filter test% fixtures-vibes% bench% profile-tui-complex-tables check,$(MAKECMDGOALS)))
ifneq ($(_PROFILE_GOALS),)
ifeq ($(GI_TEST_PROFILE_ACTIVE),)
# Dispatch all requested goals so mixed invocations (build test) still work.
.PHONY: $(MAKECMDGOALS)
$(MAKECMDGOALS): $(TESTPROFILE)
	$(TESTPROFILE) run -name '$@' -- $(PROFILE_MAKE) --no-print-directory GI_TEST_PROFILE_ACTIVE=1 $@
else
include scripts/test-targets.mk
endif
else
include scripts/test-targets.mk
endif
