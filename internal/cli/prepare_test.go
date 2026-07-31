// SPDX-License-Identifier: MIT
package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/builder/internal/engine"
	"github.com/apikcloud/builder/internal/launcher"
)

func TestPrepareCmd_ReportsAddonCount(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	rec := &recordingInvoke{resp: engine.BuildResponse{BuildDir: repoDir + "/.build", AddonCount: 1}}
	stubInvokeEngine(t, rec)

	cmd := newPrepareCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "prepared build context at")
	assert.Contains(t, out.String(), "(1 addon(s))")

	require.Len(t, rec.calls, 1)
	assert.Equal(t, engine.APIVersion, rec.calls[0].APIVersion)
	assert.Equal(t, engine.CommandPrepare, rec.calls[0].Command)
	assert.Equal(t, repoDir, rec.calls[0].RepoRoot)
}

func TestPrepareCmd_InvokeError_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	old := invokeEngine
	invokeEngine = func(ctx context.Context, mode launcher.Mode, req engine.BuildRequest, stderr io.Writer) (engine.BuildResponse, error) {
		return engine.BuildResponse{}, assert.AnError
	}
	t.Cleanup(func() { invokeEngine = old })

	cmd := newPrepareCmd()
	err = cmd.RunE(cmd, nil)
	require.Error(t, err)
}
