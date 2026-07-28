# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
