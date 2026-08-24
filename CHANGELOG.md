# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `BUILDKIT_TLS_CERT`/`BUILDKIT_TLS_KEY`/`BUILDKIT_TLS_CACERT` env vars let `odoo-builder-engine` present a client cert to an externally managed `buildkitd` that enforces mTLS, threaded through to `buildctl`'s `--tlscert`/`--tlskey`/`--tlscacert` flags

## [0.7.0] - 2026-08-24

### Added

- `BuildRequest.Output.Insecure` lets a caller push to a registry with no TLS or an unverifiable certificate (e.g. an in-cluster `registry:2` with no cert), passed through to BuildKit's `registry.insecure=true` image-exporter option

### Changed

- `migrate.sh` and the `upgrade/` directory now stay at the addons root in the built image, instead of being moved or dropped

## [0.6.0] - 2026-08-01

### Added

- Enterprise addon fetches are now cached on disk keyed by commit SHA, avoiding redundant re-fetches of an already-retrieved commit across builds
- Build progress output expanded into a staged summary, giving finer-grained visibility into each step as it completes

## [0.5.0] - 2026-07-28

### Added

- `build` now prints a one-line request summary (repo root, build dir, load flag, output type/image) to stderr right before it starts, and `prepare` logs progress while retrieving Enterprise addons (pinned commit, date-resolved commit, or branch tip) plus the resulting addon count

## [0.4.3] - 2026-07-28

### Fixed

- Container-mode builds now default to the exact image tag matching the launcher binary's own release version instead of `latest` — `docker`/`podman run` never re-pull an already-cached tag, so a host that had pulled an older `latest` once was silently running a stale image forever, even after a newer one was pushed. `ODOO_BUILDER_IMAGE` still overrides this.

### Documentation

- Note Engine Mode's performance advantage over Launcher Mode in the README, and when `--mode auto` already picks it for you

## [0.4.2] - 2026-07-28

### Performance

- Containerized builds now route `buildkitd`'s own `--root` through a bind-mounted host directory instead of the container's own overlay filesystem, letting BuildKit use its `overlayfs` snapshotter instead of falling back to the much slower `native` one — sharply reducing layer extraction/commit time for large base images

## [0.4.1] - 2026-07-28

### Fixed

- Allow blank lines and `#` comments in `packages.txt` — previously any line containing a space (including comments) was rejected as an invalid package entry; a comment-only or empty file (after stripping comments) no longer adds an unnecessary `apt-get install` step to the generated Dockerfile

## [0.4.0] - 2026-07-28

### Added

- `cache.type` option in `odoo-builder.yaml`'s `cache` section (`local` or `registry`) to force local on-disk caching even when `image.name` is set, instead of always preferring registry cache

### Fixed

- Default the container-mode fallback image to the actually-published Docker Hub image (`apik/odoo-builder`), not an unpublished GHCR reference
- Ctrl+C now actually cancels an in-progress build (local and container mode) instead of leaving it running detached
- Include a snippet of the engine's raw stdout when its response fails to decode, instead of a bare JSON parse error

### Performance

- Skip the registry manifest check for the base Odoo image on every build (`--opt image-resolve-mode=local`), since the resolved base image reference is always immutable by design

## [0.3.0] - 2026-07-28

### Added

- Split `odoo-builder` into two binaries: `odoo-builder` (thin host-side launcher) and `odoo-builder-engine` (does the real work — validate, prepare, build, inspect). The launcher sends a JSON `BuildRequest` to the engine and prints the `BuildResponse`; `odoo-builder-engine` can also be invoked directly, reading from `ODOO_BUILDER_REQUEST_FILE`/stdin and writing to `ODOO_BUILDER_RESPONSE_FILE`/stdout — the integration path for Kubernetes Jobs and CI running the container image directly
- Add quick start section to README

### Changed

- `install.sh` now downloads and installs both `odoo-builder` and `odoo-builder-engine`
- `odoo-builder inspect` now prints the resolved configuration instead of the raw `BuildRequest`

### Fixed

- Correct repo name in `install.sh` (`apikcloud/builder`, not `odoo-builder`)
- Don't retry `--load` builds into the container on rootless failure

## [0.2.0] - 2026-07-28

### Added

- `install.sh` script for easy installation

### Changed

- **Breaking:** binary renamed from `builder` to `odoo-builder` (Makefile targets, CLI `Use` string, and dist archive names updated accordingly)

## [0.1.0] - 2026-07-28

### Added

- Initial release: reproducible OCI image builder for Odoo deployments — discovers addons, resolves symlinks and git submodules, generates the Dockerfile, builds with BuildKit, and pushes/loads the resulting image
- Rename config file to `odoo-builder.yaml`, fix README repository layout documentation

### Fixed

- Pin enterprise addons to a commit or release date
