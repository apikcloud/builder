package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/version"
)

func TestVersionCmd_PrintsVersion(t *testing.T) {
	cmd := newVersionCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), version.String())
}
