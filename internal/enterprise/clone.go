// SPDX-License-Identifier: MIT
// Package enterprise clones the Odoo Enterprise addons repository
// (README.md's Enterprise Support section) host-side, before BuildKit is
// ever invoked. It contains no BuildKit dependency: authentication happens
// entirely within this package's own git subprocess, via GIT_ASKPASS, so
// the credential never appears in a clone URL, a subprocess argument (and
// therefore never in a process listing), a odoo-builder.yaml field, or any log
// line this package emits.
package enterprise

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RepoURL is the fixed Odoo Enterprise addons repository.
const RepoURL = "https://github.com/odoo/enterprise.git"

// TokenEnvVar is the environment variable holding the HTTPS token used to
// authenticate against RepoURL. Never read from odoo-builder.yaml.
const TokenEnvVar = "ODOO_ENTERPRISE_TOKEN"

// askpassScript is a POSIX-sh helper git invokes (via GIT_ASKPASS) whenever
// it needs credentials. It answers from its own environment rather than
// from any argument, so the token never appears in a process listing.
const askpassScript = "#!/bin/sh\ncase \"$1\" in\n*sername*) echo \"x-access-token\" ;;\n*assword*) echo \"$" + TokenEnvVar + "\" ;;\nesac\n"

// Clone shallow-clones repoURL at the branch named version into a new
// temporary directory, authenticating with token. token is supplied to git
// solely via a GIT_ASKPASS helper reading its own environment — never via
// repoURL, a command-line argument, or this function's own error output.
// Fails fast, without spawning git, if version or token is empty. The
// returned cleanup removes both the clone and the short-lived askpass
// script; callers must always invoke it.
func Clone(repoURL, version, token string) (dir string, cleanup func(), err error) {
	if version == "" {
		return "", nil, fmt.Errorf("enterprise: base.version must be set to select the matching Enterprise branch")
	}
	if token == "" {
		return "", nil, fmt.Errorf("enterprise: %s is not set", TokenEnvVar)
	}

	tmp, err := os.MkdirTemp("", "odoo-enterprise-")
	if err != nil {
		return "", nil, fmt.Errorf("enterprise: creating temp dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(tmp) }

	askpass := filepath.Join(tmp, ".askpass.sh")
	if err := os.WriteFile(askpass, []byte(askpassScript), 0o700); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("enterprise: writing askpass helper: %w", err)
	}

	dest := filepath.Join(tmp, "src")
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", version, repoURL, dest)
	cmd.Env = append(os.Environ(),
		"GIT_ASKPASS="+askpass,
		"GIT_TERMINAL_PROMPT=0",
		TokenEnvVar+"="+token,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("enterprise: git clone: %w: %s", runErr, stderr.String())
	}

	return dest, cleanup, nil
}
