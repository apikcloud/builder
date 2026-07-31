// SPDX-License-Identifier: MIT
// Package enterprise resolves and fetches the Odoo Enterprise addons
// repository (README.md's Enterprise Support section) host-side, before
// BuildKit is ever invoked, via the workspace-provider library's archive
// source provider (GitHub zipball fetch — no git history transfer, no
// system git dependency).
package enterprise

import (
	"context"
	"fmt"

	workspace "github.com/apikcloud/workspace-provider"
	_ "github.com/apikcloud/workspace-provider/providers/archive"
)

// RepoURL's GitHub coordinates (apiOwner/apiRepo) live in resolve.go,
// shared with ResolveCommit.

// TokenEnvVar is the environment variable holding the token used to
// authenticate against the Enterprise repository. Never read from
// odoo-builder.yaml.
const TokenEnvVar = "ODOO_ENTERPRISE_TOKEN"

// ZipballURL returns the GitHub zipball API URL for ref (a branch name,
// tag, or commit SHA — GitHub's zipball endpoint accepts all three
// interchangeably).
func ZipballURL(ref string) string {
	return fmt.Sprintf("%s/repos/%s/%s/zipball/%s", APIBaseURL, apiOwner, apiRepo, ref)
}

// Fetch downloads the Enterprise repository's tree at ref (a branch name,
// tag, or full/abbreviated commit SHA) via workspace.Prepare and the
// archive provider, extracting it into a new temporary directory. The
// returned cleanup removes that directory; callers must always invoke it.
func Fetch(ref, token string) (dir string, cleanup func(), err error) {
	if ref == "" {
		return "", nil, fmt.Errorf("enterprise: a branch, tag, or commit is required to fetch Enterprise addons")
	}
	if token == "" {
		return "", nil, fmt.Errorf("enterprise: %s is not set", TokenEnvVar)
	}

	resolver := workspace.StaticResolver{
		"enterprise": workspace.Credential{Kind: workspace.CredentialKindToken, Token: token},
	}
	req := workspace.WorkspaceRequest{
		Temporary: true,
		Spec: workspace.WorkspaceSpec{
			Sources: []workspace.Source{{
				Type:        "archive",
				Location:    ZipballURL(ref),
				Destination: ".",
				Credentials: "enterprise",
				Options:     map[string]string{"strip_components": "1", "auth_header_scheme": "token"},
			}},
		},
	}

	ws, err := workspace.Prepare(context.Background(), req, resolver)
	if err != nil {
		return "", nil, fmt.Errorf("enterprise: fetching %s: %w", ref, err)
	}
	return ws.Root, func() { _ = ws.Cleanup() }, nil
}
