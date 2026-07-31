// SPDX-License-Identifier: MIT
package prepare_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/config"
	"github.com/apikcloud/odoo-builder/internal/enterprise"
	"github.com/apikcloud/odoo-builder/internal/prepare"
	"github.com/apikcloud/odoo-builder/internal/workspace"
)

func TestPrepare_MatchesExpectedTree(t *testing.T) {
	buildDir := filepath.Join(t.TempDir(), ".build")

	count, err := prepare.Prepare("../../testdata/simple", buildDir, config.Default())
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	assert.Equal(t, treeContents(t, "../../testdata/expected/simple"), treeContents(t, buildDir))
}

func TestPrepareDeterministic(t *testing.T) {
	dir1 := filepath.Join(t.TempDir(), ".build")
	dir2 := filepath.Join(t.TempDir(), ".build")

	_, err := prepare.Prepare("../../testdata/simple", dir1, config.Default())
	require.NoError(t, err)
	_, err = prepare.Prepare("../../testdata/simple", dir2, config.Default())
	require.NoError(t, err)

	assert.Equal(t, treeContents(t, dir1), treeContents(t, dir2))
}

func TestPrepare_ResolvesSymlinkedAddon(t *testing.T) {
	repoCopy := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, workspace.CopyDir("../../testdata/symlinks", repoCopy))
	require.NoError(t, os.Remove(filepath.Join(repoCopy, "addons", "broken_addon")))

	buildDir := filepath.Join(t.TempDir(), ".build")
	count, err := prepare.Prepare(repoCopy, buildDir, config.Default())
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	assert.Equal(t, treeContents(t, "../../testdata/expected/symlinks"), treeContents(t, buildDir))

	err = filepath.WalkDir(buildDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		assert.Zero(t, d.Type()&fs.ModeSymlink, "output tree must contain no symlinks: %s", path)
		return nil
	})
	require.NoError(t, err)
}

func TestPrepare_AddonAtRepoRoot_Discovered(t *testing.T) {
	repoCopy := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoCopy))
	require.NoError(t, os.MkdirAll(filepath.Join(repoCopy, "root_module"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoCopy, "root_module", "__manifest__.py"), []byte("{'name': 'Root Module'}"), 0o644))

	buildDir := filepath.Join(t.TempDir(), ".build")
	count, err := prepare.Prepare(repoCopy, buildDir, config.Default())
	require.NoError(t, err)
	assert.Equal(t, 2, count) // sale_custom (addons/) + root_module (repo root)

	_, err = os.Stat(filepath.Join(buildDir, "addons", "root_module", "__manifest__.py"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(buildDir, "addons", "sale_custom", "__manifest__.py"))
	require.NoError(t, err)
}

func TestPrepare_BrokenSymlinkAddon_ReturnsError(t *testing.T) {
	buildDir := filepath.Join(t.TempDir(), ".build")

	_, err := prepare.Prepare("../../testdata/symlinks", buildDir, config.Default())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken_addon")
}

func TestPrepare_DuplicateAddon_ReturnsError(t *testing.T) {
	cfg, err := config.Load("../../testdata/duplicates/odoo-builder.yaml")
	require.NoError(t, err)

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err = prepare.Prepare("../../testdata/duplicates", buildDir, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dup_module")
	assert.Contains(t, err.Error(), filepath.Join("addons", "dup_module"))
	assert.Contains(t, err.Error(), filepath.Join("addons-extra", "dup_module"))
}

func TestPrepare_EmptyPackagesTxt_OmitsAptBlock(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "some_module"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "addons", "some_module", "__manifest__.py"), []byte("{'name': 'Some Module'}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "packages.txt"), []byte("\n  \n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte("odoo:18.0-20260723\n"), 0o644))

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare(root, buildDir, config.Default())
	require.NoError(t, err)

	dockerfileContent, err := os.ReadFile(filepath.Join(buildDir, "Dockerfile"))
	require.NoError(t, err)
	assert.NotContains(t, string(dockerfileContent), "packages.txt")
	assert.NotContains(t, string(dockerfileContent), "apt-get")
}

func TestPrepare_CommentOnlyPackagesTxt_OmitsAptBlock(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "some_module"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "addons", "some_module", "__manifest__.py"), []byte("{'name': 'Some Module'}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "packages.txt"), []byte("# packages.txt\n\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte("odoo:18.0-20260723\n"), 0o644))

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare(root, buildDir, config.Default())
	require.NoError(t, err)

	dockerfileContent, err := os.ReadFile(filepath.Join(buildDir, "Dockerfile"))
	require.NoError(t, err)
	assert.NotContains(t, string(dockerfileContent), "packages.txt")
	assert.NotContains(t, string(dockerfileContent), "apt-get")
}

func TestPrepare_EmptyRequirementsTxt_OmitsPipBlock(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "some_module"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "addons", "some_module", "__manifest__.py"), []byte("{'name': 'Some Module'}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("\n  \n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte("odoo:18.0-20260723\n"), 0o644))

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare(root, buildDir, config.Default())
	require.NoError(t, err)

	dockerfileContent, err := os.ReadFile(filepath.Join(buildDir, "Dockerfile"))
	require.NoError(t, err)
	assert.NotContains(t, string(dockerfileContent), "requirements.txt")
	assert.NotContains(t, string(dockerfileContent), "pip install")
}

