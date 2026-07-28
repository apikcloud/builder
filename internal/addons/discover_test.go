package addons_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/addons"
)

func TestDiscover_SingleValidAddon(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "addons", "sale_custom"), validManifest("Sale Custom"))

	found, errs := addons.Discover(root, []string{"addons"}, nil, true)
	require.Empty(t, errs)
	require.Len(t, found, 1)
	assert.Equal(t, "sale_custom", found[0].Name)
	assert.False(t, found[0].IsSymlink)
}

func TestDiscover_NestedCategoryFolder_NotDescendedIntoModule(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "addons", "oca", "web_module"), validManifest("Web Module"))
	// A file inside the module's own subtree that would break discovery if
	// it were mistakenly re-scanned as a category folder.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "oca", "web_module", "static"), 0o755))

	found, errs := addons.Discover(root, []string{"addons"}, nil, true)
	require.Empty(t, errs)
	require.Len(t, found, 1)
	assert.Equal(t, "web_module", found[0].Name)
}

func TestDiscover_ExcludedAddon_SkippedAndNotDuplicate(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "addons", "test_module"), validManifest("Test Module"))
	writeManifest(t, filepath.Join(root, "addons", "kept_module"), validManifest("Kept Module"))

	found, errs := addons.Discover(root, []string{"addons"}, []string{"test_module"}, true)
	require.Empty(t, errs)
	require.Len(t, found, 1)
	assert.Equal(t, "kept_module", found[0].Name)
}

func TestDiscover_SymlinkedAddonRoot_Resolved(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real_modules", "linked_addon")
	writeManifest(t, realDir, validManifest("Linked Addon"))

	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	linkPath := filepath.Join(root, "addons", "linked_addon")
	require.NoError(t, os.Symlink(realDir, linkPath))

	found, errs := addons.Discover(root, []string{"addons"}, nil, true)
	require.Empty(t, errs)
	require.Len(t, found, 1)
	assert.Equal(t, "linked_addon", found[0].Name)
	assert.True(t, found[0].IsSymlink)

	resolvedReal, err := filepath.EvalSymlinks(realDir)
	require.NoError(t, err)
	assert.Equal(t, resolvedReal, found[0].RealPath)
}

func TestDiscover_BrokenSymlinkAddonRoot_ReportedAndAbsent(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	linkPath := filepath.Join(root, "addons", "broken_addon")
	require.NoError(t, os.Symlink(filepath.Join(root, "does_not_exist"), linkPath))

	found, errs := addons.Discover(root, []string{"addons"}, nil, true)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "broken symlink")
	assert.Empty(t, found)
}

func TestDiscover_InvalidManifest_ReportedAndAbsent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "addons", "broken_module")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "__manifest__.py"), []byte("{'name': 'Broken'"), 0o644))

	found, errs := addons.Discover(root, []string{"addons"}, nil, true)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "invalid manifest")
	assert.Empty(t, found)
}

func TestDiscover_InvalidManifest_ValidationDisabled_RegisteredDespiteInvalidContent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "addons", "broken_module")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "__manifest__.py"), []byte("{'name': 'Broken'"), 0o644))

	found, errs := addons.Discover(root, []string{"addons"}, nil, false)
	require.Empty(t, errs)
	require.Len(t, found, 1)
	assert.Equal(t, "broken_module", found[0].Name)
}

func TestDiscover_MissingManifest_ValidationDisabled_StillNotRegistered(t *testing.T) {
	root := t.TempDir()
	// No __manifest__.py at all: validateManifests=false only skips content
	// parsing, it never waives the manifest file's presence requirement.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "not_a_module"), 0o755))

	found, errs := addons.Discover(root, []string{"addons"}, nil, false)
	require.Empty(t, errs)
	assert.Empty(t, found)
}

func TestDiscover_DuplicateAddonName_ReportedAndAbsent(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "addons", "dup_module"), validManifest("Dup A"))
	writeManifest(t, filepath.Join(root, "addons-extra", "dup_module"), validManifest("Dup B"))

	found, errs := addons.Discover(root, []string{"addons", "addons-extra"}, nil, true)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "dup_module")
	assert.Contains(t, errs[0].Error(), filepath.Join(root, "addons", "dup_module"))
	assert.Contains(t, errs[0].Error(), filepath.Join(root, "addons-extra", "dup_module"))
	assert.Empty(t, found)
}

