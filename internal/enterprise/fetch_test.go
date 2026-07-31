// SPDX-License-Identifier: MIT
package enterprise_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/builder/internal/enterprise"
)

// buildZip returns a zip archive with one entry per name -> content pair,
// wrapped in a single top-level folder the way GitHub's zipball API does
// (the github provider always strips exactly one level).
func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create("top/" + name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestFetch_SecondCallForSameRef_IsCacheHit(t *testing.T) {
	var hits int32
	zipData := buildZip(t, map[string]string{"f.txt": "content"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write(zipData)
	}))
	defer srv.Close()

	oldAPIBase := enterprise.APIBaseURL
	enterprise.APIBaseURL = srv.URL
	t.Cleanup(func() { enterprise.APIBaseURL = oldAPIBase })

	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	dir1, cleanup1, err := enterprise.Fetch("sha1", "tok")
	require.NoError(t, err)
	defer cleanup1()

	dir2, cleanup2, err := enterprise.Fetch("sha1", "tok")
	require.NoError(t, err)
	defer cleanup2()

	assert.Equal(t, dir1, dir2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))

	got, err := os.ReadFile(filepath.Join(dir2, "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(got))
}

func TestFetch_DifferentRefs_BothFetchedIndependently(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		ref := filepath.Base(r.URL.Path)
		w.Write(buildZip(t, map[string]string{"f.txt": ref}))
	}))
	defer srv.Close()

	oldAPIBase := enterprise.APIBaseURL
	enterprise.APIBaseURL = srv.URL
	t.Cleanup(func() { enterprise.APIBaseURL = oldAPIBase })

	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	dir1, cleanup1, err := enterprise.Fetch("sha1", "tok")
	require.NoError(t, err)
	defer cleanup1()

	dir2, cleanup2, err := enterprise.Fetch("sha2", "tok")
	require.NoError(t, err)
	defer cleanup2()

	assert.NotEqual(t, dir1, dir2)
	assert.Equal(t, int32(2), atomic.LoadInt32(&hits))
}

func TestFetch_CacheDirUnavailable_FallsBackUncached(t *testing.T) {
	zipData := buildZip(t, map[string]string{"f.txt": "content"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipData)
	}))
	defer srv.Close()

	oldAPIBase := enterprise.APIBaseURL
	enterprise.APIBaseURL = srv.URL
	t.Cleanup(func() { enterprise.APIBaseURL = oldAPIBase })

	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	dir, cleanup, err := enterprise.Fetch("sha1", "tok")
	require.NoError(t, err)
	defer cleanup()

	got, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(got))
}

func TestFetch_ConcurrentSameRef_ConvergesWithoutCorruption(t *testing.T) {
	zipData := buildZip(t, map[string]string{"f.txt": "content"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipData)
	}))
	defer srv.Close()

	oldAPIBase := enterprise.APIBaseURL
	enterprise.APIBaseURL = srv.URL
	t.Cleanup(func() { enterprise.APIBaseURL = oldAPIBase })

	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	const n = 8
	var wg sync.WaitGroup
	dirs := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			dir, cleanup, err := enterprise.Fetch("sha-concurrent", "tok")
			dirs[i] = dir
			errs[i] = err
			if cleanup != nil {
				cleanup()
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d", i)
	}
	for i, dir := range dirs {
		got, err := os.ReadFile(filepath.Join(dir, "f.txt"))
		require.NoError(t, err, "goroutine %d", i)
		assert.Equal(t, "content", string(got))
	}

	cacheRoot, err := enterprise.DefaultCacheDir()
	require.NoError(t, err)
	entries, err := os.ReadDir(cacheRoot)
	require.NoError(t, err)
	var refDirs int
	for _, e := range entries {
		if e.IsDir() && e.Name() == "sha-concurrent" {
			refDirs++
		}
	}
	assert.Equal(t, 1, refDirs, "exactly one sha-concurrent directory must exist under the cache root")
}
