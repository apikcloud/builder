package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/apikcloud/odoo-builder/internal/buildkit"
	"github.com/apikcloud/odoo-builder/internal/config"
	"github.com/apikcloud/odoo-builder/internal/prepare"
	"github.com/apikcloud/odoo-builder/internal/registry"
)

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

	return req, nil
}

// Build runs the full pipeline — Validate, then Prepare, then invokes
// BuildKit to produce the image artifact described by req.Output — mirroring
// README's architecture diagram (README.md:24-44).
func (e *Engine) Build(ctx context.Context, req BuildRequest) (BuildResult, error) {
	req = req.Normalize()

	if errs := e.Validate(req); len(errs) > 0 {
		return BuildResult{}, fmt.Errorf("engine: validation failed: %w", errors.Join(errs...))
	}

	cfg, err := loadConfig(req)
	if err != nil {
		return BuildResult{}, err
	}

	prepRes, err := e.prepareWithConfig(req, cfg)
	if err != nil {
		return BuildResult{}, err
	}

	output, err := resolveOutput(req.Output, req.BuildDir, cfg)
	if err != nil {
		return BuildResult{}, err
	}

	var cacheDir, cacheRef string
	if cfg.Cache.Enabled {
		if cfg.Image.Name != "" {
			cacheRef = registry.Reference(cfg.Image.Name, "buildcache")
		} else {
			var cacheErr error
			cacheDir, cacheErr = buildkit.DefaultCacheDir()
			if cacheErr != nil {
				return BuildResult{}, cacheErr
			}
		}
	}

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
