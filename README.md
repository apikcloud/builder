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

```
repo/
├── addons/
│    ├── module_a/
│    └── module_b/
├── addons-extra/
├── requirements.txt        # optional — pip install -r requirements.txt
├── packages.txt            # optional — one Debian package per line
├── builder.yaml            # optional — advanced configuration
└── .gitmodules              # optional
```

No Dockerfile expected or wanted.

## CLI

```
builder build      # build (and push/load) the image
builder prepare    # produce the deterministic .build/ context only
builder validate   # check repo layout, addons, packages.txt, builder.yaml
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
`builder.yaml`'s `image.name`/`image.tag`. Requires Engine Mode and
`builder.yaml`'s `image.name` to be set. If the load step fails, the built
tarball is kept on disk and the error names the exact command to retry.

## builder.yaml

Everything is optional — the builder works with zero configuration.

```yaml
base:
  version: "18.0"
  release: "20250611"

enterprise:
  enabled: true

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

When `enterprise.enabled` is set, the builder clones Odoo Enterprise using
BuildKit secrets for authentication — credentials are never copied into
layers or logged.

## Design principles

* Convention over configuration.
* Reproducible, deterministic builds — running `builder prepare` twice
  produces identical output.
* No Dockerfile required; BuildKit is an implementation detail.
* The builder image is the product; the local CLI is only a thin launcher.
