package addons

import (
	"fmt"
	"os"
	"path/filepath"
)

// Addon describes a single discovered Odoo module.
type Addon struct {
	Name       string // directory basename, used as the flattened module name
	SourcePath string // as encountered under the include root
	RealPath   string // symlink-resolved (== SourcePath if not a symlink)
	IsSymlink  bool
}

// manifestFilenames are checked, in order, at each candidate addon root.
var manifestFilenames = []string{"__manifest__.py", "__openerp__.py"}

// Discover walks each include path under repoRoot looking for directories
// containing __manifest__.py or __openerp__.py. See DiscoverAt for the
// per-root walk and Dedup for duplicate handling, both used here.
// validateManifests controls whether each manifest's Python-literal
// content is parsed and validated (see DiscoverAt).
//
// errs is returned even when addons is non-empty, so callers must check
// len(errs) > 0 explicitly rather than relying on a nil slice.
func Discover(repoRoot string, include, exclude []string, validateManifests bool) (addons []Addon, errs []error) {
	var found []Addon
	for _, inc := range include {
		root := filepath.Join(repoRoot, inc)
		rootAddons, rootErrs := DiscoverAt(root, exclude, validateManifests)
		found = append(found, rootAddons...)
		errs = append(errs, rootErrs...)
	}

	found, dupErrs := Dedup(found)
	errs = append(errs, dupErrs...)
	return found, errs
}

// DiscoverAt walks root directly (root is used exactly as given — no join
// against any base path) looking for addon manifests. A directory is
// registered as an addon and NOT descended into further once a manifest is
// found at its own level (so nested category folders like addons/oca/web/...
// are still walked, but a module's own internals are not re-scanned). It
// performs no duplicate detection; callers combining multiple roots (e.g.
// community addons plus an Enterprise clone) call Dedup themselves once
// every root has been gathered. A missing or non-directory root is silently
// skipped, matching Discover's existing per-include-path tolerance.
//
// When validateManifests is false, a manifest file's presence is still
// required to register a directory as an addon, but its content is never
// parsed — an escape hatch for real-world manifests using syntax
// ParseManifest doesn't (yet) support.
func DiscoverAt(root string, exclude []string, validateManifests bool) (addons []Addon, errs []error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excludeSet[name] = true
	}

	return walkAddonRoot(root, excludeSet, validateManifests)
}

// Dedup removes every addon whose Name collides with another entry in
// addons, reporting one error per colliding pair. This is detectDuplicates,
// exported so callers can merge addons gathered from more than one
// DiscoverAt call before deduplicating.
func Dedup(addons []Addon) (deduped []Addon, errs []error) {
	return detectDuplicates(addons)
}

// DiscoverShallow looks for addon manifests among root's direct children
// only: unlike DiscoverAt, it never descends into a child directory that
// lacks a manifest of its own. Such a directory is left untouched, whether
// it is a container of addons reachable another way (e.g. an `addons/`
// folder also named in Discover's include list, or the real directory a
// root-level symlink points into) or simply unrelated to Odoo addons
// (`.git`, a vendored dependency directory, ...). This makes it safe to
// call unconditionally against a repository's root to pick up addons
// (plain directories or symlinks) placed directly there, without
// re-discovering — and falsely flagging as duplicates — addons already
// found through a dedicated include path. It performs no duplicate
// detection; see Dedup.
func DiscoverShallow(root string, exclude []string, validateManifests bool) (addons []Addon, errs []error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excludeSet[name] = true
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, []error{fmt.Errorf("addons: reading %s: %w", root, err)}
	}

	var found []Addon
	for _, entry := range entries {
		kind, addon, err := resolveEntry(root, entry, excludeSet, validateManifests)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if kind == entryAddon {
			found = append(found, addon)
		}
	}

	return found, errs
}

func walkAddonRoot(dir string, exclude map[string]bool, validateManifests bool) ([]Addon, []error) {
	var found []Addon
	var errs []error

	entries, err := os.ReadDir(dir)
	if err != nil {
		errs = append(errs, fmt.Errorf("addons: reading %s: %w", dir, err))
		return found, errs
	}

	for _, entry := range entries {
		kind, addon, err := resolveEntry(dir, entry, exclude, validateManifests)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		switch kind {
		case entryContainer:
			subFound, subErrs := walkAddonRoot(filepath.Join(dir, entry.Name()), exclude, validateManifests)
			found = append(found, subFound...)
			errs = append(errs, subErrs...)
		case entryAddon:
			found = append(found, addon)
		}
	}

	return found, errs
}

// entryKind classifies a single directory entry considered by
// walkAddonRoot/DiscoverShallow.
type entryKind int

const (
	entrySkip      entryKind = iota // not a directory, or an excluded addon name
	entryContainer                  // a directory with no manifest of its own
	entryAddon                      // a directory registered as addon
)

// resolveEntry classifies entry (a child of dir) and, when it is a valid,
// non-excluded addon, builds its Addon value. err is non-nil only for
// problems that must abort discovery of this entry (stat failures, broken
// symlinks, invalid manifests) — an excluded or non-directory entry is
// reported as entrySkip with a nil error. When validateManifests is false,
// the manifest's content is never parsed (only its presence is checked).
func resolveEntry(dir string, entry os.DirEntry, exclude map[string]bool, validateManifests bool) (kind entryKind, addon Addon, err error) {
	entryPath := filepath.Join(dir, entry.Name())

	lst, err := os.Lstat(entryPath)
	if err != nil {
		return entrySkip, Addon{}, fmt.Errorf("addons: stat %s: %w", entryPath, err)
	}

	isSymlink := lst.Mode()&os.ModeSymlink != 0
	realPath := entryPath
	if isSymlink {
		resolved, err := filepath.EvalSymlinks(entryPath)
		if err != nil {
			return entrySkip, Addon{}, fmt.Errorf("addons: broken symlink: %s", entryPath)
		}
		realPath = resolved
	}

	var isDir bool
	if isSymlink {
		info, err := os.Stat(realPath)
		if err != nil {
			return entrySkip, Addon{}, fmt.Errorf("addons: stat %s: %w", realPath, err)
		}
		isDir = info.IsDir()
	} else {
		isDir = entry.IsDir()
	}
	if !isDir {
		return entrySkip, Addon{}, nil
	}

	name := entry.Name()

	manifestName, ok := findManifest(entryPath)
	if !ok {
		return entryContainer, Addon{}, nil
	}

	if exclude[name] {
		return entrySkip, Addon{}, nil
	}

	if validateManifests {
		if _, err := ParseManifest(filepath.Join(realPath, manifestName)); err != nil {
			return entrySkip, Addon{}, err
		}
	}

	return entryAddon, Addon{
		Name:       name,
		SourcePath: entryPath,
		RealPath:   realPath,
		IsSymlink:  isSymlink,
	}, nil
}

func findManifest(dir string) (string, bool) {
	for _, name := range manifestFilenames {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !info.IsDir() {
			return name, true
		}
	}
	return "", false
}

func detectDuplicates(addons []Addon) ([]Addon, []error) {
	byName := make(map[string][]Addon)
	for _, a := range addons {
		byName[a.Name] = append(byName[a.Name], a)
	}

	var errs []error
	var filtered []Addon
	for _, a := range addons {
		if len(byName[a.Name]) == 1 {
			filtered = append(filtered, a)
		}
	}

	for name, group := range byName {
		if len(group) < 2 {
			continue
		}
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				errs = append(errs, fmt.Errorf("addons: duplicate addon %q found at %s and %s", name, group[i].SourcePath, group[j].SourcePath))
			}
		}
	}

	return filtered, errs
}
