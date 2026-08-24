// SPDX-License-Identifier: MIT
package launcher

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/builder/internal/engine"
	"github.com/apikcloud/builder/internal/version"
)

// withVersion sets version.Version for the duration of t, restoring it
// afterwards — version.Version is normally fixed at build time via
// -ldflags, but ImageRef reads it at runtime, so tests drive it directly.
func withVersion(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
}

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
	t.Run("dev/dirty version falls back to latest", func(t *testing.T) {
		withVersion(t, "dev")
		assert.Equal(t, DefaultImageRepo+":latest", ImageRef())

		withVersion(t, "v0.4.2-3-gabc123-dirty")
		assert.Equal(t, DefaultImageRepo+":latest", ImageRef())
	})

	t.Run("tagged release version pins the matching image tag", func(t *testing.T) {
		withVersion(t, "v0.4.2")
		assert.Equal(t, DefaultImageRepo+":v0.4.2", ImageRef())
	})

	t.Run("env var overrides everything, tag included", func(t *testing.T) {
		withVersion(t, "v0.4.2")
		t.Setenv(ImageEnvVar, "custom:tag")
		assert.Equal(t, "custom:tag", ImageRef())
	})
}

func TestForwardedEnv(t *testing.T) {
	assert.Empty(t, ForwardedEnv())

	t.Setenv("BUILDKIT_HOST", "unix:///tmp/x.sock")
	assert.Equal(t, []string{"BUILDKIT_HOST=unix:///tmp/x.sock"}, ForwardedEnv())

	t.Setenv("BUILDKIT_TLS_CERT", "/tls/cert.pem")
	t.Setenv("BUILDKIT_TLS_KEY", "/tls/key.pem")
	t.Setenv("BUILDKIT_TLS_CACERT", "/tls/ca.pem")
	assert.Equal(t, []string{
		"BUILDKIT_HOST=unix:///tmp/x.sock",
		"BUILDKIT_TLS_CERT=/tls/cert.pem",
		"BUILDKIT_TLS_KEY=/tls/key.pem",
		"BUILDKIT_TLS_CACERT=/tls/ca.pem",
	}, ForwardedEnv())
}
