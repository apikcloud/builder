// SPDX-License-Identifier: MIT
// Package engine hosts the trigger-independent BuildRequest model and the
// Engine that executes it. Every entry point (CLI today; a REST API,
// Kubernetes Job, or CI integration later) builds the same BuildRequest and
// calls the same Engine methods — no Odoo-specific business logic lives in
// internal/cli or any future adapter.
package engine

import "path/filepath"

// BuildRequest is the serializable description of everything required to
// execute a build. It is JSON/YAML compatible so a future entry point can
// persist it, transmit it to a remote build service, or embed it in a
// Kubernetes Job spec.
type BuildRequest struct {
	// RepoRoot is the local filesystem path to the already-checked-out
	// repository. Cloning or mounting the repository is the entry point's
	// responsibility, not the engine's.
	RepoRoot string `json:"repoRoot" yaml:"repoRoot"`

	// BuildDir is the deterministic build context directory. Defaults to
	// "<RepoRoot>/.build" when empty (see Normalize).
	BuildDir string `json:"buildDir,omitempty" yaml:"buildDir,omitempty"`

	// Output describes the produced image artifact's destination.
	Output OutputSpec `json:"output,omitempty" yaml:"output,omitempty"`
}

// OutputSpec describes a build's output artifact.
type OutputSpec struct {
	// Type selects "oci" (a local OCI-layout tarball) or "registry" (a
	// push). Left empty by the caller, it is resolved by Engine.Build from
	// odoo-builder.yaml's image.name (see resolveOutput in output.go) — this
	// requires I/O Normalize cannot perform, so it does not happen here.
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	// Path is the local filesystem destination. Used when Type == "oci".
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	// Image is the full "name:tag" reference to push to. Used when
	// Type == "registry"; if empty, resolved from odoo-builder.yaml's
	// image.name/image.tag.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`
}

// Normalize returns a copy of req with defaults filled in. It never
// mutates req. Output defaults are resolved separately by Engine.Build
// (see output.go), since they require odoo-builder.yaml's cfg.Image.
func (req BuildRequest) Normalize() BuildRequest {
	if req.BuildDir == "" {
		req.BuildDir = filepath.Join(req.RepoRoot, ".build")
	}
	return req
}
