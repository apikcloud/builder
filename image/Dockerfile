# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
WORKDIR /src
# github.com/apikcloud/workspace-provider is a private repo — GOPRIVATE
# skips the public module proxy/checksum DB for it, and the insteadOf
# rewrite makes `go mod download` fetch it over SSH (forwarded via
# --mount=type=ssh below, see the `image`/`image-dev` Makefile targets'
# `docker build --ssh default`) instead of unauthenticated HTTPS.
ENV GOPRIVATE=github.com/apikcloud/workspace-provider
RUN mkdir -p -m 0700 /root/.ssh && ssh-keyscan -t rsa,ed25519 github.com >> /root/.ssh/known_hosts \
    && git config --global url."git@github.com:".insteadOf "https://github.com/"
COPY go.mod go.sum ./
RUN --mount=type=ssh go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
      -ldflags "-X github.com/apikcloud/builder/internal/version.Version=${VERSION} -X github.com/apikcloud/builder/internal/version.Commit=${COMMIT} -X github.com/apikcloud/builder/internal/version.Date=${DATE}" \
      -o /out/odoo-builder-engine ./cmd/odoo-builder-engine
# odoo-builder (the launcher) is intentionally NOT built into this image —
# it never runs inside the container (see
# thoughts/shared/plans/2026-07-28-two-binary-launcher-engine-split.md).

FROM debian:bookworm-slim AS buildkit
ARG BUILDKIT_VERSION=v0.20.2
ARG TARGETARCH
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
# The upstream release tarball's root contains a bin/ directory, so
# extracting to /usr/local places buildctl/buildkitd at /usr/local/bin/.
RUN curl -fsSL "https://github.com/moby/buildkit/releases/download/${BUILDKIT_VERSION}/buildkit-${BUILDKIT_VERSION}.linux-${TARGETARCH}.tar.gz" \
      -o /tmp/buildkit.tar.gz \
    && tar -xzf /tmp/buildkit.tar.gz -C /usr/local \
    && rm /tmp/buildkit.tar.gz

FROM debian:bookworm-slim
# runc is not in README's Image Contents list, but buildkitd's OCI worker
# (image/buildkitd.toml's [worker.oci]) execs it directly and has no
# fallback — without it buildkitd fails at startup with "failed to find
# runc binary", confirmed via this plan's own manual verification.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       git openssh-client ca-certificates tar gzip runc \
    && rm -rf /var/lib/apt/lists/*
COPY --from=buildkit /usr/local/bin/buildctl /usr/local/bin/buildctl
COPY --from=buildkit /usr/local/bin/buildkitd /usr/local/bin/buildkitd
COPY --from=build /out/odoo-builder-engine /usr/local/bin/odoo-builder-engine
COPY image/buildkitd.toml /etc/buildkit/buildkitd.toml
COPY image/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
