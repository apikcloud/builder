// SPDX-License-Identifier: MIT
package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/workspace"
)

func TestPrepareCmd_ReportsAddonCount(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoDir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })

	cmd := newPrepareCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "prepared build context at")
	assert.Contains(t, out.String(), "(1 addon(s))")
}
