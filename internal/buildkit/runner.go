// SPDX-License-Identifier: MIT
// Package buildkit launches or reaches a BuildKit daemon and executes
// builds against a prepared build context. It is the only package that
// shells out to buildctl/buildkitd — callers depend only on the Runner
// interface, keeping their logic unit-testable without either binary
// installed.
package buildkit

import "context"

// BuildOptions describes one BuildKit invocation.
type BuildOptions struct {
	// ContextDir is the prepared build directory (contains Dockerfile,
	// addons/, and optionally requirements.txt/packages.txt).
	ContextDir string

	// DockerfilePath is the path to the Dockerfile used for this build.
	// Its directory is passed to buildctl's "dockerfile=" local mount.
	DockerfilePath string

	// OutputType selects the buildctl exporter: "oci" (a local OCI-layout
	// tarball at OutputPath), "docker" (a local docker-archive tarball at
	// OutputPath, tagged Image — loadable via "docker load"/"podman load",
	// used by `builder build --load`), or "registry" (a push to Image).
	OutputType string

	// OutputPath is the local filesystem destination for the exported
	// artifact. Used when OutputType is "oci" or "docker".
	OutputPath string

	// Image is the full "name:tag" reference to push to or tag the loaded
	// archive with. Used when OutputType is "registry" or "docker".
	Image string

	// Insecure, when true, allows pushing without TLS verification. Only
	// applies when OutputType == "registry".
	Insecure bool

	// CacheDir enables BuildKit's local on-disk cache (type=local) at this
	// directory, used when cache is enabled and no registry image is
	// configured. Mutually exclusive with CacheRef — callers should set at
	// most one; if both are set, CacheRef takes precedence.
	CacheDir string

	// CacheRef enables BuildKit's registry-based cache (type=registry) at
	// this "name:tag" reference, used when cache is enabled and a registry
	// image is configured. Mutually exclusive with CacheDir.
	CacheRef string

	// Platforms lists the target platforms (e.g. "linux/amd64",
	// "linux/arm64") passed to buildctl as --opt platform=.... A nil or
	// empty slice omits the flag entirely, letting BuildKit fall back to
	// its own default (the host's platform).
	Platforms []string
}

// BuildOutput reports what a build produced.
type BuildOutput struct {
	// OutputPath is set when OutputType was "oci" or "docker".
	OutputPath string
	// ImageRef is set when OutputType was "registry" or "docker".
	ImageRef string
}

// Runner executes a BuildOptions request and returns its result.
type Runner interface {
	Build(ctx context.Context, opts BuildOptions) (BuildOutput, error)
}
