// SPDX-License-Identifier: MIT
package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRequest_Normalize(t *testing.T) {
	t.Run("defaults derived from RepoRoot", func(t *testing.T) {
		req := BuildRequest{RepoRoot: "/repo"}.Normalize()
		assert.Equal(t, filepath.Join("/repo", ".build"), req.BuildDir)
	})

	t.Run("explicit values preserved", func(t *testing.T) {
		req := BuildRequest{
			RepoRoot: "/repo",
			BuildDir: "/custom/build",
			Output:   OutputSpec{Type: "registry", Path: "/custom/out.tar"},
		}.Normalize()
		assert.Equal(t, "/custom/build", req.BuildDir)
		assert.Equal(t, "registry", req.Output.Type)
		assert.Equal(t, "/custom/out.tar", req.Output.Path)
	})

	t.Run("does not mutate receiver, idempotent", func(t *testing.T) {
		req := BuildRequest{RepoRoot: "/repo"}
		first := req.Normalize()
		second := req.Normalize()
		assert.Equal(t, first, second)
		assert.Empty(t, req.BuildDir)
	})
}
