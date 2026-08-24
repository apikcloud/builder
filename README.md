# Odoo Image Builder

Reproducible OCI image builder for Odoo deployments. Give it a repository of
addons, get back a container image — no Dockerfile to write, works the same
on Docker, Podman, Kubernetes Jobs, and CI.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/apikcloud/builder/main/install.sh | sh
```

Downloads the latest release for your platform (linux/amd64, linux/arm64),
verifies its checksum, and installs both `odoo-builder` and
`odoo-builder-engine` to `/usr/local/bin` (falls back to `~/.local/bin`). Set
`VERSION=vX.Y.Z` to pin a release.

## Quick start

```bash
cd your-addons-repo
odoo-builder build
```

Zero-config: builds a local OCI-layout tarball at `.build/image.oci.tar`.

To push to a registry (the common case), set `image.name` in
`odoo-builder.yaml`:

```yaml
image:
  name: registry.example.com/customer/odoo
  tag: production
```

```bash
odoo-builder build
```

Same command — setting `image.name` is what makes it push. See
[odoo-builder.yaml](#odoo-builderyaml) below for the full config (base
version, Enterprise, `--load` into a local Docker/Podman instead of
pushing, etc.).

## What it does

```
Repository → validate → prepare workspace → generate Dockerfile → BuildKit → OCI image → registry
```

* discovers Odoo addons recursively (anything with `__manifest__.py` or
  `__openerp__.py`)
* resolves symlinks and git submodules, flattens everything into a clean
  build context
* generates the Dockerfile for you, based on an official Odoo base image
* builds with BuildKit and pushes to any OCI-compatible registry (or loads
  straight into your local Docker/Podman)

## Repository layout

Custom addons live at the repository root. Third-party addons (OCA, vendors,
...) are git submodules under `.third-party/<owner>/<repo>`, exposed via a
symlink at the repository root pointing into the submodule — never copied in.

```bash
repo/
├── module_a/                # custom addon
├── module_b/
├── some_module -> .third-party/OCA/some-repo/some_module   # symlink
├── .third-party/
│    └── OCA/some-repo/      # git submodule
├── migrate.sh               # optional — Odoo commands to run between updates; copied to /mnt/extra-addons/migrate.sh in the image
├── upgrade/                 # optional — Python scripts for major upgrades only (not an addon); copied to /mnt/extra-addons/upgrade/ in the image
├── requirements.txt         # optional — pip install -r requirements.txt
├── packages.txt             # optional — one Debian package per line, blank lines/#comments allowed
├── odoo_version.txt         # optional — base image ref, if odoo-builder.yaml's base.version is unset
├── odoo-builder.yaml        # optional — advanced configuration
└── .gitmodules               # optional
```

A plain `addons/` (and `addons-extra/`) directory of modules is also
supported via `odoo-builder.yaml`'s `addons.include` — the builder discovers
addons both at the repository root and under any included directory.

No Dockerfile expected or wanted.



## CLI

```bash
odoo-builder build      # build (and push/load) the image
odoo-builder prepare    # produce the deterministic .build/ context only
odoo-builder validate   # check repo layout, addons, packages.txt, odoo-builder.yaml
odoo-builder inspect    # print the resolved configuration without building
odoo-builder version
```

### Two binaries: launcher and engine

`odoo-builder` ships as two binaries: `odoo-builder` (a thin, host-side
launcher) and `odoo-builder-engine` (does the real work — validate, prepare,
build, inspect). Every command the launcher runs builds a versioned
`BuildRequest`, sends it as JSON to the engine, and prints the `BuildResponse`
it gets back. The engine runs either directly on the host, as a local
subprocess (**Engine Mode**, when `odoo-builder-engine` and — for `build` —
`buildctl`/`buildkitd` are on `PATH`), or inside the distributable container
image via Docker/Podman (**Launcher Mode**, piping the same JSON request/
response over the container's stdin/stdout, using the exact pinned BuildKit
version). Default is automatic (`--mode auto`); override with `--mode engine`
or `--mode launcher`, or the `ODOO_BUILDER_MODE` env var.

`odoo-builder-engine` can also be invoked directly, without the launcher —
reading a `BuildRequest` from `ODOO_BUILDER_REQUEST_FILE` (or stdin) and
writing its `BuildResponse` to `ODOO_BUILDER_RESPONSE_FILE` (or stdout). This
is the integration path for Kubernetes Jobs and CI systems that run the
container image directly rather than shelling out to the launcher.

### Performance: install `buildctl`/`buildkitd` on the host if you can

Engine Mode (running directly on the host, no container) is significantly
faster than Launcher Mode: BuildKit's `--root` sits directly on the host's
own filesystem, so its `overlayfs` snapshotter is used natively. Launcher
Mode still routes `buildkitd`'s `--root` through a bind-mounted host
directory for the same `overlayfs` benefit, but a containerized build
still carries `docker`/`podman run` startup overhead, image pull/export
steps, and one more layer of process indirection on top.

If your host has `buildctl`/`buildkitd` installed (see [BuildKit's
releases](https://github.com/moby/buildkit/releases)) and can run them
directly (no `RootlessKit` required), `--mode auto` (the default) already
prefers Engine Mode automatically — no configuration needed. Only hosts
without `buildctl`/`buildkitd`, or where they refuse to start unprivileged
(`buildkitd`'s own rootless requirement), fall back to Launcher Mode.

### `--load`: build straight into your local image store

```bash
odoo-builder build --load
```

Builds and loads the image into the local Docker/Podman image store
(`docker load`/`podman load`) instead of pushing — tagged from
`odoo-builder.yaml`'s `image.name`/`image.tag`. Requires Engine Mode and
`odoo-builder.yaml`'s `image.name` to be set. If the load step fails, the
built tarball is kept on disk and the error names the exact command to
retry.

## odoo-builder.yaml

Everything is optional — the builder works with zero configuration.

```yaml
base:
  version: "18.0"
  release: "20250611"

