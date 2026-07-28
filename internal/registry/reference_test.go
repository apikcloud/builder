// SPDX-License-Identifier: MIT
package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/registry"
)

func TestReference(t *testing.T) {
	t.Run("empty tag defaults to latest", func(t *testing.T) {
		assert.Equal(t, "app:latest", registry.Reference("app", ""))
	})

	t.Run("explicit tag preserved", func(t *testing.T) {
		assert.Equal(t, "app:v1", registry.Reference("app", "v1"))
	})
}

func TestValidate(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		err := registry.Validate("")
		require.Error(t, err)
	})

	t.Run("whitespace in name", func(t *testing.T) {
		err := registry.Validate("bad name")
		require.Error(t, err)
	})

	t.Run("embedded digest", func(t *testing.T) {
		err := registry.Validate("app@sha256:abcd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "digest")
	})

	t.Run("embedded tag", func(t *testing.T) {
		err := registry.Validate("app:v1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tag")
	})

	t.Run("plain registry path is valid", func(t *testing.T) {
		assert.NoError(t, registry.Validate("registry.example.com/app"))
	})

	t.Run("port in host is valid", func(t *testing.T) {
		assert.NoError(t, registry.Validate("localhost:5000/app"))
	})
}
