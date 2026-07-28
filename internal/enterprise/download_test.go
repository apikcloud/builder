package enterprise_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/enterprise"
)

// buildZip returns a zip archive whose entries are named
// "<topLevel>/<name>" for each name -> content pair, matching the shape of
// a GitHub zipball (always wrapped in one top-level folder).
func buildZip(t *testing.T, topLevel string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(topLevel + "/" + name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestDownload_ExtractsAndStripsTopLevelFolder(t *testing.T) {
	zipData := buildZip(t, "odoo-enterprise-abc123", map[string]string{
		"some_module/__manifest__.py": "{'name': 'Some Module'}\n",
	})

	withFakeGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/odoo/enterprise/zipball/deadbeef", r.URL.Path)
		assert.Equal(t, "token tok", r.Header.Get("Authorization"))
		w.Write(zipData)
	})

	dir, cleanup, err := enterprise.Download("deadbeef", "tok")
	require.NoError(t, err)
	defer cleanup()

	content, err := os.ReadFile(filepath.Join(dir, "some_module", "__manifest__.py"))
	require.NoError(t, err)
	assert.Equal(t, "{'name': 'Some Module'}\n", string(content))
}

func TestDownload_CleanupRemovesExtractedTree(t *testing.T) {
	zipData := buildZip(t, "top", map[string]string{"f.txt": "x\n"})
	withFakeGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) { w.Write(zipData) })

	dir, cleanup, err := enterprise.Download("deadbeef", "tok")
	require.NoError(t, err)
	cleanup()

	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}

func TestDownload_APIError_ReturnsError(t *testing.T) {
	withFakeGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, _, err := enterprise.Download("deadbeef", "tok")
	require.Error(t, err)
}

func TestDownload_EmptyCommit_ReturnsError(t *testing.T) {
	_, _, err := enterprise.Download("", "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit")
}

func TestDownload_EmptyToken_ReturnsError(t *testing.T) {
	_, _, err := enterprise.Download("deadbeef", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), enterprise.TokenEnvVar)
}

func TestDownload_ZipSlip_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("top/../../evil.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("pwned"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	withFakeGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) { w.Write(buf.Bytes()) })

	_, _, err = enterprise.Download("deadbeef", "tok")
	require.Error(t, err)
}
