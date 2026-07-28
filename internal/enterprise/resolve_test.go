// SPDX-License-Identifier: MIT
package enterprise_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/enterprise"
)

// withFakeGitHubAPI points enterprise.APIBaseURL at a local test server
// serving handler for the duration of the test, restoring the real GitHub
// API URL afterward.
func withFakeGitHubAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	old := enterprise.APIBaseURL
	enterprise.APIBaseURL = srv.URL
	t.Cleanup(func() { enterprise.APIBaseURL = old })
}

func TestResolveCommit_ReturnsNewestCommitSHA(t *testing.T) {
	withFakeGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/odoo/enterprise/commits", r.URL.Path)
		assert.Equal(t, "18.0", r.URL.Query().Get("sha"))
		assert.Equal(t, "2025-06-11T23:59:59Z", r.URL.Query().Get("until"))
		assert.Equal(t, "token correct-token", r.Header.Get("Authorization"))

		fmt.Fprint(w, `[{"sha": "abc123"}, {"sha": "older456"}]`)
	})

	sha, err := enterprise.ResolveCommit("18.0", "20250611", "correct-token")
	require.NoError(t, err)
	assert.Equal(t, "abc123", sha)
}

func TestResolveCommit_NoCommitsFound_ReturnsError(t *testing.T) {
	withFakeGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})

	_, err := enterprise.ResolveCommit("18.0", "20250611", "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no commit found")
}

func TestResolveCommit_APIError_ReturnsError(t *testing.T) {
	withFakeGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message": "Bad credentials"}`)
	})

	_, err := enterprise.ResolveCommit("18.0", "20250611", "wrong-token")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "wrong-token", "the real credential must never appear in an error message")
}

func TestResolveCommit_EmptyBranch_ReturnsError(t *testing.T) {
	_, err := enterprise.ResolveCommit("", "20250611", "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base.version")
}

func TestResolveCommit_MalformedDate_ReturnsError(t *testing.T) {
	_, err := enterprise.ResolveCommit("18.0", "2025-06-11", "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "YYYYMMDD")
}

func TestResolveCommit_EmptyToken_ReturnsError(t *testing.T) {
	_, err := enterprise.ResolveCommit("18.0", "20250611", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), enterprise.TokenEnvVar)
}
