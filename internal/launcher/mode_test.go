// SPDX-License-Identifier: MIT
package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMode(t *testing.T) {
	t.Run("no flag no env defaults to auto", func(t *testing.T) {
		mode, err := ResolveMode("")
		require.NoError(t, err)
		assert.Equal(t, ModeAuto, mode)
	})

	t.Run("flag wins regardless of env", func(t *testing.T) {
		t.Setenv(ModeEnvVar, "launcher")
		mode, err := ResolveMode("engine")
		require.NoError(t, err)
		assert.Equal(t, ModeEngine, mode)
	})

	t.Run("env used when no flag", func(t *testing.T) {
		t.Setenv(ModeEnvVar, "launcher")
		mode, err := ResolveMode("")
		require.NoError(t, err)
		assert.Equal(t, ModeLauncher, mode)
	})

	t.Run("invalid flag value errors", func(t *testing.T) {
		_, err := ResolveMode("bogus")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("invalid env value errors", func(t *testing.T) {
		t.Setenv(ModeEnvVar, "bogus")
		_, err := ResolveMode("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})
}
