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

	"github.com/apikcloud/odoo-builder/internal/engine"
)

// DefaultImage is the distributable odoo-builder image run when the host
// can't run BuildKit directly. Overridable via ImageEnvVar.
const DefaultImage = "ghcr.io/apikcloud/odoo-builder"

// ImageEnvVar, when set, overrides DefaultImage (including its tag).
const ImageEnvVar = "ODOO_BUILDER_IMAGE"

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
// set, otherwise DefaultImage.
func ImageRef() string {
	if v := os.Getenv(ImageEnvVar); v != "" {
		return v
	}
	return DefaultImage
}
