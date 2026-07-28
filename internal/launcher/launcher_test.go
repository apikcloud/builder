// SPDX-License-Identifier: MIT
package launcher

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/engine"
)

func stubLookPath(t *testing.T, found map[string]bool) {
	t.Helper()
	old := lookPath
	lookPath = func(file string) (string, error) {
		if found[file] {
			return "/usr/bin/" + file, nil
		}
		return "", fmt.Errorf("%s: not found", file)
	}
	t.Cleanup(func() { lookPath = old })
}

func TestNeeded(t *testing.T) {
	t.Run("engine binary missing always needs container, regardless of command", func(t *testing.T) {
		stubLookPath(t, map[string]bool{"odoo-builder-engine": false, "buildctl": true, "buildkitd": true})
		assert.True(t, Needed(engine.CommandBuild))
		assert.True(t, Needed(engine.CommandValidate))
	})

	t.Run("engine binary present, non-build commands never need container", func(t *testing.T) {
		stubLookPath(t, map[string]bool{"odoo-builder-engine": true, "buildctl": false, "buildkitd": false})
		assert.False(t, Needed(engine.CommandValidate))
		assert.False(t, Needed(engine.CommandPrepare))
		assert.False(t, Needed(engine.CommandInspect))
	})

	t.Run("engine binary present, build command depends on buildctl/buildkitd", func(t *testing.T) {
		stubLookPath(t, map[string]bool{"odoo-builder-engine": true, "buildctl": true, "buildkitd": true})
		assert.False(t, Needed(engine.CommandBuild))

		stubLookPath(t, map[string]bool{"odoo-builder-engine": true, "buildctl": false, "buildkitd": true})
		assert.True(t, Needed(engine.CommandBuild))

		stubLookPath(t, map[string]bool{"odoo-builder-engine": true, "buildctl": true, "buildkitd": false})
		assert.True(t, Needed(engine.CommandBuild))

		stubLookPath(t, map[string]bool{"odoo-builder-engine": true, "buildctl": false, "buildkitd": false})
		assert.True(t, Needed(engine.CommandBuild))
	})
}

func TestDetectRuntime(t *testing.T) {
	stubLookPath(t, map[string]bool{"docker": true, "podman": true})
	runtime, err := DetectRuntime()
	require.NoError(t, err)
	assert.Equal(t, Docker, runtime)

	stubLookPath(t, map[string]bool{"docker": false, "podman": true})
	runtime, err = DetectRuntime()
	require.NoError(t, err)
	assert.Equal(t, Podman, runtime)

	stubLookPath(t, map[string]bool{"docker": false, "podman": false})
	_, err = DetectRuntime()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker")
	assert.Contains(t, err.Error(), "podman")
}

func TestImageRef(t *testing.T) {
	assert.Equal(t, DefaultImage, ImageRef())

	t.Setenv(ImageEnvVar, "custom:tag")
	assert.Equal(t, "custom:tag", ImageRef())
}

func TestForwardedEnv(t *testing.T) {
	assert.Empty(t, ForwardedEnv())

	t.Setenv("BUILDKIT_HOST", "unix:///tmp/x.sock")
	assert.Equal(t, []string{"BUILDKIT_HOST=unix:///tmp/x.sock"}, ForwardedEnv())
}
