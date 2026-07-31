// SPDX-License-Identifier: MIT
// Package dockerfile resolves the base image and renders the Dockerfile
// content for a build context.
package dockerfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/apikcloud/builder/internal/config"
)

var releaseDatePattern = regexp.MustCompile(`\d{8}`)

// ResolveBaseImage determines the FROM reference for repoRoot given base.
// If base.Version is set, returns "odoo:<version>" (or
// "odoo:<version>-<release>" if base.Release is also set). Otherwise it
// reads <repoRoot>/odoo_version.txt: a file expected to contain exactly
// one non-blank line holding a full image reference (must include a
// ":<tag>" and an 8-digit YYYYMMDD date substring somewhere in the tag),
// returned verbatim. Returns an error if neither source yields a usable
// reference, or if odoo_version.txt exists but is malformed.
func ResolveBaseImage(repoRoot string, base config.Base) (string, error) {
	if base.Version != "" {
		ref := "odoo:" + base.Version
		if base.Release != "" {
			ref += "-" + base.Release
		}
		return ref, nil
	}

	path := filepath.Join(repoRoot, "odoo_version.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("dockerfile: no base image resolvable: odoo-builder.yaml base.version is empty and %s does not exist", path)
		}
		return "", fmt.Errorf("dockerfile: reading %s: %w", path, err)
	}

	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	if len(lines) != 1 {
		return "", fmt.Errorf("dockerfile: %s: expected exactly one non-blank line with the base image reference, found %d", path, len(lines))
	}

	ref := lines[0]

	if !strings.Contains(ref, ":") {
		return "", fmt.Errorf("dockerfile: %s: image reference %q must include a tag", path, ref)
	}

	if !releaseDatePattern.MatchString(ref) {
		return "", fmt.Errorf("dockerfile: %s: image reference %q must contain an 8-digit release date (YYYYMMDD)", path, ref)
	}

	return ref, nil
}
