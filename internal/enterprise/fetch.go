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
	"os"

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

// Fetch downloads the Enterprise repository's tree at ref (an immutable
// commit SHA — see below).
//
// Fetch caches its result on disk under DefaultCacheDir(), keyed by ref: a
// second Fetch call for the same ref — in this process or a later one —
// returns the cached tree with no network access at all. ref MUST be an
// immutable identifier (a full or abbreviated commit SHA): the cache is
// never invalidated or evicted, so passing a mutable branch or tag name
// would serve stale content forever. Callers resolve a branch name to its
// current HEAD commit before calling Fetch for exactly this reason (see
// ResolveCommit, ResolveBranchHead).
//
// If the cache directory itself is unavailable (e.g. no writable user
// cache directory), Fetch logs a warning to stderr and falls back to a
// one-shot temporary fetch, so a cache problem never fails an otherwise-
// working build.
//
// The returned cleanup is a no-op whenever the result came from (or was
// published to) the cache — that directory persists across calls and
// processes — and only removes an actual temporary directory on the
// uncached fallback path.
func Fetch(ref, token string) (dir string, cleanup func(), err error) {
	if ref == "" {
		return "", nil, fmt.Errorf("enterprise: a branch, tag, or commit is required to fetch Enterprise addons")
	}
	if token == "" {
		return "", nil, fmt.Errorf("enterprise: %s is not set", TokenEnvVar)
	}

	cacheRoot, cacheErr := DefaultCacheDir()
	if cacheErr != nil {
		fmt.Fprintf(os.Stderr, "odoo-builder: enterprise: cache unavailable (%v), fetching %s without cache\n", cacheErr, ref)
		return fetchUncached(ref, token)
	}

	dir, err = cachedFetch(cacheRoot, ref, func(target string) error {
		_, prepErr := workspace.Prepare(context.Background(), workspace.WorkspaceRequest{
			Root:      target,
			Temporary: false,
			Spec:      workspace.WorkspaceSpec{Sources: []workspace.Source{enterpriseSource(ref)}},
		}, enterpriseResolver(token))
		return prepErr
	})
	if err != nil {
		return "", nil, fmt.Errorf("enterprise: fetching %s: %w", ref, err)
	}
	return dir, func() {}, nil
}

func fetchUncached(ref, token string) (string, func(), error) {
	ws, err := workspace.Prepare(context.Background(), workspace.WorkspaceRequest{
		Temporary: true,
		Spec:      workspace.WorkspaceSpec{Sources: []workspace.Source{enterpriseSource(ref)}},
	}, enterpriseResolver(token))
	if err != nil {
		return "", nil, fmt.Errorf("enterprise: fetching %s: %w", ref, err)
	}
	return ws.Root, func() { _ = ws.Cleanup() }, nil
}
