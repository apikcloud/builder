// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apikcloud/builder/internal/buildkit"
	"github.com/apikcloud/builder/internal/config"
	"github.com/apikcloud/builder/internal/dockerfile"
	"github.com/apikcloud/builder/internal/prepare"
	"github.com/apikcloud/builder/internal/registry"
)

// errValidationFailed is wrapped into Build's pre-build validation error so
// Execute can distinguish "failed during pre-build validation" from "failed
// during the actual BuildKit invocation" via errors.Is, without
// string-matching the error's text.
var errValidationFailed = errors.New("engine: validation failed")

// Engine executes BuildRequests. It has no knowledge of how a request was
// triggered.
type Engine struct {
	Runner buildkit.Runner
}

// New returns an Engine backed by the real buildctl/buildkitd Runner.
func New() *Engine {
	return &Engine{Runner: buildkit.NewRunner()}
}

// PrepareResult reports what Prepare produced.
type PrepareResult struct {
	BuildDir   string
	AddonCount int
}

// BuildResult reports what Build produced.
type BuildResult struct {
	PrepareResult
	ImagePath string // set when Output.Type was "oci"
	ImageRef  string // set when Output.Type was "registry"
}

// Validate checks req.RepoRoot's layout, config, and addons, returning
// every problem found rather than stopping at the first.
func (e *Engine) Validate(req BuildRequest) []error {
	req = req.Normalize()
	return prepare.Validate(req.RepoRoot)
}

// Prepare produces the deterministic build context described by req.
func (e *Engine) Prepare(req BuildRequest) (PrepareResult, error) {
	req = req.Normalize()

	cfg, err := loadConfig(req)
	if err != nil {
		return PrepareResult{}, err
	}
	return e.prepareWithConfig(req, cfg)
}

func (e *Engine) prepareWithConfig(req BuildRequest, cfg *config.Config) (PrepareResult, error) {
	count, err := prepare.Prepare(req.RepoRoot, req.BuildDir, cfg)
	if err != nil {
		return PrepareResult{}, err
	}
	return PrepareResult{BuildDir: req.BuildDir, AddonCount: count}, nil
}

func loadConfig(req BuildRequest) (*config.Config, error) {
	return config.Load(filepath.Join(req.RepoRoot, "odoo-builder.yaml"))
}

// Inspect returns req's fully-resolved form — normalized paths and, if
// Output.Type was left empty, the same odoo-builder.yaml-driven resolution
// Engine.Build applies (internal/engine/output.go's resolveOutput) —
// without discovering addons, preparing the build context, or invoking
// BuildKit. Used by `builder inspect` to show what a subsequent `builder
// build` would do.
func (e *Engine) Inspect(req BuildRequest) (BuildRequest, error) {
	req = req.Normalize()

	cfg, err := loadConfig(req)
	if err != nil {
		return BuildRequest{}, err
	}

	output, err := resolveOutput(req.Output, req.BuildDir, cfg)
	if err != nil {
		return BuildRequest{}, err
	}
	req.Output = output

	resolved, err := resolveSpec(req.RepoRoot, cfg)
	if err != nil {
		return BuildRequest{}, err
	}
	req.Resolved = resolved

	return req, nil
}

// resolveSpec translates cfg (odoo-builder.yaml, already loaded) into the
// wire-contract ResolvedSpec returned in BuildResponse and echoed on
// BuildRequest.Resolved.
func resolveSpec(repoRoot string, cfg *config.Config) (*ResolvedSpec, error) {
	baseImage, err := dockerfile.ResolveBaseImage(repoRoot, cfg.Base)
	if err != nil {
		return nil, err
	}

	return &ResolvedSpec{
		Base: BaseImageSpec{
			Version: cfg.Base.Version,
			Release: cfg.Base.Release,
			Image:   baseImage,
		},
		Image: ImageSpec{
			Name:   cfg.Image.Name,
			Tag:    cfg.Image.Tag,
			Labels: cfg.Labels,
			Env:    cfg.Environment,
		},
		Builder: BuilderSpec{
			Platforms:    cfg.Build.Platform,
			CacheEnabled: cfg.Cache.Enabled,
		},
		Enterprise: EnterpriseSpec{
			Enabled: cfg.Enterprise.Enabled,
			Commit:  cfg.Enterprise.Commit,
			Date:    cfg.Enterprise.Date,
		},
		Addons: AddonsSpec{
			Include:                cfg.Addons.Include,
			Exclude:                cfg.Addons.Exclude,
			SkipManifestValidation: cfg.Addons.SkipManifestValidation,
		},
	}, nil
}

