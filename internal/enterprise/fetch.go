// SPDX-License-Identifier: MIT
// Package enterprise resolves and fetches the Odoo Enterprise addons
// repository (README.md's Enterprise Support section) host-side, before
// BuildKit is ever invoked, via the workspace-provider library's github
// source provider (GitHub zipball fetch — no git history transfer, no
// system git dependency).
package enterprise

import (
	"context"
	"fmt"

	workspace "github.com/apikcloud/workspace-provider"
	_ "github.com/apikcloud/workspace-provider/providers/github"
)

// TokenEnvVar is the environment variable holding the token used to
// authenticate against the Enterprise repository. Never read from
// odoo-builder.yaml.
const TokenEnvVar = "ODOO_ENTERPRISE_TOKEN"

func enterpriseSource(ref string) workspace.Source {
	return workspace.Source{
		Type:        "github",
		Location:    repoOwner + "/" + repoName,
		Destination: ".",
		Credentials: "enterprise",
		Options:     map[string]string{"ref": ref, "api_base_url": APIBaseURL},
	}
}

func enterpriseResolver(token string) workspace.StaticResolver {
	return workspace.StaticResolver{"enterprise": tokenCredential(token)}
}

// Fetch downloads the Enterprise repository's tree at ref (a branch name,
// tag, or full/abbreviated commit SHA — GitHub's zipball endpoint accepts
// all three interchangeably) via workspace.Prepare and the github provider,
// extracting it into a new temporary directory. The returned cleanup
// removes that directory; callers must always invoke it.
func Fetch(ref, token string) (dir string, cleanup func(), err error) {
	if ref == "" {
		return "", nil, fmt.Errorf("enterprise: a branch, tag, or commit is required to fetch Enterprise addons")
	}
	if token == "" {
		return "", nil, fmt.Errorf("enterprise: %s is not set", TokenEnvVar)
	}

	ws, err := workspace.Prepare(context.Background(), workspace.WorkspaceRequest{
		Temporary: true,
		Spec:      workspace.WorkspaceSpec{Sources: []workspace.Source{enterpriseSource(ref)}},
	}, enterpriseResolver(token))
	if err != nil {
		return "", nil, fmt.Errorf("enterprise: fetching %s: %w", ref, err)
	}
	return ws.Root, func() { _ = ws.Cleanup() }, nil
}
