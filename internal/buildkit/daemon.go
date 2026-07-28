// SPDX-License-Identifier: MIT
package buildkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrRootlessRequired is returned (wrapped, use errors.Is) by ensureDaemon
// when the spawned buildkitd refuses to start because it requires root or
// a RootlessKit-managed user namespace. Callers (internal/cli) use this to
// decide whether to retry the build inside the distributable, root-by-
// default container image instead of failing outright.
var ErrRootlessRequired = errors.New("buildkit: buildkitd requires root or RootlessKit to start; retry inside the odoo-builder container image")

// rootlessMarkers are substrings buildkitd's own stderr contains when it
// refuses to start in a non-root, non-RootlessKit environment. Matched
// case-sensitively against buildkitd's actual, observed message; if a
// future buildkitd release rewords this, classifyDaemonError simply stops
// recognizing it and callers see today's existing generic error instead —
// a safe degradation, not a crash.
var rootlessMarkers = []string{
	"mapped root in a user namespace",
	"RootlessKit",
}

// buildkitdRunDirBase returns the directory ensureDaemon uses for one
// buildkitd invocation's --root/socket: ODOO_BUILDER_BUILDKITD_ROOT if
// set — already created and bind-mounted host-side by
// internal/launcher.invokeContainer's hostBuildkitdRootDir, specifically
// so it is NOT on the container's own overlay filesystem (see
// ensureDaemon's doc comment) — otherwise a fresh os.MkdirTemp (bare-host
// Engine Mode, or odoo-builder-engine invoked directly with no launcher
// involved at all, e.g. a Kubernetes Job).
func buildkitdRunDirBase() (string, error) {
	if dir := os.Getenv("ODOO_BUILDER_BUILDKITD_ROOT"); dir != "" {
		return dir, nil
	}
	return os.MkdirTemp("", "odoo-builder-buildkitd-")
}

// ensureDaemon returns a BUILDKIT_HOST-style address reachable for the
// duration of one build, plus a cleanup func to call afterwards.
//
// If BUILDKIT_HOST is already set, it is returned as-is and cleanup is a
// no-op: an externally managed daemon (e.g. a sidecar, or one running
// inside the future distributable image) is assumed. Otherwise a
// buildkitd process is spawned on a fresh temporary unix socket and torn
// down by cleanup once the build completes.
//
// Its run directory comes from buildkitdRunDirBase, which defaults to a
// plain os.MkdirTemp — fine for invokeLocal's bare-host Engine Mode, where
// nothing here is nested inside another container. Inside
// invokeContainer's containerized Engine Mode, that default directory
// sits on the container's own overlay-filesystem writable layer, and
// BuildKit's snapshotter auto-detection refuses to nest
// overlayfs-on-overlayfs — it silently falls back to the "native"
// snapshotter (real per-file copies instead of copy-on-write mounts),
// dramatically slower extracting large/many-file layers.
// ODOO_BUILDER_BUILDKITD_ROOT (set by invokeContainer, bind-mounting a
// host directory to it) routes around this: a bind-mounted directory is
// never itself an overlayfs mount, so BuildKit picks the real "overlayfs"
// snapshotter instead. Confirmed directly against this image's own
// buildkitd: --root under the container's default /tmp logs "auto
// snapshotter: using native"; --root under a bind-mounted directory logs
// "auto snapshotter: using overlayfs".
func ensureDaemon(ctx context.Context) (addr string, cleanup func(), err error) {
	if h := os.Getenv("BUILDKIT_HOST"); h != "" {
		return h, func() {}, nil
	}

	runDir, err := buildkitdRunDirBase()
	if err != nil {
		return "", nil, fmt.Errorf("buildkit: resolving buildkitd run dir: %w", err)
	}

	sockPath := filepath.Join(runDir, "buildkitd.sock")
	addr = "unix://" + sockPath

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "buildkitd",
		"--addr", addr,
		"--root", filepath.Join(runDir, "root"),
	)
	cmd.Stdout = io.MultiWriter(os.Stderr, &stderr)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	if err := cmd.Start(); err != nil {
		os.RemoveAll(runDir)
		return "", nil, fmt.Errorf("buildkit: starting buildkitd: %w", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if waitErr := waitForSocketOrExit(ctx, sockPath, exited, 10*time.Second); waitErr != nil {
		_ = cmd.Process.Kill()
		os.RemoveAll(runDir)
		return "", nil, classifyDaemonError(waitErr, stderr.String())
	}

	cleanup = func() {
		_ = cmd.Process.Kill()
		<-exited
		os.RemoveAll(runDir)
	}
	return addr, cleanup, nil
}

// waitForSocketOrExit polls for path to appear, but returns immediately
// (rather than waiting out the full timeout) if the process exits first —
// exited receives cmd.Wait()'s result exactly once.
func waitForSocketOrExit(ctx context.Context, path string, exited <-chan error, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		select {
		case waitErr := <-exited:
			if waitErr == nil {
				waitErr = fmt.Errorf("buildkitd exited before its socket was ready")
			}
			return waitErr
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
			if time.Now().After(deadline) {
				return fmt.Errorf("buildkit: buildkitd socket %s not ready after %s", path, timeout)
			}
		}
	}
}

// classifyDaemonError wraps waitErr with ErrRootlessRequired when
// stderrText matches buildkitd's rootless-mode refusal; otherwise it
// returns today's existing generic error shape, stderrText included for
// diagnosis.
func classifyDaemonError(waitErr error, stderrText string) error {
	for _, marker := range rootlessMarkers {
		if strings.Contains(stderrText, marker) {
			return fmt.Errorf("%w: %s", ErrRootlessRequired, strings.TrimSpace(stderrText))
		}
	}
	return fmt.Errorf("buildkit: buildkitd: %w: %s", waitErr, strings.TrimSpace(stderrText))
}
