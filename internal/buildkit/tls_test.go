// SPDX-License-Identifier: MIT
package buildkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTLSEnv(t *testing.T) {
	t.Run("none set returns zero value", func(t *testing.T) {
		got, err := loadTLSEnv(true)
		require.NoError(t, err)
		assert.Equal(t, tlsEnv{}, got)
		assert.Nil(t, tlsArgs(got))
	})

	t.Run("all three set and external host returns populated value", func(t *testing.T) {
		t.Setenv("BUILDKIT_TLS_CERT", "/tls/cert.pem")
		t.Setenv("BUILDKIT_TLS_KEY", "/tls/key.pem")
		t.Setenv("BUILDKIT_TLS_CACERT", "/tls/ca.pem")

		got, err := loadTLSEnv(true)
		require.NoError(t, err)
		assert.Equal(t, tlsEnv{cert: "/tls/cert.pem", key: "/tls/key.pem", cacert: "/tls/ca.pem"}, got)
		assert.Equal(t, []string{"--tlscert", "/tls/cert.pem", "--tlskey", "/tls/key.pem", "--tlscacert", "/tls/ca.pem"}, tlsArgs(got))
	})

	t.Run("partial set errors", func(t *testing.T) {
		t.Setenv("BUILDKIT_TLS_CERT", "/tls/cert.pem")
		t.Setenv("BUILDKIT_TLS_KEY", "/tls/key.pem")

		_, err := loadTLSEnv(true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must all be set together")
	})

	t.Run("all three set without external host errors", func(t *testing.T) {
		t.Setenv("BUILDKIT_TLS_CERT", "/tls/cert.pem")
		t.Setenv("BUILDKIT_TLS_KEY", "/tls/key.pem")
		t.Setenv("BUILDKIT_TLS_CACERT", "/tls/ca.pem")

		_, err := loadTLSEnv(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "self-spawned buildkitd")
	})
}
