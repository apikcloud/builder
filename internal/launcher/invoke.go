// SPDX-License-Identifier: MIT
package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/apikcloud/odoo-builder/internal/engine"
)

// gracefulCancel replaces a *exec.Cmd's default ctx-cancellation behavior
// (an immediate, unblockable SIGKILL) with os.Interrupt (SIGINT on Unix)
// plus a grace period. It matters specifically for invokeContainer:
// exec.CommandContext's default Cancel kills only the local docker/
// podman-run client, which does not stop the container running
// server-side — docker/podman's own sig-proxy forwards the signal into the
// container (where entrypoint.sh now forwards it again, to
// odoo-builder-engine) only if the client process actually receives it
// rather than being killed out from under it. The WaitDelay fallback still
// guarantees termination if the graceful path hangs.
func gracefulCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 10 * time.Second
}

// Invoke runs req against the engine — locally if mode/Needed() allow it,
// containerized otherwise — and returns its BuildResponse. stderr is
// streamed through live in both cases (build progress); stdout is
// captured and JSON-decoded, not streamed.
func Invoke(ctx context.Context, mode Mode, req engine.BuildRequest, stderr io.Writer) (engine.BuildResponse, error) {
	needsContainer := mode == ModeLauncher || (mode == ModeAuto && Needed(req.Command))

	if mode == ModeEngine && !engineAvailable() {
		return engine.BuildResponse{}, fmt.Errorf("launcher: --mode engine requires %s on PATH", EngineBinary)
	}

	if !needsContainer {
		resp, err := invokeLocal(ctx, req, stderr)
		// req.Load never retries into the container: the container's
		// resolved ImagePath/BuildDir are in-container paths (RepoRoot is
		// rewritten to /workspace below) and are never translated back to
		// host paths, so a subsequent `docker/podman load -i` against
		// resp.ImagePath on the host would fail. Surface the rootless
		// error directly instead — same as an explicit --mode engine.
		if err != nil && mode == ModeAuto && !req.Load && resp.ErrorCode == engine.ErrorCodeRootlessRequired {
			fmt.Fprintln(stderr, "builder: buildkitd can't run directly on this host (needs root/RootlessKit) — retrying inside the odoo-builder container image")
			return invokeContainer(ctx, req, stderr)
		}
		return resp, err
	}

	return invokeContainer(ctx, req, stderr)
}

func invokeLocal(ctx context.Context, req engine.BuildRequest, stderr io.Writer) (engine.BuildResponse, error) {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return engine.BuildResponse{}, err
	}

	cmd := exec.CommandContext(ctx, EngineBinary)
	cmd.Stdin = bytes.NewReader(reqJSON)
	cmd.Stderr = stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	gracefulCancel(cmd)

	runErr := cmd.Run() // non-zero exit (resp.Error set) is not itself fatal here

	var resp engine.BuildResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		if runErr != nil {
			return engine.BuildResponse{}, runErr
		}
		return engine.BuildResponse{}, fmt.Errorf("launcher: decoding engine response: %w (stdout: %s)", err, snippet(out.Bytes()))
	}
	if resp.Error != "" {
		// resp is returned alongside the error (not discarded) precisely so
		// callers — here, Invoke's auto-retry check above — can read
		// resp.ErrorCode without re-deriving it from err's text.
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

func invokeContainer(ctx context.Context, req engine.BuildRequest, stderr io.Writer) (engine.BuildResponse, error) {
	runtime, err := DetectRuntime()
	if err != nil {
		return engine.BuildResponse{}, err
	}

	// req.RepoRoot is the host's path to the repo (BuildArgs mounts it at
	// /workspace below); the engine running inside the container has no
	// visibility into the host filesystem, so the request sent over stdin
	// must reference the repo at its in-container mount point instead.
	containerReq := req
	containerReq.RepoRoot = "/workspace"

	reqJSON, err := json.Marshal(containerReq)
	if err != nil {
		return engine.BuildResponse{}, err
	}

	dockerConfigDir := dockerConfigDirIfPresent()
	env := append(ForwardedEnv(), hostIDEnv(os.Getuid(), os.Getgid())...)
	hostCacheDir, cacheEnv := hostCacheDirIfResolvable()
	env = append(env, cacheEnv...)

	args := BuildArgs(ImageRef(), req.RepoRoot, dockerConfigDir, hostCacheDir, env, nil)
	cmd := exec.CommandContext(ctx, string(runtime), args...)
	cmd.Stdin = bytes.NewReader(reqJSON)
	cmd.Stderr = stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	gracefulCancel(cmd)

	runErr := cmd.Run()

	var resp engine.BuildResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		if runErr != nil {
			return engine.BuildResponse{}, runErr
		}
		return engine.BuildResponse{}, fmt.Errorf("launcher: decoding engine response: %w (stdout: %s)", err, snippet(out.Bytes()))
	}
	if resp.Error != "" {
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

// snippet returns a bounded, single-line preview of b for embedding in error
// messages — the raw bytes are otherwise discarded once JSON-decoding them
// fails, leaving no way to see what a misbehaving engine/runtime actually
// wrote to stdout (e.g. pull/build progress text leaking in ahead of the
// JSON response).
func snippet(b []byte) string {
	const maxLen = 200
	s := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, string(b))
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// dockerConfigDirIfPresent returns the host's ~/.docker directory if it
// exists, otherwise "".
func dockerConfigDirIfPresent() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".docker")
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return dir
	}
	return ""
}

// hostCacheDirIfResolvable returns the host's persistent BuildKit
// local-cache directory (os.UserCacheDir()/odoo-builder) if it's resolvable
// and creatable, plus the XDG_CACHE_HOME=/host-cache env entry that makes
// the containerized builder's own buildkit.DefaultCacheDir() call resolve
// onto it. Returns ("", nil) if unresolvable.
func hostCacheDirIfResolvable() (string, []string) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", nil
	}
	dir := filepath.Join(base, "odoo-builder")
	if os.MkdirAll(dir, 0o755) != nil {
		return "", nil
	}
	return dir, []string{"XDG_CACHE_HOME=/host-cache"}
}

// hostIDEnv returns "HOST_UID=<uid>"/"HOST_GID=<gid>" entries for uid/gid
// values that are actually valid (os.Getuid()/os.Getgid() return -1 on
// platforms without the concept, e.g. Windows, which is silently skipped).
// image/entrypoint.sh uses these to chown build output in the mounted
// workspace back to the host user afterwards — the container itself must
// run as root for buildkitd's OCI worker (mount, overlayfs), which would
// otherwise leave every file it writes root-owned on the host.
func hostIDEnv(uid, gid int) []string {
	var out []string
	if uid >= 0 {
		out = append(out, fmt.Sprintf("HOST_UID=%d", uid))
	}
	if gid >= 0 {
		out = append(out, fmt.Sprintf("HOST_GID=%d", gid))
	}
	return out
}
