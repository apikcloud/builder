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
)

// DefaultImage is the distributable odoo-builder image run when the host
// can't run BuildKit directly. Overridable via ImageEnvVar.
const DefaultImage = "ghcr.io/apikcloud/odoo-builder"

// ImageEnvVar, when set, overrides DefaultImage (including its tag).
const ImageEnvVar = "ODOO_BUILDER_IMAGE"

// lookPath is exec.LookPath, indirected so tests can simulate
// buildctl/buildkitd being present or absent without needing the real
// binaries on PATH.
var lookPath = exec.LookPath

// Needed reports whether odoo-builder build must go straight to running the
// distributable image because the required binaries aren't even present:
// true unless both buildctl and buildkitd are found on PATH. This does
// NOT detect "present but can't run" (e.g. buildkitd needing RootlessKit)
// — that case is caught separately, by internal/buildkit's
// ErrRootlessRequired, after a direct attempt actually fails.
func Needed() bool {
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
