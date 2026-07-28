// SPDX-License-Identifier: MIT
package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildArgs(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		got := BuildArgs("img:tag", "/ws", "", "", "", nil, []string{"build"})
		assert.Equal(t, []string{
			"run", "--rm", "--privileged", "-i",
			"-v", "/ws:/workspace",
			"-w", "/workspace",
			"img:tag", "build",
		}, got)
	})

	t.Run("with docker config dir", func(t *testing.T) {
		got := BuildArgs("img:tag", "/ws", "/home/u/.docker", "", "", nil, []string{"build"})
		assert.Equal(t, []string{
			"run", "--rm", "--privileged", "-i",
			"-v", "/ws:/workspace",
			"-w", "/workspace",
			"-v", "/home/u/.docker:/root/.docker:ro",
			"img:tag", "build",
		}, got)
	})

	t.Run("with host cache dir", func(t *testing.T) {
		got := BuildArgs("img:tag", "/ws", "", "/home/u/.cache/odoo-builder", "", []string{"XDG_CACHE_HOME=/host-cache"}, []string{"build"})
		assert.Equal(t, []string{
			"run", "--rm", "--privileged", "-i",
			"-v", "/ws:/workspace",
			"-w", "/workspace",
			"-v", "/home/u/.cache/odoo-builder:/host-cache/odoo-builder",
			"-e", "XDG_CACHE_HOME=/host-cache",
			"img:tag", "build",
		}, got)
	})

	t.Run("with host buildkit root dir", func(t *testing.T) {
		got := BuildArgs("img:tag", "/ws", "", "", "/hbk", []string{"ODOO_BUILDER_BUILDKITD_ROOT=/host-buildkit-root"}, []string{"build"})
		assert.Equal(t, []string{
			"run", "--rm", "--privileged", "-i",
			"-v", "/ws:/workspace",
			"-w", "/workspace",
			"-v", "/hbk:/host-buildkit-root",
			"-e", "ODOO_BUILDER_BUILDKITD_ROOT=/host-buildkit-root",
			"img:tag", "build",
		}, got)
	})

	t.Run("with env entries", func(t *testing.T) {
		got := BuildArgs("img:tag", "/ws", "", "", "", []string{"A=1", "B=2"}, []string{"build"})
		assert.Equal(t, []string{
			"run", "--rm", "--privileged", "-i",
			"-v", "/ws:/workspace",
			"-w", "/workspace",
			"-e", "A=1",
			"-e", "B=2",
			"img:tag", "build",
		}, got)
	})

	t.Run("args always last", func(t *testing.T) {
		got := BuildArgs("img:tag", "/ws", "/dc", "/hc", "/hbk", []string{"A=1"}, []string{"build", "--foo"})
		assert.Equal(t, []string{
			"run", "--rm", "--privileged", "-i",
			"-v", "/ws:/workspace",
			"-w", "/workspace",
			"-v", "/dc:/root/.docker:ro",
			"-v", "/hc:/host-cache/odoo-builder",
			"-v", "/hbk:/host-buildkit-root",
			"-e", "A=1",
			"img:tag", "build", "--foo",
		}, got)
	})
}
