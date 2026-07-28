package dockerfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/config"
	"github.com/apikcloud/odoo-builder/internal/dockerfile"
)

func TestResolveBaseImage_VersionOnly(t *testing.T) {
	ref, err := dockerfile.ResolveBaseImage(t.TempDir(), config.Base{Version: "18.0"})
	require.NoError(t, err)
	assert.Equal(t, "odoo:18.0", ref)
}

func TestResolveBaseImage_VersionAndRelease(t *testing.T) {
	ref, err := dockerfile.ResolveBaseImage(t.TempDir(), config.Base{Version: "18.0", Release: "20250611"})
	require.NoError(t, err)
	assert.Equal(t, "odoo:18.0-20250611", ref)
}

func TestResolveBaseImage_VersionTakesPrecedenceOverFile(t *testing.T) {
	root := t.TempDir()
	writeOdooVersion(t, root, "odoo:17.0-20250101\n")

	ref, err := dockerfile.ResolveBaseImage(root, config.Base{Version: "18.0"})
	require.NoError(t, err)
	assert.Equal(t, "odoo:18.0", ref)
}

func TestResolveBaseImage_FallsBackToFile(t *testing.T) {
	root := t.TempDir()
	writeOdooVersion(t, root, "xxx/odoo-y:19.0-20260723-enterprise\n")

	ref, err := dockerfile.ResolveBaseImage(root, config.Base{})
	require.NoError(t, err)
	assert.Equal(t, "xxx/odoo-y:19.0-20260723-enterprise", ref)
}

func TestResolveBaseImage_NoSource_ReturnsError(t *testing.T) {
	_, err := dockerfile.ResolveBaseImage(t.TempDir(), config.Base{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "odoo_version.txt")
}

func TestResolveBaseImage_EmptyFile_ReturnsError(t *testing.T) {
	root := t.TempDir()
	writeOdooVersion(t, root, "")

	_, err := dockerfile.ResolveBaseImage(root, config.Base{})
	require.Error(t, err)
}

func TestResolveBaseImage_TwoNonBlankLines_ReturnsError(t *testing.T) {
	root := t.TempDir()
	writeOdooVersion(t, root, "odoo:18.0-20260723\nodoo:19.0-20260723\n")

	_, err := dockerfile.ResolveBaseImage(root, config.Base{})
	require.Error(t, err)
}

func TestResolveBaseImage_MissingTag_ReturnsError(t *testing.T) {
	root := t.TempDir()
	writeOdooVersion(t, root, "odoo\n")

	_, err := dockerfile.ResolveBaseImage(root, config.Base{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag")
}

func TestResolveBaseImage_NoReleaseDate_ReturnsError(t *testing.T) {
	root := t.TempDir()
	writeOdooVersion(t, root, "odoo:18.0\n")

	_, err := dockerfile.ResolveBaseImage(root, config.Base{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release date")
}

func TestResolveBaseImage_TrailingWhitespaceTrimmed(t *testing.T) {
	root := t.TempDir()
	writeOdooVersion(t, root, "  odoo:18.0-20260723  \n\n")

	ref, err := dockerfile.ResolveBaseImage(root, config.Base{})
	require.NoError(t, err)
	assert.Equal(t, "odoo:18.0-20260723", ref)
}

func writeOdooVersion(t *testing.T, root, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, "odoo_version.txt"), []byte(content), 0o644))
}
