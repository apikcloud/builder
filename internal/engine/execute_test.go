// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/buildkit"
	"github.com/apikcloud/odoo-builder/internal/workspace"
)

func TestExecute_UnsupportedAPIVersion(t *testing.T) {
	e := &Engine{}
	resp := e.Execute(context.Background(), BuildRequest{APIVersion: "v99", Command: CommandValidate})
	assert.Equal(t, ErrorCodeUnsupportedAPIVersion, resp.ErrorCode)
	assert.NotEmpty(t, resp.Error)
}

func TestExecute_UnknownCommand(t *testing.T) {
	e := &Engine{}
	resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: "bogus"})
	assert.Equal(t, ErrorCodeUnknownCommand, resp.ErrorCode)
	assert.NotEmpty(t, resp.Error)
}

func TestExecute_Validate(t *testing.T) {
	t.Run("success returns empty, non-nil ValidationErrors", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		e := &Engine{}
		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandValidate, RepoRoot: repoDir})
		assert.Empty(t, resp.ErrorCode)
		assert.NotNil(t, resp.ValidationErrors)
		assert.Empty(t, resp.ValidationErrors)
	})

	t.Run("failure sets ErrorCodeValidationFailed", func(t *testing.T) {
		e := &Engine{}
		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandValidate, RepoRoot: t.TempDir()})
		assert.Equal(t, ErrorCodeValidationFailed, resp.ErrorCode)
		assert.NotEmpty(t, resp.ValidationErrors)
	})
}

func TestExecute_Prepare(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		e := &Engine{}
		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandPrepare, RepoRoot: repoDir})
		assert.Empty(t, resp.ErrorCode)
		assert.Equal(t, 1, resp.AddonCount)
		assert.NotEmpty(t, resp.BuildDir)
	})

	t.Run("failure sets ErrorCodePrepareFailed", func(t *testing.T) {
		e := &Engine{}
		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandPrepare, RepoRoot: t.TempDir()})
		assert.Equal(t, ErrorCodePrepareFailed, resp.ErrorCode)
		assert.NotEmpty(t, resp.Error)
	})
}

func TestExecute_Inspect(t *testing.T) {
	t.Run("success populates Resolved", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		e := &Engine{}
		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandInspect, RepoRoot: repoDir})
		assert.Empty(t, resp.ErrorCode)
		require.NotNil(t, resp.Resolved)
		assert.NotEmpty(t, resp.Resolved.Base.Image)
	})

	t.Run("failure sets ErrorCodePrepareFailed", func(t *testing.T) {
		e := &Engine{}
		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandInspect, RepoRoot: t.TempDir()})
		assert.Equal(t, ErrorCodePrepareFailed, resp.ErrorCode)
		assert.NotEmpty(t, resp.Error)
	})
}

func TestExecute_Build(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		runner := &fakeRunner{out: buildkit.BuildOutput{OutputPath: "/fake/image.oci.tar"}}
		e := &Engine{Runner: runner}

		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandBuild, RepoRoot: repoDir})
		assert.Empty(t, resp.ErrorCode)
		assert.Equal(t, "/fake/image.oci.tar", resp.ImagePath)
		assert.Equal(t, 1, resp.AddonCount)
	})

	t.Run("validation failure sets ErrorCodeValidationFailed", func(t *testing.T) {
		e := &Engine{Runner: &fakeRunner{}}
		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandBuild, RepoRoot: t.TempDir()})
		assert.Equal(t, ErrorCodeValidationFailed, resp.ErrorCode)
		assert.NotEmpty(t, resp.Error)
	})

	t.Run("rootless-required error sets ErrorCodeRootlessRequired", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		runner := &fakeRunner{err: buildkit.ErrRootlessRequired}
		e := &Engine{Runner: runner}

		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandBuild, RepoRoot: repoDir})
		assert.Equal(t, ErrorCodeRootlessRequired, resp.ErrorCode)
	})

	t.Run("other build error sets ErrorCodeBuildFailed", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, workspace.CopyDir("../../testdata/simple", repoDir))

		runner := &fakeRunner{err: assert.AnError}
		e := &Engine{Runner: runner}

		resp := e.Execute(context.Background(), BuildRequest{APIVersion: APIVersion, Command: CommandBuild, RepoRoot: repoDir})
		assert.Equal(t, ErrorCodeBuildFailed, resp.ErrorCode)
	})
}
