// SPDX-License-Identifier: MIT
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/builder/internal/engine"
)

func TestInspectCmd_PrintsResolvedSpec(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	resolved := &engine.ResolvedSpec{Base: engine.BaseImageSpec{Image: "odoo:17.0-20240101"}}
	rec := &recordingInvoke{resp: engine.BuildResponse{Resolved: resolved}}
	stubInvokeEngine(t, rec)

	cmd := newInspectCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))

	var got engine.ResolvedSpec
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, "odoo:17.0-20240101", got.Base.Image)

	require.Len(t, rec.calls, 1)
	assert.Equal(t, engine.APIVersion, rec.calls[0].APIVersion)
	assert.Equal(t, engine.CommandInspect, rec.calls[0].Command)
	assert.Equal(t, repoDir, rec.calls[0].RepoRoot)
}

func TestInspectCmd_InvokeError_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{err: assert.AnError}
	stubInvokeEngine(t, rec)

	cmd := newInspectCmd()
	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
}
