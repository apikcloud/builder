// SPDX-License-Identifier: MIT
package prepare_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/enterprise"
	"github.com/apikcloud/odoo-builder/internal/prepare"
)

func TestValidate_ValidRepo_NoErrors(t *testing.T) {
	errs := prepare.Validate("../../testdata/simple")
	assert.Empty(t, errs)
}

func TestValidate_MissingAddonsDir_ReturnsError(t *testing.T) {
	root := t.TempDir()
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "addons")
}

func TestValidate_AddonOnlyAtRepoRoot_NoErrors(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "root_module")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "__manifest__.py"), []byte("{'name': 'Root Module'}"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	assert.Empty(t, errs)
}

func TestValidate_MalformedBuilderYAML_ReturnsError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo-builder.yaml"), []byte("addons:\n  include: [unterminated\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "odoo-builder.yaml")
}

func TestValidate_MalformedPackagesTxt_ReturnsError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "packages.txt"), []byte("wkhtmltopdf extra-arg\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "packages.txt")
}

func TestValidate_PackagesTxtCommentsAndBlankLines_NoError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "packages.txt"),
		[]byte("# packages.txt\n\nwkhtmltopdf\n# another comment\ngit\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	assert.Empty(t, errs)
}

func TestValidate_PackagesTxtOnlyCommentsAndBlanks_NoError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "packages.txt"), []byte("# packages.txt\n\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	assert.Empty(t, errs)
}

func TestValidate_MalformedImageName_ReturnsError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo-builder.yaml"), []byte("image:\n  name: \"bad name\"\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "registry:")
}

func TestValidate_BrokenSymlink_ReturnsError(t *testing.T) {
	errs := prepare.Validate("../../testdata/symlinks")
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "broken symlink")
	assert.Contains(t, errs[0].Error(), "broken_addon")
}

func TestValidate_DuplicateAddon_ReturnsError(t *testing.T) {
	errs := prepare.Validate("../../testdata/duplicates")
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "dup_module")
}

func TestValidate_SkipManifestValidation_ToleratesInvalidManifestContent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "addons", "broken_module")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "__manifest__.py"), []byte("{'name': 'Broken'"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo-builder.yaml"), []byte("addons:\n  skip_manifest_validation: true\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	assert.Empty(t, errs)
}

func TestValidate_InvalidManifest_ReturnsError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "addons", "broken_module")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "__manifest__.py"), []byte("{'name': 'Broken'"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "invalid manifest")
}

func TestValidate_NoBaseVersion_ReturnsError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))

	errs := prepare.Validate(root)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "odoo_version.txt")
}

func TestValidate_MalformedOdooVersionFile_ReturnsError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte("odoo:18.0\n"), 0o644))

	errs := prepare.Validate(root)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "release date")
}

func TestValidate_EnterpriseEnabledNoBaseVersion_ReturnsError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo-builder.yaml"), []byte("enterprise:\n  enabled: true\n"), 0o644))

	errs := prepare.Validate(root)
	require.NotEmpty(t, errs)
	assert.Contains(t, errors.Join(errs...).Error(), "base.version")
}

func TestValidate_EnterpriseEnabledWithCommit_NoBaseVersionRequired(t *testing.T) {
	t.Setenv(enterprise.TokenEnvVar, "tok")

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo-builder.yaml"), []byte("enterprise:\n  enabled: true\n  commit: deadbeef\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	for _, e := range errs {
		assert.NotContains(t, e.Error(), "base.version")
	}
}

func TestValidate_EnterpriseEnabledNoToken_ReturnsError(t *testing.T) {
	t.Setenv(enterprise.TokenEnvVar, "")

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo-builder.yaml"), []byte("enterprise:\n  enabled: true\nbase:\n  version: \"18.0\"\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	require.NotEmpty(t, errs)
	assert.Contains(t, errors.Join(errs...).Error(), enterprise.TokenEnvVar)
}

func TestValidate_InvalidPlatform_ReturnsError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo-builder.yaml"),
		[]byte("build:\n  platform:\n    - linux/amd64\n    - amd64\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	require.NotEmpty(t, errs)
	joined := errors.Join(errs...).Error()
	assert.Contains(t, joined, "amd64")
	assert.Contains(t, joined, "build.platform")
}

func TestValidate_ValidPlatforms_NoErrors(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "addons"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo-builder.yaml"),
		[]byte("build:\n  platform:\n    - linux/amd64\n    - linux/arm64\n"), 0o644))
	writeOdooVersion(t, root)

	errs := prepare.Validate(root)
	assert.Empty(t, errs)
}

func writeOdooVersion(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte("odoo:18.0-20260723\n"), 0o644))
}
