# Release Notes

This page summarizes what's new and fixed in each version.

## [0.8.0] - 2026-08-24

Support for connecting to a remote build server that requires a client certificate.

### ✨ What's new

- If your organization's build server (`buildkitd`) requires client certificates for secure connections, you can now point `odoo-builder` at it by setting three new environment variables (`BUILDKIT_TLS_CERT`, `BUILDKIT_TLS_KEY`, `BUILDKIT_TLS_CACERT`) with the paths to your certificate, key, and CA files.

## [0.7.0] - 2026-08-24

A new option for pushing to registries without TLS, plus a fix to keep upgrade scripts in place inside the built image.

### ✨ What's new

- You can now push images to a registry that doesn't use TLS or has an unverifiable certificate (e.g. an in-cluster registry with no cert) — useful for internal/dev setups.

### 🔄 Improvements

- `migrate.sh` and the `upgrade/` folder now stay at the addons root of the built image, so upgrade tooling that expects them there keeps working.

## [0.6.0] - 2026-08-01

Faster repeat builds for Enterprise users, plus clearer progress reporting.

### ✨ What's new

- Enterprise addons are now cached on disk after first fetch, so rebuilding with the same commit is faster — no more re-downloading what you already have.
- Build progress now shows a staged, step-by-step summary, making it easier to see exactly what's happening during a build.

## [0.5.0] - 2026-07-28

A small visibility improvement so builds no longer look stuck.

### ✨ What's new

- You'll now see a short summary of what's about to build, plus live progress while Enterprise addons are being fetched — no more silent waiting during that step.

## [0.4.3] - 2026-07-28

A follow-up fix to last version's build speedup, plus a documentation note.

### 🐛 Fixes

- Builds run via the container image now always use the exact image version matching your `odoo-builder` install, instead of possibly reusing an older cached image under the same tag. This makes sure everyone actually gets last version's speedup, and any future fix, right away.

### 📝 Documentation

- The README now explains when running directly on your machine (if you have `buildctl`/`buildkitd` installed) is faster than the container image, and that `odoo-builder` already picks the faster option for you automatically when it can.

## [0.4.2] - 2026-07-28

A speed fix under the hood for anyone building inside the container image.

### 🔄 Improvements

- Builds run inside the odoo-builder container image now extract layers much faster — especially noticeable with large base images (like Odoo Enterprise). No changes needed on your end.

## [0.4.1] - 2026-07-28

Small fix for anyone documenting their `packages.txt`.

### 🐛 Fixes

- `packages.txt` now accepts blank lines and `#` comments, so you can document why a package is there without it being rejected as invalid.

## [0.4.0] - 2026-07-28

Faster, more reliable builds: better caching control, and Ctrl+C actually works now.

### ✨ What's new

- New `cache.type` option lets you cache builds on disk even when pushing images to a registry, instead of always going through the registry for cache — faster repeat builds.

### 🔄 Improvements

- Builds skip a redundant network check for the base Odoo image, shaving a bit of time off every run.

### 🐛 Fixes

- Pressing Ctrl+C during a build now actually stops it, instead of leaving it running in the background.
- Fixed a fallback path that pointed at a container image that was never actually published.
- Clearer error messages when something goes wrong decoding a build's response.

## [0.3.0] - 2026-07-28

Under-the-hood rework: the CLI now runs as two cooperating binaries, plus a couple of small fixes.

### ✨ What's new

- `odoo-builder` now ships alongside a second binary, `odoo-builder-engine`, which does the actual build work. This makes it easier to run builds directly inside Kubernetes Jobs or CI without going through the launcher. The install script installs both automatically, and everyday usage of `odoo-builder build` / `prepare` / `validate` / `inspect` is unchanged.
- Quick start section added to the README.

### 🐛 Fixes

- Fixed a rare issue where `--load` builds could get stuck retrying inside the container on rootless setups.
- Fixed the repository name referenced by `install.sh`.

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
