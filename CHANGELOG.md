# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-07-28

### Added

- Initial release: reproducible OCI image builder for Odoo deployments — discovers addons, resolves symlinks and git submodules, generates the Dockerfile, builds with BuildKit, and pushes/loads the resulting image
- Rename config file to `odoo-builder.yaml`, fix README repository layout documentation

### Fixed

- Pin enterprise addons to a commit or release date
