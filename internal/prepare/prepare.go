// SPDX-License-Identifier: MIT
package prepare

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/apikcloud/odoo-builder/internal/addons"
	"github.com/apikcloud/odoo-builder/internal/config"
	"github.com/apikcloud/odoo-builder/internal/dockerfile"
	"github.com/apikcloud/odoo-builder/internal/enterprise"
	"github.com/apikcloud/odoo-builder/internal/workspace"
)

// EnterpriseFetchFunc and EnterpriseResolveCommitFunc match
// enterprise.Fetch/ResolveCommit's signatures. Prepare calls them through
// these exported variables so tests can point them at fake implementations
// instead of the real network; production code never reassigns them.
var (
	EnterpriseFetchFunc             = enterprise.Fetch
	EnterpriseResolveCommitFunc     = enterprise.ResolveCommit
	EnterpriseResolveBranchHeadFunc = enterprise.ResolveBranchHead
)

// Prepare builds the deterministic build context for repoRoot at buildDir
// and returns the number of addons discovered and flattened.
func Prepare(repoRoot, buildDir string, cfg *config.Config) (int, error) {
	if _, err := workspace.New(buildDir); err != nil {
		return 0, err
	}

	for _, name := range []string{"requirements.txt", "packages.txt"} {
		src := filepath.Join(repoRoot, name)
		if !fileExists(src) {
			continue
		}
		if err := workspace.CopyFile(src, filepath.Join(buildDir, name)); err != nil {
			return 0, err
		}
	}

	if cfg.Submodules.Init {
		if err := addons.InitSubmodules(repoRoot, cfg.Submodules.Recursive); err != nil {
			return 0, err
		}
	}

	addonsDir := filepath.Join(buildDir, "addons")
	if err := workspace.EnsureDir(addonsDir); err != nil {
		return 0, err
	}

	discovered, errs := addons.Discover(repoRoot, cfg.Addons.Include, cfg.Addons.Exclude, !cfg.Addons.SkipManifestValidation)

	rootAddons, rootErrs := addons.DiscoverShallow(repoRoot, cfg.Addons.Exclude, !cfg.Addons.SkipManifestValidation)
	errs = append(errs, rootErrs...)

	merged, dupErrs := addons.Dedup(append(discovered, rootAddons...))
	discovered = merged
	errs = append(errs, dupErrs...)

	// Enterprise addons are kept in their own list and their own build-context
	// directory: they must never be flattened together with community addons
	// (dockerfile.Generate copies them into Odoo's own core addons directory
	// separately, not /mnt/extra-addons). Dedup is still run across both
	// sources so a name collision is still caught, but its filtered result is
	// discarded — only community addons are removed by a collision here,
	// never enterprise ones.
	var enterpriseAddons []addons.Addon
	if cfg.Enterprise.Enabled {
		token := os.Getenv(enterprise.TokenEnvVar)

		var ref string
		switch {
		case cfg.Enterprise.Commit != "":
			// Explicit override: an exact commit wins over any date/branch
			// resolution, and needs no base.version at all.
			fmt.Fprintf(os.Stderr, "odoo-builder: retrieving Enterprise addons at pinned commit %s\n", cfg.Enterprise.Commit)
			ref = cfg.Enterprise.Commit
		case enterpriseResolveDate(cfg) != "":
			// Reproducible default: pin Enterprise addons to the same day
			// as the community base image (base.release), or an explicit
			// enterprise.date override, instead of the branch's tip.
			date := enterpriseResolveDate(cfg)
			fmt.Fprintf(os.Stderr, "odoo-builder: resolving Enterprise commit on branch %s as of %s\n", cfg.Base.Version, date)
			resolveStart := time.Now()
			sha, resolveErr := EnterpriseResolveCommitFunc(cfg.Base.Version, date, token)
			if resolveErr != nil {
				return 0, resolveErr
			}
			fmt.Fprintf(os.Stderr, "odoo-builder: retrieving Enterprise addons at commit %s (resolved in %s)\n", sha, time.Since(resolveStart).Round(time.Millisecond))
			ref = sha
		default:
			// No date available to pin against: resolve the branch's current
			// HEAD commit (rather than using the bare branch name as ref, as
			// before) so this case's fetch is keyed by an immutable SHA and
			// can be served from EnterpriseFetchFunc's cache exactly like the
			// commit/date-pinned cases above.
			fmt.Fprintf(os.Stderr, "odoo-builder: resolving Enterprise HEAD commit on branch %s\n", cfg.Base.Version)
			resolveStart := time.Now()
			sha, resolveErr := EnterpriseResolveBranchHeadFunc(cfg.Base.Version, token)
			if resolveErr != nil {
				return 0, resolveErr
			}
			fmt.Fprintf(os.Stderr, "odoo-builder: retrieving Enterprise addons at commit %s (resolved in %s)\n", sha, time.Since(resolveStart).Round(time.Millisecond))
			ref = sha
		}

		fetchStart := time.Now()
		entDir, cleanup, fetchErr := EnterpriseFetchFunc(ref, token)
		if fetchErr != nil {
			return 0, fetchErr
		}
		defer cleanup()
		fmt.Fprintf(os.Stderr, "odoo-builder: fetched Enterprise tree in %s\n", time.Since(fetchStart).Round(time.Millisecond))

		discoverStart := time.Now()
		entAddons, entErrs := addons.DiscoverAt(entDir, cfg.Addons.Exclude, !cfg.Addons.SkipManifestValidation)
		errs = append(errs, entErrs...)
		enterpriseAddons = entAddons
		fmt.Fprintf(os.Stderr, "odoo-builder: retrieved %d Enterprise addon(s) (discovered in %s)\n", len(entAddons), time.Since(discoverStart).Round(time.Millisecond))

		combined := append(append([]addons.Addon{}, discovered...), entAddons...)
		_, dupErrs := addons.Dedup(combined)
		errs = append(errs, dupErrs...)
	}

	if len(errs) > 0 {
		return 0, errors.Join(errs...)
	}

	sort.Slice(discovered, func(i, j int) bool { return discovered[i].Name < discovered[j].Name })
	for _, addon := range discovered {
		if err := workspace.CopyDir(addon.RealPath, filepath.Join(addonsDir, addon.Name)); err != nil {
			return 0, err
		}
	}

	if len(enterpriseAddons) > 0 {
		sort.Slice(enterpriseAddons, func(i, j int) bool { return enterpriseAddons[i].Name < enterpriseAddons[j].Name })
		enterpriseAddonsDir := filepath.Join(buildDir, "enterprise-addons")
		if err := workspace.EnsureDir(enterpriseAddonsDir); err != nil {
			return 0, err
		}
		for _, addon := range enterpriseAddons {
			if err := workspace.CopyDir(addon.RealPath, filepath.Join(enterpriseAddonsDir, addon.Name)); err != nil {
				return 0, err
			}
		}
	}

	baseImage, err := dockerfile.ResolveBaseImage(repoRoot, cfg.Base)
	if err != nil {
		return 0, err
	}

	content := dockerfile.Generate(
		baseImage,
		cfg,
		hasNonBlankLine(filepath.Join(buildDir, "requirements.txt")),
		hasNonBlankLine(filepath.Join(buildDir, "packages.txt")),
		len(enterpriseAddons) > 0,
	)

	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte(content), 0o644); err != nil {
		return 0, fmt.Errorf("prepare: writing Dockerfile: %w", err)
	}

	return len(discovered) + len(enterpriseAddons), nil
}

// enterpriseResolveDate returns the date Enterprise addons should be
// pinned to (YYYYMMDD): cfg.Enterprise.Date if set, else cfg.Base.Release,
// else "" when neither is set (no date to pin against).
func enterpriseResolveDate(cfg *config.Config) string {
	if cfg.Enterprise.Date != "" {
		return cfg.Enterprise.Date
	}
	return cfg.Base.Release
}