func TestDiscover_MultipleAddons_SortableByName(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "addons", "zeta"), validManifest("Zeta"))
	writeManifest(t, filepath.Join(root, "addons", "alpha"), validManifest("Alpha"))

	found, errs := addons.Discover(root, []string{"addons"}, nil, true)
	require.Empty(t, errs)
	require.Len(t, found, 2)

	names := []string{found[0].Name, found[1].Name}
	sort.Strings(names)
	assert.Equal(t, []string{"alpha", "zeta"}, names)
}

func TestDiscoverAt_MissingRoot_ReturnsEmptyNoError(t *testing.T) {
	found, errs := addons.DiscoverAt(filepath.Join(t.TempDir(), "missing"), nil, true)
	require.Empty(t, errs)
	assert.Empty(t, found)
}

func TestDedup_CrossRootDuplicate_ReportedAndAbsent(t *testing.T) {
	rootA := t.TempDir()
	writeManifest(t, filepath.Join(rootA, "same_name"), validManifest("A"))
	rootB := t.TempDir()
	writeManifest(t, filepath.Join(rootB, "same_name"), validManifest("B"))

	foundA, errsA := addons.DiscoverAt(rootA, nil, true)
	require.Empty(t, errsA)
	foundB, errsB := addons.DiscoverAt(rootB, nil, true)
	require.Empty(t, errsB)

	deduped, errs := addons.Dedup(append(foundA, foundB...))
	require.Empty(t, deduped)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "same_name")
	assert.Contains(t, errs[0].Error(), filepath.Join(rootA, "same_name"))
	assert.Contains(t, errs[0].Error(), filepath.Join(rootB, "same_name"))
}

func TestDiscoverShallow_TopLevelAddon_Found(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "root_module"), validManifest("Root Module"))

	found, errs := addons.DiscoverShallow(root, nil, true)
	require.Empty(t, errs)
	require.Len(t, found, 1)
	assert.Equal(t, "root_module", found[0].Name)
}

func TestDiscoverShallow_SymlinkedTopLevelAddon_Found(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, ".third-party", "vendor", "linked_addon")
	writeManifest(t, realDir, validManifest("Linked Addon"))
	require.NoError(t, os.Symlink(realDir, filepath.Join(root, "linked_addon")))

	found, errs := addons.DiscoverShallow(root, nil, true)
	require.Empty(t, errs)
	require.Len(t, found, 1)
	assert.Equal(t, "linked_addon", found[0].Name)
	assert.True(t, found[0].IsSymlink)
}

func TestDiscoverShallow_ContainerDirectory_NotRecursedInto(t *testing.T) {
	root := t.TempDir()
	// A container directory holding real addon code one level below its own
	// top level (e.g. `.third-party/vendor/module`, or an `addons/` folder
	// also covered by a dedicated include path) must not be descended into:
	// only root's own direct children are ever considered.
	writeManifest(t, filepath.Join(root, "addons", "sale_custom"), validManifest("Sale Custom"))

	found, errs := addons.DiscoverShallow(root, nil, true)
	require.Empty(t, errs)
	assert.Empty(t, found)
}

func TestDiscoverShallow_ExcludedName_Skipped(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "test_module"), validManifest("Test Module"))
	writeManifest(t, filepath.Join(root, "kept_module"), validManifest("Kept Module"))

	found, errs := addons.DiscoverShallow(root, []string{"test_module"}, true)
	require.Empty(t, errs)
	require.Len(t, found, 1)
	assert.Equal(t, "kept_module", found[0].Name)
}

func TestDiscoverShallow_MissingRoot_ReturnsEmptyNoError(t *testing.T) {
	found, errs := addons.DiscoverShallow(filepath.Join(t.TempDir(), "missing"), nil, true)
	require.Empty(t, errs)
	assert.Empty(t, found)
}

func writeManifest(t *testing.T, dir string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "__manifest__.py"), []byte(content), 0o644))
}

func validManifest(name string) string {
	return "{\n    'name': '" + name + "',\n    'installable': True,\n}\n"
}
