// SPDX-License-Identifier: MIT
// Package launcher runs the distributable odoo-builder container image
// via Docker or Podman — either because the host has no buildctl/
// buildkitd installed, or because internal/buildkit detected they can't
// actually run there (see buildkit.ErrRootlessRequired) — and also loads a
// build's local docker-archive output into the host's Docker/Podman image
// store (odoo-builder build --load, see Load). It is the only package that
// shells out to docker/podman.
package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/apikcloud/odoo-builder/internal/engine"
	"github.com/apikcloud/odoo-builder/internal/version"
)

// DefaultImageRepo is the distributable odoo-builder image's repository
// (no tag) run when the host can't run BuildKit directly. See ImageRef
// for how its tag is chosen. Overridable wholesale via ImageEnvVar (which
// specifies the full reference, tag included).
const DefaultImageRepo = "docker.io/apik/odoo-builder"

// ImageEnvVar, when set, overrides ImageRef's entire return value
// (including the tag).
const ImageEnvVar = "ODOO_BUILDER_IMAGE"

// releaseVersionPattern matches version.Version's exact shape at a tagged
// release build — e.g. "v0.4.2" (see Makefile's VERSION, built from `git
// describe --tags --always --dirty` against the tagged commit itself in
// .github/workflows/release.yml, which pushes the image under that same
// tag). A plain `go build` ("dev") or a dirty/ahead-of-tag checkout
// ("vX.Y.Z-N-gHASH[-dirty]") has no matching image tag in the registry,
// so ImageRef falls back to "latest" for those instead.
var releaseVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// EngineBinary is the name of the engine binary the launcher execs
// locally, resolved via $PATH (installed alongside odoo-builder by
// install.sh/dist-archive).
const EngineBinary = "odoo-builder-engine"

// lookPath is exec.LookPath, indirected so tests can simulate
// buildctl/buildkitd/the engine binary being present or absent without
// needing the real binaries on PATH.
var lookPath = exec.LookPath

// engineAvailable reports whether EngineBinary is on $PATH.
func engineAvailable() bool {
	_, err := lookPath(EngineBinary)
	return err == nil
}

// Needed reports whether a request must run containerized: true if the
// engine binary itself isn't on PATH, or — for cmd == engine.CommandBuild
// specifically — buildctl/buildkitd aren't on PATH either. Other commands
// (validate/prepare/inspect) don't require BuildKit tools, only the engine
// binary. This does NOT detect "buildctl/buildkitd present but can't run"
// (e.g. buildkitd needing RootlessKit) — that case is caught separately, by
// internal/buildkit's ErrRootlessRequired, after a direct attempt actually
// fails.
func Needed(cmd engine.Command) bool {
	if !engineAvailable() {
		return true
	}
	if cmd != engine.CommandBuild {
		return false
	}
	_, buildctlErr := lookPath("buildctl")
	_, buildkitdErr := lookPath("buildkitd")
	return buildctlErr != nil || buildkitdErr != nil
}

// Runtime is a container runtime the launcher can shell out to.
type Runtime string

const (
	Docker Runtime = "docker"
	Podman Runtime = "podman"
)

// DetectRuntime returns the first of docker, podman found on PATH, in
// that order. Returns an error naming both if neither is present.
func DetectRuntime() (Runtime, error) {
	if _, err := lookPath("docker"); err == nil {
		return Docker, nil
	}
	if _, err := lookPath("podman"); err == nil {
		return Podman, nil
	}
	return "", fmt.Errorf("launcher: neither docker nor podman found on PATH (required to run %s)", ImageRef())
}

// ImageRef returns the image reference to run: ImageEnvVar's value if
// set, otherwise DefaultImageRepo tagged with the running binary's own
// version — so the launcher always drives the exact image release built
// alongside it, rather than a possibly-stale "latest" a host happened to
// pull once before (docker/podman run don't re-pull an existing tag on
// their own). Falls back to ":latest" when the binary's own version isn't
// itself a tagged release (local/dev builds), since no matching image tag
// exists in the registry for those.
func ImageRef() string {
	if v := os.Getenv(ImageEnvVar); v != "" {
		return v
	}
	tag := "latest"
	if releaseVersionPattern.MatchString(version.Version) {
		tag = version.Version
	}
	return DefaultImageRepo + ":" + tag
}
