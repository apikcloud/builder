// SPDX-License-Identifier: MIT
package prepare_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func validManifestEnt(name string) string {
	return "{\n    'name': '" + name + "',\n    'installable': True,\n}\n"
}

// writeManifest creates dir and writes a __manifest__.py inside it holding
// content, for tests faking Enterprise download output.
func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "__manifest__.py"), []byte(content), 0o644))
}
