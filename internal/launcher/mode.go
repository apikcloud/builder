package launcher

import (
	"fmt"
	"os"
)

// ModeEnvVar, when set, is ResolveMode's fallback when no --mode flag is
// given. Deliberately absent from forwardedEnvVars in args.go: forwarding
// it into the container would make a host-forced ModeLauncher try to
// re-launch a nested container from inside the one already running.
const ModeEnvVar = "ODOO_BUILDER_MODE"

// Mode selects how `builder build` decides between running BuildKit
// directly and running it inside the distributable container image.
type Mode string

const (
	// ModeAuto is the default: Needed()'s buildctl/buildkitd-on-PATH probe
	// decides, exactly as before this type existed.
	ModeAuto Mode = "auto"
	// ModeEngine forces the direct path unconditionally: no launcher, and
	// no ErrRootlessRequired auto-retry either — forcing engine mode means
	// stay local and surface the real error, not silently containerize.
	ModeEngine Mode = "engine"
	// ModeLauncher forces the container path unconditionally, even when
	// buildctl/buildkitd are present on the host — the escape hatch for
	// pinning the exact BuildKit version baked into the image regardless
	// of host drift, or for debugging the launcher path itself.
	ModeLauncher Mode = "launcher"
)

// ResolveMode returns the Mode to use: flagValue if non-empty, else
// ModeEnvVar's value if set, else ModeAuto. Returns an error naming the bad
// value if either is set to something other than auto/engine/launcher.
func ResolveMode(flagValue string) (Mode, error) {
	v := flagValue
	if v == "" {
		v = os.Getenv(ModeEnvVar)
	}
	if v == "" {
		return ModeAuto, nil
	}

	switch Mode(v) {
	case ModeAuto, ModeEngine, ModeLauncher:
		return Mode(v), nil
	default:
		return "", fmt.Errorf("launcher: invalid mode %q (must be %q, %q, or %q)", v, ModeAuto, ModeEngine, ModeLauncher)
	}
}
