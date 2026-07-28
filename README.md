# Odoo Image Builder

Reproducible OCI image builder for Odoo deployments. Give it a repository of
addons, get back a container image — no Dockerfile to write, works the same
on Docker, Podman, Kubernetes Jobs, and CI.

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

```
repo/
├── module_a/                # custom addon
├── module_b/
├── some_module -> .third-party/OCA/some-repo/some_module   # symlink
├── .third-party/
│    └── OCA/some-repo/      # git submodule
├── requirements.txt         # optional — pip install -r requirements.txt
├── packages.txt             # optional — one Debian package per line
├── odoo_version.txt         # optional — base image ref, if odoo-builder.yaml's base.version is unset
├── odoo-builder.yaml        # optional — advanced configuration
└── .gitmodules               # optional
```

A plain `addons/` (and `addons-extra/`) directory of modules is also
supported via `odoo-builder.yaml`'s `addons.include` — the builder discovers
addons both at the repository root and under any included directory.

No Dockerfile expected or wanted.

## CLI

```
builder build      # build (and push/load) the image
builder prepare    # produce the deterministic .build/ context only
builder validate   # check repo layout, addons, packages.txt, odoo-builder.yaml
builder inspect     # print the resolved BuildRequest without building
builder version
```

### Engine Mode vs Launcher Mode

`builder build` runs BuildKit either directly on the host (**Engine Mode**,
if `buildctl`/`buildkitd` are on `PATH`) or inside the distributable builder
image via Docker/Podman (**Launcher Mode**, using the exact pinned BuildKit
version). Default is automatic (`--mode auto`); override with `--mode engine`
or `--mode launcher`, or the `ODOO_BUILDER_MODE` env var.

### `--load`: build straight into your local image store

```
builder build --load
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

## Design principles

* Convention over configuration.
* Reproducible, deterministic builds — running `builder prepare` twice
  produces identical output.
* No Dockerfile required; BuildKit is an implementation detail.
* The builder image is the product; the local CLI is only a thin launcher.