func TestPrepare_CommentOnlyRequirementsTxt_OmitsPipBlock(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "some_module"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "addons", "some_module", "__manifest__.py"), []byte("{'name': 'Some Module'}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("# requirements.txt\n\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte("odoo:18.0-20260723\n"), 0o644))

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare(root, buildDir, config.Default())
	require.NoError(t, err)

	dockerfileContent, err := os.ReadFile(filepath.Join(buildDir, "Dockerfile"))
	require.NoError(t, err)
	assert.NotContains(t, string(dockerfileContent), "requirements.txt")
	assert.NotContains(t, string(dockerfileContent), "pip install")
}

func TestPrepare_RequirementsTxtWithComments_KeepsPipBlock(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "some_module"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "addons", "some_module", "__manifest__.py"), []byte("{'name': 'Some Module'}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("# top-level comment\nrequests==2.31.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte("odoo:18.0-20260723\n"), 0o644))

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare(root, buildDir, config.Default())
	require.NoError(t, err)

	dockerfileContent, err := os.ReadFile(filepath.Join(buildDir, "Dockerfile"))
	require.NoError(t, err)
	// pip parses "#" comments in requirements.txt natively — unlike
	// packages.txt's apt-get/xargs, no filtering is needed before install.
	assert.Contains(t, string(dockerfileContent), "pip install --no-cache-dir -r /tmp/requirements.txt")
}

func TestPrepare_NoBaseVersion_ReturnsError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "some_module"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "addons", "some_module", "__manifest__.py"), []byte("{'name': 'Some Module'}"), 0o644))

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare(root, buildDir, config.Default())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "odoo_version.txt")
}

func TestPrepare_SkipManifestValidation_ToleratesInvalidManifestContent(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons", "broken_module"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "addons", "broken_module", "__manifest__.py"), []byte("{'name': 'Broken'"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte("odoo:18.0-20260723\n"), 0o644))

	cfg := config.Default()
	cfg.Addons.SkipManifestValidation = true

	buildDir := filepath.Join(t.TempDir(), ".build")
	count, err := prepare.Prepare(root, buildDir, cfg)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	_, err = os.Stat(filepath.Join(buildDir, "addons", "broken_module", "__manifest__.py"))
	require.NoError(t, err)
}

func TestPrepare_Enterprise(t *testing.T) {
	entFixture := t.TempDir()
	writeManifest(t, filepath.Join(entFixture, "enterprise_addon"), validManifestEnt("Enterprise Addon"))

	old := prepare.EnterpriseFetchFunc
	prepare.EnterpriseFetchFunc = func(_, _ string) (string, func(), error) {
		return entFixture, func() {}, nil
	}
	t.Cleanup(func() { prepare.EnterpriseFetchFunc = old })
	t.Setenv(enterprise.TokenEnvVar, "tok")

	cfg := config.Default()
	cfg.Enterprise.Enabled = true
	cfg.Base.Version = "18.0"

	buildDir := filepath.Join(t.TempDir(), ".build")
	count, err := prepare.Prepare("../../testdata/simple", buildDir, cfg)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // sale_custom + enterprise_addon

	_, err = os.Stat(filepath.Join(buildDir, "addons", "sale_custom", "__manifest__.py"))
	require.NoError(t, err, "community addons must stay under addons/")
	_, err = os.Stat(filepath.Join(buildDir, "enterprise-addons", "enterprise_addon", "__manifest__.py"))
	require.NoError(t, err, "enterprise addons must be copied under their own enterprise-addons/ directory")
	_, err = os.Stat(filepath.Join(buildDir, "addons", "enterprise_addon"))
	assert.True(t, os.IsNotExist(err), "enterprise addons must not also appear under addons/")
}

func TestPrepare_Enterprise_ExplicitCommit_SkipsDateAndCloneResolution(t *testing.T) {
	entFixture := t.TempDir()
	writeManifest(t, filepath.Join(entFixture, "enterprise_addon"), validManifestEnt("Enterprise Addon"))

	oldFetch, oldResolve := prepare.EnterpriseFetchFunc, prepare.EnterpriseResolveCommitFunc
	t.Cleanup(func() {
		prepare.EnterpriseFetchFunc = oldFetch
		prepare.EnterpriseResolveCommitFunc = oldResolve
	})

	var fetchedRef string
	prepare.EnterpriseFetchFunc = func(ref, token string) (string, func(), error) {
		fetchedRef = ref
		return entFixture, func() {}, nil
	}
	prepare.EnterpriseResolveCommitFunc = func(branch, date, token string) (string, error) {
		t.Fatal("ResolveCommit must not be called when enterprise.commit is set")
		return "", nil
	}
	t.Setenv(enterprise.TokenEnvVar, "tok")

	cfg := config.Default()
	cfg.Enterprise.Enabled = true
	cfg.Enterprise.Commit = "deadbeef"
	// Deliberately no base.version/base.release: an explicit commit must
	// need neither.

	buildDir := filepath.Join(t.TempDir(), ".build")
	count, err := prepare.Prepare("../../testdata/simple", buildDir, cfg)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, "deadbeef", fetchedRef)
}

