package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/engine"
	"github.com/apikcloud/odoo-builder/internal/workspace"
)

func TestInspectCmd_PrintsResolvedBuildRequest(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	cmd := newInspectCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))

	var req engine.BuildRequest
	require.NoError(t, json.Unmarshal(out.Bytes(), &req))
	assert.Equal(t, "oci", req.Output.Type)
	assert.True(t, filepath.IsAbs(req.Output.Path))
	assert.Contains(t, req.Output.Path, "image.oci.tar")

	_, err = os.Stat(filepath.Join(repoDir, ".build"))
	assert.True(t, os.IsNotExist(err))
}

func TestInspectCmd_MalformedConfig_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "odoo-builder.yaml"),
		[]byte("image:\n  name: \"app:v1\"\n"), 0o644))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	cmd := newInspectCmd()
	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
}
