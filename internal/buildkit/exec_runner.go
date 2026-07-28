package buildkit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// execRunner shells out to buildctl, self-launching a buildkitd on a
// temporary unix socket when BUILDKIT_HOST is not already set.
type execRunner struct{}

// NewRunner returns the real, subprocess-based Runner.
func NewRunner() Runner { return execRunner{} }

func (execRunner) Build(ctx context.Context, opts BuildOptions) (BuildOutput, error) {
	addr, cleanup, err := ensureDaemon(ctx)
	if err != nil {
		return BuildOutput{}, err
	}
	defer cleanup()

	var outputArg string
	switch opts.OutputType {
	case "oci":
		if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
			return BuildOutput{}, fmt.Errorf("buildkit: creating output directory: %w", err)
		}
		outputArg = "type=oci,dest=" + opts.OutputPath
	case "docker":
		if opts.Image == "" {
			return BuildOutput{}, fmt.Errorf("buildkit: output type \"docker\" requires Image")
		}
		if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
			return BuildOutput{}, fmt.Errorf("buildkit: creating output directory: %w", err)
		}
		// name= embeds the reference into the archive's manifest.json
		// (RepoTags), exactly as "docker save" does, so "docker load"/
		// "podman load" apply the tag automatically — no separate
		// "docker tag" step needed after loading.
		outputArg = "type=docker,name=" + opts.Image + ",dest=" + opts.OutputPath
	case "registry":
		if opts.Image == "" {
			return BuildOutput{}, fmt.Errorf("buildkit: output type \"registry\" requires Image")
		}
		outputArg = "type=image,name=" + opts.Image + ",push=true"
	default:
		return BuildOutput{}, fmt.Errorf("buildkit: unsupported output type %q (must be \"oci\", \"docker\", or \"registry\")", opts.OutputType)
	}

	if opts.CacheRef == "" && opts.CacheDir != "" {
		if err := os.MkdirAll(opts.CacheDir, 0o755); err != nil {
			return BuildOutput{}, fmt.Errorf("buildkit: creating cache directory: %w", err)
		}
	}

	args := []string{
		"--addr", addr,
		"build",
		"--frontend", "dockerfile.v0",
		"--local", "context=" + opts.ContextDir,
		"--local", "dockerfile=" + filepath.Dir(opts.DockerfilePath),
		"--output", outputArg,
	}
	args = append(args, platformArgs(opts.Platforms)...)
	args = append(args, cacheArgs(opts.CacheRef, opts.CacheDir)...)

	cmd := exec.CommandContext(ctx, "buildctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return BuildOutput{}, fmt.Errorf("buildkit: buildctl build: %w", err)
	}

	if opts.OutputType == "registry" {
		return BuildOutput{ImageRef: opts.Image}, nil
	}
	if opts.OutputType == "docker" {
		return BuildOutput{OutputPath: opts.OutputPath, ImageRef: opts.Image}, nil
	}
	return BuildOutput{OutputPath: opts.OutputPath}, nil
}
