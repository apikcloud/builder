// SPDX-License-Identifier: MIT
package enterprise

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Download fetches the Enterprise repository's tree at commit (a full or
// abbreviated SHA) via GitHub's zipball API and extracts it into a new
// temporary directory, stripping the single top-level folder GitHub always
// wraps a zipball's contents in. Unlike Clone, this never runs git and
// fetches exactly one commit's tree rather than any repository history —
// what pins a build to an exact, reproducible Enterprise commit (see
// ResolveCommit) actually downloads. The returned cleanup removes the
// temporary directory; callers must always invoke it.
func Download(commit, token string) (dir string, cleanup func(), err error) {
	if commit == "" {
		return "", nil, fmt.Errorf("enterprise: commit must be set to download a pinned Enterprise tree")
	}
	if token == "" {
		return "", nil, fmt.Errorf("enterprise: %s is not set", TokenEnvVar)
	}

	tmp, err := os.MkdirTemp("", "odoo-enterprise-")
	if err != nil {
		return "", nil, fmt.Errorf("enterprise: creating temp dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(tmp) }

	url := fmt.Sprintf("%s/repos/%s/%s/zipball/%s", APIBaseURL, apiOwner, apiRepo, commit)
	req, reqErr := http.NewRequest(http.MethodGet, url, nil)
	if reqErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("enterprise: building zipball request: %w", reqErr)
	}
	req.Header.Set("Authorization", "token "+token)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, doErr := client.Do(req)
	if doErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("enterprise: downloading commit %s: %w", commit, doErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cleanup()
		return "", nil, fmt.Errorf("enterprise: downloading commit %s failed: %s", commit, resp.Status)
	}

	zipPath := filepath.Join(tmp, "enterprise.zip")
	zipFile, createErr := os.Create(zipPath)
	if createErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("enterprise: writing zipball: %w", createErr)
	}
	if _, copyErr := io.Copy(zipFile, resp.Body); copyErr != nil {
		zipFile.Close()
		cleanup()
		return "", nil, fmt.Errorf("enterprise: writing zipball: %w", copyErr)
	}
	zipFile.Close()

	dest := filepath.Join(tmp, "src")
	if extractErr := extractZip(zipPath, dest); extractErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("enterprise: extracting zipball: %w", extractErr)
	}

	return dest, cleanup, nil
}

// extractZip extracts src into dest, stripping the single top-level
// directory GitHub's zipball API always wraps its contents in (e.g.
// odoo-enterprise-<shortsha>/...).
func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	destRoot := filepath.Clean(dest) + string(os.PathSeparator)

	for _, f := range r.File {
		rel := stripTopLevel(f.Name)
		if rel == "" {
			continue
		}

		target := filepath.Join(dest, rel)
		if !strings.HasPrefix(target+string(os.PathSeparator), destRoot) && target != filepath.Clean(dest) {
			return fmt.Errorf("zip entry escapes destination: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// stripTopLevel drops name's first path segment (the
// "<owner>-<repo>-<shortsha>/" folder GitHub's zipball API wraps every
// entry in), returning "" for the top-level folder entry itself.
func stripTopLevel(name string) string {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) < 2 || parts[1] == "" {
		return ""
	}
	return parts[1]
}
