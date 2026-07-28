// SPDX-License-Identifier: MIT
package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/engine"
)

func TestValidateCmd_OK(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{resp: engine.BuildResponse{ValidationErrors: []string{}}}
	stubInvokeEngine(t, rec)

	cmd := newValidateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "OK")

	require.Len(t, rec.calls, 1)
	assert.Equal(t, engine.APIVersion, rec.calls[0].APIVersion)
	assert.Equal(t, engine.CommandValidate, rec.calls[0].Command)
	assert.Equal(t, repoDir, rec.calls[0].RepoRoot)
}

func TestValidateCmd_ReportsErrors(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{resp: engine.BuildResponse{ValidationErrors: []string{"no addons found", "missing base image"}}}
	stubInvokeEngine(t, rec)

	cmd := newValidateCmd()
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)

	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 error(s) found")
	assert.Contains(t, errOut.String(), "no addons found")
	assert.Contains(t, errOut.String(), "missing base image")
}

func TestValidateCmd_InvokeError_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{err: assert.AnError}
	stubInvokeEngine(t, rec)

	cmd := newValidateCmd()
	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
}
