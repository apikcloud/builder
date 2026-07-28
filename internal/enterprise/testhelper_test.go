// SPDX-License-Identifier: MIT
package enterprise_test

import (
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return string(out)
}

// gitHTTPBackend locates the git-http-backend CGI executable that ships
// alongside the git binary, skipping the test if it is not installed.
func gitHTTPBackend(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Skip("git --exec-path failed, cannot locate git-http-backend")
	}
	backend := filepath.Join(strings.TrimSpace(string(out)), "git-http-backend")
	if _, err := os.Stat(backend); err != nil {
		t.Skip("git-http-backend not found next to git, skipping smart-HTTP fixture test")
	}
	return backend
}

// startAuthedGitServer creates a bare git repo whose default branch is
// named branch and contains one file (name -> content), serves it over
// smart-HTTP git (via git-http-backend, which — unlike the dumb protocol —
// supports the shallow (--depth) clones enterprise.Clone always requests)
// with Basic Auth (username "x-access-token", password token), and returns
// the server's clone URL. The server and its backing temp directories are
// cleaned up via t.Cleanup.
func startAuthedGitServer(t *testing.T, branch, name, content, token string) (repoURL string) {
	t.Helper()
	requireGit(t)
	backend := gitHTTPBackend(t)

	work := t.TempDir()
	runGit(t, work, "init", "-q", "-b", branch)
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "Test")
	runGit(t, work, "config", "commit.gpgsign", "false")
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(work, name)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(work, name), []byte(content), 0o644))
	runGit(t, work, "add", "-A")
	runGit(t, work, "commit", "-q", "-m", "fixture")

	root := t.TempDir()
	bare := filepath.Join(root, "repo.git")
	runGit(t, "", "clone", "-q", "--bare", work, bare)
	runGit(t, bare, "config", "http.receivepack", "false")

	cgiHandler := &cgi.Handler{
		Path: backend,
		Env:  []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "x-access-token" || pass != token {
			w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		cgiHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	return srv.URL + "/repo.git"
}
