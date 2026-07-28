package launcher

import (
	"context"
	"io"
	"os/exec"
)

// Load imports archivePath — a docker-archive tarball produced by an
// Engine-Mode build with buildkit.BuildOptions.OutputType == "docker" —
// into runtime's local image store via "docker load"/"podman load". The
// archive's embedded reference (buildctl's type=docker name= parameter) is
// applied automatically by load, exactly as with "docker save"/"docker
// load" — no separate tag step is needed.
//
// Used only by `builder build --load` (internal/cli/build.go) against the
// host's own Docker/Podman — never from inside the distributable builder
// image, which has no docker CLI and no docker socket mounted (see
// BuildArgs and image/Dockerfile).
func Load(ctx context.Context, runtime Runtime, archivePath string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, string(runtime), "load", "-i", archivePath)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
