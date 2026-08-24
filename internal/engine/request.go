// SPDX-License-Identifier: MIT
// Package engine hosts the trigger-independent BuildRequest model and the
// Engine that executes it. Every entry point (CLI today; a REST API,
// Kubernetes Job, or CI integration later) builds the same BuildRequest and
// calls the same Engine methods — no Odoo-specific business logic lives in
// internal/cli or any future adapter.
package engine

import "path/filepath"

// APIVersion identifies the BuildRequest/BuildResponse wire schema. Bump on
// any incompatible field change; the engine rejects requests with an
// unknown/newer major version it can't safely interpret.
const APIVersion = "v1"

// Command selects which engine operation a BuildRequest triggers.
type Command string

const (
	CommandValidate Command = "validate"
	CommandPrepare  Command = "prepare"
	CommandBuild    Command = "build"
	CommandInspect  Command = "inspect"
)

// BuildRequest is the serializable description of everything required to
// execute a build. It is JSON/YAML compatible so a future entry point can
// persist it, transmit it to a remote build service, or embed it in a
// Kubernetes Job spec.
type BuildRequest struct {
	APIVersion string  `json:"apiVersion" yaml:"apiVersion"`
	Command    Command `json:"command" yaml:"command"`

	// RepoRoot is the local filesystem path to the already-checked-out
	// repository. Cloning or mounting the repository is the entry point's
	// responsibility, not the engine's.
	RepoRoot string `json:"repoRoot" yaml:"repoRoot"`

	// BuildDir is the deterministic build context directory. Defaults to
	// "<RepoRoot>/.build" when empty (see Normalize).
	BuildDir string `json:"buildDir,omitempty" yaml:"buildDir,omitempty"`

	// Output describes the produced image artifact's destination.
	Output OutputSpec `json:"output,omitempty" yaml:"output,omitempty"`

	// Load, if true, is only valid with Command == CommandBuild; forces
	// Output.Type to "docker" (mirrors today's `build --load`).
	Load bool `json:"load,omitempty" yaml:"load,omitempty"`

	// Resolved is filled in by the engine (never read from an incoming
	// request) and echoed back in BuildResponse.Resolved for `inspect` to
	// print. A caller-supplied value is ignored.
	Resolved *ResolvedSpec `json:"resolved,omitempty" yaml:"resolved,omitempty"`
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
	// Insecure, when true and Type == "registry", allows pushing to a
	// registry with no TLS or an unverifiable certificate (e.g. an
	// in-cluster registry:2 with no cert). Ignored for other Types.
	Insecure bool `json:"insecure,omitempty" yaml:"insecure,omitempty"`
}

// ResolvedSpec is the engine's fully-resolved view of a request, derived
// from odoo-builder.yaml (internal/config.Config) plus BuildRequest — the
// same information Engine.Inspect returns today, restructured for the
// wire contract.
type ResolvedSpec struct {
	Base       BaseImageSpec  `json:"base"`
	Image      ImageSpec      `json:"image"`
	Builder    BuilderSpec    `json:"builder"`
	Enterprise EnterpriseSpec `json:"enterprise"`
	Addons     AddonsSpec     `json:"addons"`
}

type BaseImageSpec struct {
	Version string `json:"version,omitempty"`
	Release string `json:"release,omitempty"`
	Image   string `json:"image"` // resolved "odoo:<version>[-<release>]"
}

type ImageSpec struct {
	Name   string            `json:"name,omitempty"`
	Tag    string            `json:"tag,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
}

type BuilderSpec struct {
	Platforms    []string `json:"platforms,omitempty"`
	CacheEnabled bool     `json:"cacheEnabled"`
}

type EnterpriseSpec struct {
	Enabled bool   `json:"enabled"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
}

type AddonsSpec struct {
	Include                []string `json:"include,omitempty"`
	Exclude                []string `json:"exclude,omitempty"`
	SkipManifestValidation bool     `json:"skipManifestValidation"`
}

// BuildResponse is what the engine returns for any Command.
type BuildResponse struct {
	APIVersion string `json:"apiVersion"`

	// Resolved is set for CommandInspect (and, incidentally, available
	// after any other command since the engine always resolves config
	// first) — the same data as BuildRequest.Resolved.
	Resolved *ResolvedSpec `json:"resolved,omitempty"`

	// ValidationErrors is set for CommandValidate (empty slice, not nil,
	// on success — the CLI distinguishes "ran and passed" from "didn't run").
	ValidationErrors []string `json:"validationErrors,omitempty"`

	// PrepareResult mirrors engine.PrepareResult, set for
	// CommandPrepare/CommandBuild.
	BuildDir   string `json:"buildDir,omitempty"`
	AddonCount int    `json:"addonCount,omitempty"`

	// BuildResult fields, set for CommandBuild only.
	ImagePath string `json:"imagePath,omitempty"`
	ImageRef  string `json:"imageRef,omitempty"`

	// Error carries any failure as a string (errors don't survive JSON
	// round-trips otherwise) — nil/omitted means success.
	Error string `json:"error,omitempty"`

	// ErrorCode is a machine-readable classification of Error, set
	// whenever Error is — CI/K8s callers branch on this instead of
	// string-matching Error's text (which is for humans and may be
	// reworded). See ErrorCode's const block for the full enum.
	ErrorCode ErrorCode `json:"errorCode,omitempty"`
}

// ErrorCode classifies a BuildResponse's failure. The zero value
// ("") means no error.
type ErrorCode string

const (
	// ErrorCodeUnsupportedAPIVersion: req.APIVersion didn't match APIVersion.
	ErrorCodeUnsupportedAPIVersion ErrorCode = "unsupported_api_version"
	// ErrorCodeUnknownCommand: req.Command wasn't one of the four Command values.
	ErrorCodeUnknownCommand ErrorCode = "unknown_command"
	// ErrorCodeValidationFailed: CommandValidate found one or more problems
	// (also set — alongside ValidationErrors — when CommandBuild's internal
	// pre-build Validate call fails).
	ErrorCodeValidationFailed ErrorCode = "validation_failed"
	// ErrorCodePrepareFailed: CommandPrepare/CommandBuild's Prepare step failed
	// (config load, addon discovery/dedup, Enterprise resolution, etc).
	ErrorCodePrepareFailed ErrorCode = "prepare_failed"
	// ErrorCodeRootlessRequired: CommandBuild's Runner.Build failed with
	// buildkit.ErrRootlessRequired — detected via errors.Is inside Execute,
	// before the error is stringified, so this never depends on message-text
	// matching. internal/launcher's Invoke uses this (not string-sniffing)
	// to decide on the auto-mode container retry.
	ErrorCodeRootlessRequired ErrorCode = "rootless_required"
	// ErrorCodeBuildFailed: CommandBuild failed for any other reason.
	ErrorCodeBuildFailed ErrorCode = "build_failed"
)

// Normalize returns a copy of req with defaults filled in. It never
// mutates req. Output defaults are resolved separately by Engine.Build
// (see output.go), since they require odoo-builder.yaml's cfg.Image.
func (req BuildRequest) Normalize() BuildRequest {
	if req.BuildDir == "" {
		req.BuildDir = filepath.Join(req.RepoRoot, ".build")
	}
	return req
}
