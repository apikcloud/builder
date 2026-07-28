MODULE  := github.com/apikcloud/odoo-builder
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION)

IMAGE     ?= apik/odoo-builder
IMAGE_TAG ?= $(VERSION)
PLATFORMS ?= linux/amd64,linux/arm64
DIST_TARGETS ?= linux/amd64 linux/arm64

COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_ARGS := --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE)

.PHONY: build test lint fmt vet clean image dist dist-archive image-verify image-push

build:
	go build -ldflags "$(LDFLAGS)" -o bin/odoo-builder ./cmd/builder

dist:
	@mkdir -p bin/dist
	@for pair in $(DIST_TARGETS); do \
		os=$${pair%/*}; arch=$${pair#*/}; \
		out=bin/dist/odoo-builder-$$os-$$arch; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $$out ./cmd/builder; \
	done

dist-archive: dist
	@cd bin/dist && for f in odoo-builder-*; do tar -czf $$f.tar.gz $$f; done
	@cd bin/dist && sha256sum *.tar.gz > checksums.txt

image:
	docker build \
	  $(BUILD_ARGS) \
	  -t $(IMAGE):$(IMAGE_TAG) \
	  -f image/Dockerfile .

image-verify:
	docker buildx build \
	  --platform $(PLATFORMS) \
	  $(BUILD_ARGS) \
	  -f image/Dockerfile .

image-push:
	docker buildx build \
	  --platform $(PLATFORMS) \
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
