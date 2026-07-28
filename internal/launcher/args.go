// SPDX-License-Identifier: MIT
package launcher

import "os"

// forwardedEnvVars are the only host environment variables passed into
// the container — just enough for a build to succeed, not a blanket
// os.Environ() passthrough (which would leak unrelated host secrets).
var forwardedEnvVars = []string{"ODOO_ENTERPRISE_TOKEN", "BUILDKIT_HOST", "DOCKER_CONFIG"}

// ForwardedEnv returns "KEY=VALUE" pairs for every forwardedEnvVars entry
// currently set in the host environment.
func ForwardedEnv() []string {
	var out []string
	for _, k := range forwardedEnvVars {
		if v, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// BuildArgs returns the argument list (everything after the runtime
// binary name itself) for launching image with workspace mounted
// read-write at /workspace (also the container's working directory), run
// --privileged (buildkitd's OCI worker needs root/equivalent
// capabilities — see image/Dockerfile, which runs as root for exactly
// this reason) and -i (keeps stdin open/attached — without it, "docker
// run" closes the container's stdin immediately, and the piped
// BuildRequest JSON internal/launcher.invokeContainer writes never
// reaches odoo-builder-engine, which then fails with EOF reading its
// request). If dockerConfigDir is non-empty, it is mounted read-only
// at /root/.docker. If hostCacheDir is non-empty, it is mounted
// read-write at /host-cache/odoo-builder — paired with an
// XDG_CACHE_HOME=/host-cache entry in env (added by Run) so the
// containerized builder's own buildkit.DefaultCacheDir() call resolves
// onto this host-backed directory instead of the container's own,
// discarded-on-exit filesystem. env entries (as returned by
// ForwardedEnv/hostIDEnv) become "-e" flags. args are appended last,
// forwarded to the image's entrypoint.
//
// Pure: performs no I/O and has no dependency on the real filesystem or
// PATH, so it is exhaustively unit-testable without docker/podman
// installed.
func BuildArgs(image, workspace, dockerConfigDir, hostCacheDir string, env, args []string) []string {
	out := []string{"run", "--rm", "--privileged", "-i",
		"-v", workspace + ":/workspace",
		"-w", "/workspace",
	}
	if dockerConfigDir != "" {
		out = append(out, "-v", dockerConfigDir+":/root/.docker:ro")
	}
	if hostCacheDir != "" {
		out = append(out, "-v", hostCacheDir+":/host-cache/odoo-builder")
	}
	for _, e := range env {
		out = append(out, "-e", e)
	}
	out = append(out, image)
	out = append(out, args...)
	return out
}
