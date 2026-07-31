// SPDX-License-Identifier: MIT
package enterprise

import (
	"context"
	"fmt"

	workspace "github.com/apikcloud/workspace-provider/pkg/workspace"
	githubprovider "github.com/apikcloud/workspace-provider/providers/github"
)

// repoOwner and repoName are the Enterprise repository's GitHub coordinates,
// shared with fetch.go.
const (
	repoOwner = "odoo"
	repoName  = "enterprise"
)

// APIBaseURL is the GitHub REST/zipball API root used for both commit
// resolution and fetching. Overridable so tests can point it at a local
// fixture server instead of the real GitHub API; production code never
// reassigns it.
var APIBaseURL = "https://api.github.com"

// ResolveCommit returns the SHA of the newest commit on branch at or
// before date (YYYYMMDD) in the Enterprise repository, so a build can pin
// Enterprise addons to the same day as the community base image
// (base.release) instead of a branch's ever-moving tip.
func ResolveCommit(branch, date, token string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("enterprise: base.version must be set to resolve an Enterprise commit by date")
	}
	if len(date) != 8 {
		return "", fmt.Errorf("enterprise: date %q must be in YYYYMMDD format", date)
	}
	if token == "" {
		return "", fmt.Errorf("enterprise: %s is not set", TokenEnvVar)
	}
	sha, err := githubprovider.ResolveCommit(context.Background(), APIBaseURL, repoOwner, repoName, branch, date, tokenCredential(token), "token")
	if err != nil {
		return "", fmt.Errorf("enterprise: %w", err)
	}
	return sha, nil
}

// ResolveBranchHead returns the SHA of the newest commit on branch — its
// current HEAD. Used so a build with neither enterprise.commit nor a
// resolvable date (the branch-tip case) still resolves an immutable SHA
// before fetching, letting Fetch's on-disk cache (see cache.go) apply to
// that case too instead of only to explicitly pinned builds.
func ResolveBranchHead(branch, token string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("enterprise: base.version must be set to resolve the Enterprise branch tip")
	}
	if token == "" {
		return "", fmt.Errorf("enterprise: %s is not set", TokenEnvVar)
	}
	sha, err := githubprovider.ResolveCommit(context.Background(), APIBaseURL, repoOwner, repoName, branch, "", tokenCredential(token), "token")
	if err != nil {
		return "", fmt.Errorf("enterprise: %w", err)
	}
	return sha, nil
}

func tokenCredential(token string) workspace.Credential {
	return workspace.Credential{Kind: workspace.CredentialKindToken, Token: token}
}
