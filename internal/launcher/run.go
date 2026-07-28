package launcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Run launches ImageRef() via runtime, mounting workspace at /workspace,
// forwarding ForwardedEnv() plus HOST_UID/HOST_GID (see hostIDEnv), and
// passing args to the image's entrypoint. When the host cache directory
// (os.UserCacheDir()/odoo-builder) is resolvable and creatable, it is also
// bind-mounted in (see BuildArgs' hostCacheDir) so the containerized
// builder's local BuildKit cache (builder.yaml's cache.enabled with no
// image.name) persists on the host across container-launcher runs instead
// of being discarded with the container. stdout/stderr/stdin are streamed
// through as-is. Returns the container's error (including a non-zero exit
// via *exec.ExitError) or nil on success.
func Run(ctx context.Context, runtime Runtime, workspace string, args []string, stdout, stderr io.Writer) error {
	dockerConfigDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		if fi, statErr := os.Stat(filepath.Join(home, ".docker")); statErr == nil && fi.IsDir() {
			dockerConfigDir = filepath.Join(home, ".docker")
		}
	}

	env := append(ForwardedEnv(), hostIDEnv(os.Getuid(), os.Getgid())...)

	hostCacheDir := ""
	if base, err := os.UserCacheDir(); err == nil {
		dir := filepath.Join(base, "odoo-builder")
		if os.MkdirAll(dir, 0o755) == nil {
			hostCacheDir = dir
			env = append(env, "XDG_CACHE_HOME=/host-cache")
		}
	}

	cmdArgs := BuildArgs(ImageRef(), workspace, dockerConfigDir, hostCacheDir, env, args)

	cmd := exec.CommandContext(ctx, string(runtime), cmdArgs...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
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
