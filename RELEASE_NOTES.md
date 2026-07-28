# Release Notes

This page summarizes what's new and fixed in each version.

## [0.2.0] - 2026-07-28

Small housekeeping release: the CLI has a new name and an easier install path.

### ✨ What's new

- New `install.sh` script — install the CLI with one command.

### ⚠️ Points d'attention

- Breaking change: the binary is now called `odoo-builder` instead of `builder`. Update any scripts, aliases, or CI configs that invoke the old name.

## [0.1.0] - 2026-07-28

First release of Odoo Image Builder. 🎉

### ✨ What's new

- Turn any Odoo addons repository into a ready-to-run container image — no Dockerfile needed. Works with Docker, Podman, Kubernetes Jobs, and CI.
- Automatically finds your addons, follows symlinks and git submodules, and builds a clean image with BuildKit.
- Configuration file renamed to `odoo-builder.yaml` for clarity.

### 🐛 Fixes

- Enterprise addons are now pinned to a specific commit or release date, so builds stay reproducible.