enterprise:
  enabled: true
  # commit: "abc123..."  # optional — pin to an exact commit, overrides date/branch resolution
  # date: "20250611"     # optional — pin to this day instead of base.release

addons:
  include:
    - addons
    - addons-extra
  exclude:
    - test_module

submodules:
  init: true
  recursive: true

build:
  platform:
    - linux/amd64
    - linux/arm64

cache:
  enabled: true
  # type: local     # optional — "local" (on-disk dir) or "registry"
  #                 # (<image.name>:buildcache). Defaults to "registry"
  #                 # when image.name is set, "local" otherwise; set
  #                 # explicitly to keep caching locally while still
  #                 # pushing the built image to a registry.

image:
  name: registry.example.com/customer/odoo
  tag: production

labels:
  org.opencontainers.image.vendor: Example

environment:
  MY_ENV: value
```

## Enterprise support

When `enterprise.enabled` is set, the builder fetches the Odoo Enterprise
addons repository host-side, authenticating via the `ODOO_ENTERPRISE_TOKEN`
environment variable — the token never appears in `odoo-builder.yaml`, a
command line, or a log line. Which commit it fetches is resolved in this
order:

1. **`enterprise.commit`** — an exact commit SHA, used as-is. No
   `base.version` needed.
2. **A date** — `enterprise.date` if set, else `base.release`: the newest
   commit on the `base.version` branch at or before that day, so Enterprise
   addons are pinned to the same day as the community base image and the
   build stays reproducible.
3. **Neither set** — the `base.version` branch's tip at build time (not
   reproducible across builds on different days).

## Local development

```bash
make test-local
```

Builds this checkout's `odoo-builder`/`odoo-builder-engine` (`make build`) and
runs them against `testdata/simple` with `PATH` pinned to this checkout's
`bin/` — so it always exercises the binaries you just built, not whatever
`odoo-builder-engine` happens to already be on `$PATH` (e.g. an older install
under `~/.local/bin`).

Default `TEST_MODE=auto` mirrors the CLI's own `--mode auto`: tries Engine
Mode first, and — only if `buildkitd` can't run directly on the host (e.g. no
root/RootlessKit) — retries automatically in Launcher Mode. That automatic
fallback uses `ODOO_BUILDER_IMAGE` if set, else whatever `:latest` was last
pulled/built locally, which may be stale. Run `eval "$(make image-dev)"`
first (tags `apik/odoo-builder:dev`; plain `make image` tags a version string
`ImageRef` won't resolve to) so the fallback — or an explicit
`TEST_MODE=launcher` — runs your current code instead:

```bash
eval "$(make image-dev)"
make test-local TEST_MODE=launcher
```

Override the target repo:

```bash
make test-local TESTDIR=path/to/repo
```

If `TESTDIR/odoo-builder.yaml` is untracked (not committed — check with `git
status --short`), `test-local` warns before running: an untracked config
enabling `enterprise.enabled`, for instance, silently triggers a real network
fetch against the Enterprise repository on every local test run.

## Design principles

* Convention over configuration.
* Reproducible, deterministic builds — running `odoo-builder prepare` twice
  produces identical output.
* No Dockerfile required; BuildKit is an implementation detail.
* The builder image is the product; `odoo-builder` is only a thin,
  host-side launcher — all real work happens in `odoo-builder-engine`,
  whether that binary runs locally or inside the container image.

## License

MIT — see [LICENSE](LICENSE).