func TestPrepare_Enterprise_DateResolvesCommitBeforeDownloading(t *testing.T) {
	entFixture := t.TempDir()
	writeManifest(t, filepath.Join(entFixture, "enterprise_addon"), validManifestEnt("Enterprise Addon"))

	oldFetch, oldResolve := prepare.EnterpriseFetchFunc, prepare.EnterpriseResolveCommitFunc
	t.Cleanup(func() {
		prepare.EnterpriseFetchFunc = oldFetch
		prepare.EnterpriseResolveCommitFunc = oldResolve
	})

	var resolvedBranch, resolvedDate, fetchedRef string
	prepare.EnterpriseResolveCommitFunc = func(branch, date, token string) (string, error) {
		resolvedBranch, resolvedDate = branch, date
		return "resolvedsha", nil
	}
	prepare.EnterpriseFetchFunc = func(ref, token string) (string, func(), error) {
		fetchedRef = ref
		return entFixture, func() {}, nil
	}
	t.Setenv(enterprise.TokenEnvVar, "tok")

	cfg := config.Default()
	cfg.Enterprise.Enabled = true
	cfg.Base.Version = "18.0"
	cfg.Base.Release = "20250611"

	buildDir := filepath.Join(t.TempDir(), ".build")
	count, err := prepare.Prepare("../../testdata/simple", buildDir, cfg)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, "18.0", resolvedBranch)
	assert.Equal(t, "20250611", resolvedDate, "must default to base.release when enterprise.date is unset")
	assert.Equal(t, "resolvedsha", fetchedRef)
}

func TestPrepare_Enterprise_ExplicitDateOverridesBaseRelease(t *testing.T) {
	entFixture := t.TempDir()
	writeManifest(t, filepath.Join(entFixture, "enterprise_addon"), validManifestEnt("Enterprise Addon"))

	oldFetch, oldResolve := prepare.EnterpriseFetchFunc, prepare.EnterpriseResolveCommitFunc
	t.Cleanup(func() {
		prepare.EnterpriseFetchFunc = oldFetch
		prepare.EnterpriseResolveCommitFunc = oldResolve
	})

	var resolvedDate string
	prepare.EnterpriseResolveCommitFunc = func(branch, date, token string) (string, error) {
		resolvedDate = date
		return "resolvedsha", nil
	}
	prepare.EnterpriseFetchFunc = func(ref, token string) (string, func(), error) {
		return entFixture, func() {}, nil
	}
	t.Setenv(enterprise.TokenEnvVar, "tok")

	cfg := config.Default()
	cfg.Enterprise.Enabled = true
	cfg.Base.Version = "18.0"
	cfg.Base.Release = "20250611"
	cfg.Enterprise.Date = "20250101"

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare("../../testdata/simple", buildDir, cfg)
	require.NoError(t, err)
	assert.Equal(t, "20250101", resolvedDate)
}

func TestPrepare_EnterpriseTokenMissing_ReturnsError(t *testing.T) {
	// EnterpriseFetchFunc is left at its default (the real enterprise.Fetch):
	// an empty token must fail fast, before any network activity, which is
	// exactly what this test verifies.
	t.Setenv(enterprise.TokenEnvVar, "")

	cfg := config.Default()
	cfg.Enterprise.Enabled = true
	cfg.Base.Version = "18.0"

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare("../../testdata/simple", buildDir, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), enterprise.TokenEnvVar)

	_, statErr := os.Stat(filepath.Join(buildDir, "addons", "enterprise_addon"))
	assert.True(t, os.IsNotExist(statErr), ".build/ must contain no partial Enterprise content")
}

func TestPrepare_EnterpriseDuplicatesCommunity_ReturnsError(t *testing.T) {
	entFixture := t.TempDir()
	writeManifest(t, filepath.Join(entFixture, "sale_custom"), validManifestEnt("Enterprise Sale Custom"))

	old := prepare.EnterpriseFetchFunc
	prepare.EnterpriseFetchFunc = func(_, _ string) (string, func(), error) {
		return entFixture, func() {}, nil
	}
	t.Cleanup(func() { prepare.EnterpriseFetchFunc = old })
	t.Setenv(enterprise.TokenEnvVar, "tok")

	cfg := config.Default()
	cfg.Enterprise.Enabled = true
	cfg.Base.Version = "18.0"

	buildDir := filepath.Join(t.TempDir(), ".build")
	_, err := prepare.Prepare("../../testdata/simple", buildDir, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sale_custom")
}

// treeContents walks root and returns relative-path -> file-content for
// every regular file, skipping .gitkeep placeholders used to keep empty
// directories under git.
func treeContents(t *testing.T, root string) map[string]string {
	t.Helper()

	contents := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() == ".gitkeep" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		contents[rel] = string(data)
		return nil
	})
	require.NoError(t, err)

	return contents
}
