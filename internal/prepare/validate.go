// SPDX-License-Identifier: MIT
// Package prepare orchestrates repository validation and build-context
// preparation.
package prepare

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apikcloud/builder/internal/addons"
	"github.com/apikcloud/builder/internal/buildkit"
	"github.com/apikcloud/builder/internal/config"
	"github.com/apikcloud/builder/internal/dockerfile"
	"github.com/apikcloud/builder/internal/enterprise"
	"github.com/apikcloud/builder/internal/registry"
)

// Validate checks repoRoot's layout, odoo-builder.yaml syntax (if present),
// packages.txt syntax (if present), and addon discovery problems
// (duplicates, broken symlinks, invalid manifests). It aggregates every
// problem found rather than stopping at the first one.
func Validate(repoRoot string) []error {
	var errs []error

	cfg, cfgErr := config.Load(filepath.Join(repoRoot, "odoo-builder.yaml"))
	if cfgErr != nil {
		errs = append(errs, cfgErr)
		cfg = config.Default()
	}

	if err := validateAddonsPresence(repoRoot, cfg); err != nil {
		errs = append(errs, err)
	}

	if cfg.Image.Name != "" {
		if err := registry.Validate(cfg.Image.Name); err != nil {
			errs = append(errs, err)
		}
	}

	if cfg.Enterprise.Enabled {
		if cfg.Base.Version == "" && cfg.Enterprise.Commit == "" {
			errs = append(errs, fmt.Errorf("validate: enterprise.enabled requires base.version to be set (to select the matching Enterprise branch), unless enterprise.commit pins an exact commit"))
		}
		if os.Getenv(enterprise.TokenEnvVar) == "" {
			errs = append(errs, fmt.Errorf("validate: enterprise.enabled requires %s to be set", enterprise.TokenEnvVar))
		}
	}

	for _, p := range cfg.Build.Platform {
		if err := buildkit.ValidatePlatform(p); err != nil {
			errs = append(errs, fmt.Errorf("validate: build.platform: %w", err))
		}
	}

	if err := validatePackagesTxt(repoRoot); err != nil {
		errs = append(errs, err)
	}

	discovered, discoverErrs := addons.Discover(repoRoot, cfg.Addons.Include, cfg.Addons.Exclude, !cfg.Addons.SkipManifestValidation)
	errs = append(errs, discoverErrs...)

	rootAddons, rootErrs := addons.DiscoverShallow(repoRoot, cfg.Addons.Exclude, !cfg.Addons.SkipManifestValidation)
	errs = append(errs, rootErrs...)

	_, dupErrs := addons.Dedup(append(discovered, rootAddons...))
	errs = append(errs, dupErrs...)

	if _, err := dockerfile.ResolveBaseImage(repoRoot, cfg.Base); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func validateAddonsPresence(repoRoot string, cfg *config.Config) error {
	hasAddons := dirExists(filepath.Join(repoRoot, "addons"))
	hasAddonsExtra := dirExists(filepath.Join(repoRoot, "addons-extra"))
	if hasAddons || hasAddonsExtra {
		return nil
	}

	rootAddons, _ := addons.DiscoverShallow(repoRoot, cfg.Addons.Exclude, !cfg.Addons.SkipManifestValidation)
	if len(rootAddons) > 0 {
		return nil
	}

	return fmt.Errorf("validate: no addons/ or addons-extra/ directory found in %s, and no addon directories found directly in its root", repoRoot)
}

func validatePackagesTxt(repoRoot string) error {
	path := filepath.Join(repoRoot, "packages.txt")
	if !fileExists(path) {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("validate: reading %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.ContainsAny(line, " \t") {
			return fmt.Errorf("validate: %s:%d: invalid package entry %q (expected one package name per line)", path, lineNo, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("validate: reading %s: %w", path, err)
	}

	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// hasNonBlankLine reports whether path exists and contains at least one
// line that is neither blank nor a "#" comment (both requirements.txt's
// pip syntax and packages.txt's convention, see validatePackagesTxt, treat
// "#"-prefixed lines as comments) — so a file containing only blank lines
// and/or comments is treated the same as an empty or absent one, and
// Generate skips the COPY/RUN step for it entirely.
func hasNonBlankLine(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			return true
		}
	}
	return false
}
