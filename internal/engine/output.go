// SPDX-License-Identifier: MIT
package engine

import (
	"fmt"
	"path/filepath"

	"github.com/apikcloud/odoo-builder/internal/config"
	"github.com/apikcloud/odoo-builder/internal/registry"
)

// resolveOutput fills in spec's remaining defaults, consulting cfg.
//
// Convention: when the caller leaves Type entirely unspecified, odoo-builder.yaml's
// image.name selects a registry push (image.tag defaults to "latest");
// otherwise the build falls back to a local OCI-layout tarball under
// buildDir. An explicit Type from the caller always wins over convention, so
// a future API/CI adapter can override per-request without touching
// odoo-builder.yaml — "docker" (used by `builder build --load`) is one such
// explicit-only type, never selected by convention.
func resolveOutput(spec OutputSpec, buildDir string, cfg *config.Config) (OutputSpec, error) {
	if spec.Type == "" {
		if cfg.Image.Name != "" {
			spec.Type = "registry"
		} else {
			spec.Type = "oci"
		}
	}

	switch spec.Type {
	case "oci":
		if spec.Path == "" {
			spec.Path = filepath.Join(buildDir, "image.oci.tar")
		}
		return spec, nil

	case "docker":
		if spec.Image == "" {
			if cfg.Image.Name == "" {
				return OutputSpec{}, fmt.Errorf("engine: output type \"docker\" requires odoo-builder.yaml's image.name (or an explicit BuildRequest.Output.Image) — --load needs a tag to give the loaded image")
			}
			if err := registry.Validate(cfg.Image.Name); err != nil {
				return OutputSpec{}, err
			}
			spec.Image = registry.Reference(cfg.Image.Name, cfg.Image.Tag)
		}
		if spec.Path == "" {
			spec.Path = filepath.Join(buildDir, "image.docker.tar")
		}
		return spec, nil

	case "registry":
		if spec.Image == "" {
			if cfg.Image.Name == "" {
				return OutputSpec{}, fmt.Errorf("engine: output type \"registry\" requires odoo-builder.yaml's image.name (or an explicit BuildRequest.Output.Image)")
			}
			if err := registry.Validate(cfg.Image.Name); err != nil {
				return OutputSpec{}, err
			}
			spec.Image = registry.Reference(cfg.Image.Name, cfg.Image.Tag)
		}
		return spec, nil

	default:
		return OutputSpec{}, fmt.Errorf("engine: unknown output type %q (must be \"oci\", \"docker\", or \"registry\")", spec.Type)
	}
}