// Build runs the full pipeline — Validate, then Prepare, then invokes
// BuildKit to produce the image artifact described by req.Output — mirroring
// README's architecture diagram (README.md:24-44).
func (e *Engine) Build(ctx context.Context, req BuildRequest) (BuildResult, error) {
	req = req.Normalize()

	fmt.Fprintln(os.Stderr, "==> [1/3] validating")
	if errs := e.Validate(req); len(errs) > 0 {
		return BuildResult{}, fmt.Errorf("%w: %w", errValidationFailed, errors.Join(errs...))
	}

	cfg, err := loadConfig(req)
	if err != nil {
		return BuildResult{}, err
	}

	resolved, err := resolveSpec(req.RepoRoot, cfg)
	if err != nil {
		return BuildResult{}, err
	}

	output, err := resolveOutput(req.Output, req.BuildDir, cfg)
	if err != nil {
		return BuildResult{}, err
	}

	printBuildSummary(req, cfg, resolved, output)

	fmt.Fprintln(os.Stderr, "==> [2/3] preparing build context")
	prepRes, err := e.prepareWithConfig(req, cfg)
	if err != nil {
		return BuildResult{}, err
	}
	fmt.Fprintf(os.Stderr, "odoo-builder: prepared %d addon(s) at %s\n", prepRes.AddonCount, prepRes.BuildDir)

	var cacheDir, cacheRef string
	if cfg.Cache.Enabled {
		switch cfg.Cache.Type {
		case "local":
			var cacheErr error
			cacheDir, cacheErr = buildkit.DefaultCacheDir()
			if cacheErr != nil {
				return BuildResult{}, cacheErr
			}
		case "registry":
			if cfg.Image.Name == "" {
				return BuildResult{}, fmt.Errorf("engine: cache.type \"registry\" requires image.name to be set")
			}
			cacheRef = registry.Reference(cfg.Image.Name, "buildcache")
		case "":
			if cfg.Image.Name != "" {
				cacheRef = registry.Reference(cfg.Image.Name, "buildcache")
			} else {
				var cacheErr error
				cacheDir, cacheErr = buildkit.DefaultCacheDir()
				if cacheErr != nil {
					return BuildResult{}, cacheErr
				}
			}
		default:
			return BuildResult{}, fmt.Errorf("engine: unknown cache.type %q (must be \"local\" or \"registry\")", cfg.Cache.Type)
		}
	}

	fmt.Fprintln(os.Stderr, "==> [3/3] building image")
	out, err := e.Runner.Build(ctx, buildkit.BuildOptions{
		ContextDir:     prepRes.BuildDir,
		DockerfilePath: filepath.Join(prepRes.BuildDir, "Dockerfile"),
		OutputType:     output.Type,
		OutputPath:     output.Path,
		Image:          output.Image,
		CacheDir:       cacheDir,
		CacheRef:       cacheRef,
		Platforms:      cfg.Build.Platform,
	})
	if err != nil {
		return BuildResult{}, err
	}

	return BuildResult{PrepareResult: prepRes, ImagePath: out.OutputPath, ImageRef: out.ImageRef}, nil
}

// printBuildSummary writes a multi-line summary of the resolved build to
// stderr, once config/output resolution has succeeded but before Prepare
// runs — addon counts aren't included here since Prepare is what actually
// discovers them (see the "prepared N addon(s)" line Build logs right
// after prepareWithConfig returns).
func printBuildSummary(req BuildRequest, cfg *config.Config, resolved *ResolvedSpec, output OutputSpec) {
	w := os.Stderr
	fmt.Fprintln(w, "odoo-builder: build summary")
	fmt.Fprintf(w, "  repo:        %s\n", req.RepoRoot)
	fmt.Fprintf(w, "  build dir:   %s\n", req.BuildDir)
	fmt.Fprintf(w, "  base image:  %s\n", baseImageSummary(resolved.Base))
	fmt.Fprintf(w, "  enterprise:  %s\n", enterpriseSummary(resolved))
	fmt.Fprintf(w, "  cache:       %s\n", cacheSummary(cfg, resolved.Builder.CacheEnabled))
	fmt.Fprintf(w, "  output:      %s\n", outputSummary(output))
	if req.Load {
		fmt.Fprintln(w, "  load:        true")
	}
}

// baseImageSummary formats spec.Image alongside the base.version/release
// that produced it, when those were explicitly set (as opposed to
// resolved from odoo_version.txt, see dockerfile.ResolveBaseImage).
func baseImageSummary(spec BaseImageSpec) string {
	if spec.Version == "" {
		return spec.Image
	}
	if spec.Release == "" {
		return fmt.Sprintf("%s (version=%s)", spec.Image, spec.Version)
	}
	return fmt.Sprintf("%s (version=%s, release=%s)", spec.Image, spec.Version, spec.Release)
}

// enterpriseSummary reports whether Enterprise addons are enabled and, if
// so, what they're pinned to — mirroring prepare.enterpriseResolveDate's
// fallback order (an explicit commit wins, then enterprise.date, then
// base.release) so this never drifts from what prepare.Prepare actually
// does when it retrieves them.
func enterpriseSummary(resolved *ResolvedSpec) string {
	if !resolved.Enterprise.Enabled {
		return "disabled"
	}
	switch {
	case resolved.Enterprise.Commit != "":
		return fmt.Sprintf("enabled (commit=%s)", resolved.Enterprise.Commit)
	case resolved.Enterprise.Date != "":
		return fmt.Sprintf("enabled (pinned to %s)", resolved.Enterprise.Date)
	case resolved.Base.Release != "":
		return fmt.Sprintf("enabled (pinned to %s, via base.release)", resolved.Base.Release)
	default:
		return "enabled (branch tip)"
	}
}

// cacheSummary reports whether BuildKit caching is enabled and, if so, its
// configured type (cfg.Cache.Type — resolved.Builder doesn't carry it, only
// whether caching is enabled at all).
func cacheSummary(cfg *config.Config, enabled bool) string {
	if !enabled {
		return "disabled"
	}
	if cfg.Cache.Type != "" {
		return fmt.Sprintf("enabled (type=%s)", cfg.Cache.Type)
	}
	return "enabled"
}

// outputSummary formats output (already fully resolved by resolveOutput)
// for display.
func outputSummary(output OutputSpec) string {
	switch output.Type {
	case "oci", "docker":
		return fmt.Sprintf("%s -> %s", output.Type, output.Path)
	case "registry":
		return fmt.Sprintf("registry -> %s", output.Image)
	default:
		return output.Type
	}
}
