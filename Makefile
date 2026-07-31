MODULE  := github.com/apikcloud/odoo-builder
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION)

IMAGE     ?= apik/odoo-builder
IMAGE_TAG ?= $(VERSION)
PLATFORMS ?= linux/amd64,linux/arm64
DIST_TARGETS ?= linux/amd64 linux/arm64

TESTDIR   ?= testdata/simple
TEST_MODE ?= auto

COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_ARGS := --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE)
# --ssh default forwards the invoking user's SSH agent into the build, so
# the Dockerfile's `go mod download` can fetch the private
# github.com/apikcloud/workspace-provider module over SSH (see its
# GOPRIVATE/insteadOf setup). Requires SSH_AUTH_SOCK to be set (a running
# ssh-agent with the right key added).
SSH_ARGS := --ssh default

.PHONY: build test lint fmt vet clean image image-dev dist dist-archive image-verify image-push test-local

build:
	go build -ldflags "$(LDFLAGS)" -o bin/odoo-builder ./cmd/odoo-builder
	go build -ldflags "$(LDFLAGS)" -o bin/odoo-builder-engine ./cmd/odoo-builder-engine

dist:
	@mkdir -p bin/dist
	@for pair in $(DIST_TARGETS); do \
		os=$${pair%/*}; arch=$${pair#*/}; \
		for bin in odoo-builder odoo-builder-engine; do \
			out=bin/dist/$$bin-$$os-$$arch; \
			echo "building $$out"; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $$out ./cmd/$$bin; \
		done; \
	done

dist-archive: dist
	@cd bin/dist && for os_arch in $$(for pair in $(DIST_TARGETS); do echo $${pair%/*}-$${pair#*/}; done); do \
		tar -czf odoo-builder-$$os_arch.tar.gz odoo-builder-$$os_arch odoo-builder-engine-$$os_arch; \
	done
	@cd bin/dist && sha256sum *.tar.gz > checksums.txt

image:
	docker build \
	  $(SSH_ARGS) \
	  $(BUILD_ARGS) \
	  -t $(IMAGE):$(IMAGE_TAG) \
	  -f image/Dockerfile .

# image-dev builds the distributable image from local sources and tags it
# ":dev" — a tag internal/launcher.ImageRef never resolves to on its own
# (only an exact release tag or "latest"), so Launcher Mode would otherwise
# keep running whatever ":latest" was last pulled instead of this build.
# Make can't export into the invoking shell's environment, so this only
# prints the export line (build log goes to stderr); run it as:
#   eval "$(make image-dev)"
image-dev:
	@docker build $(SSH_ARGS) $(BUILD_ARGS) -t $(IMAGE):dev -f image/Dockerfile . >&2
	@echo "export ODOO_BUILDER_IMAGE=$(IMAGE):dev"

image-verify:
	docker buildx build \
	  --platform $(PLATFORMS) \
	  $(SSH_ARGS) \
	  $(BUILD_ARGS) \
	  -f image/Dockerfile .

image-push:
	docker buildx build \
	  --platform $(PLATFORMS) \
	  $(SSH_ARGS) \
	  $(BUILD_ARGS) \
	  -t $(IMAGE):$(IMAGE_TAG) \
	  -t $(IMAGE):latest \
	  --push \
	  -f image/Dockerfile .

test:
	go test ./... -v

vet:
	go vet ./...

lint: vet
	@fmt_out=$$(gofmt -l .); \
	if [ -n "$$fmt_out" ]; then \
		echo "gofmt needed on:"; echo "$$fmt_out"; exit 1; \
	fi

fmt:
	gofmt -w .

clean:
	rm -rf bin .build

# test-local runs this checkout's freshly built binaries against TESTDIR
# (default testdata/simple) — not whatever odoo-builder-engine/odoo-builder
# happen to already be on $PATH (e.g. an older install under ~/.local/bin):
# PATH is pinned to this checkout's bin/ below, so that risk is closed
# regardless of TEST_MODE. TEST_MODE=auto (default, mirrors the CLI's own
# --mode auto) tries Engine Mode first and, only on a buildkitd-rootless
# failure, retries automatically in Launcher Mode — which uses
# ODOO_BUILDER_IMAGE if set, else falls back to whatever :latest was last
# pulled/built locally, which may be stale. Run `eval "$(make image-dev)"`
# first if you want that automatic fallback (or an explicit
# TEST_MODE=launcher below) to run your current code instead.
# Override with `make test-local TESTDIR=path/to/repo TEST_MODE=launcher`.
test-local: build
	@if [ -f $(TESTDIR)/odoo-builder.yaml ] && ! git ls-files --error-unmatch $(TESTDIR)/odoo-builder.yaml >/dev/null 2>&1; then \
		echo "warning: $(TESTDIR)/odoo-builder.yaml is untracked (not committed) -- remove it unless you intend to hit a real network fetch (e.g. enterprise.enabled)"; \
	fi
	cd $(TESTDIR) && PATH="$(CURDIR)/bin:$$PATH" $(CURDIR)/bin/odoo-builder build --mode $(TEST_MODE)
