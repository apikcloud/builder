// SPDX-License-Identifier: MIT
package buildkit

import (
	"fmt"
	"os"
)

// tlsEnv holds the client TLS material buildctl needs to complete an mTLS
// handshake with an externally managed buildkitd. Read directly from the
// environment (loadTLSEnv), the same ambient, per-deployment tier as
// BUILDKIT_HOST (daemon.go:76) — not threaded through BuildOptions,
// BuildRequest, or odoo-builder.yaml, since it varies with the environment
// builder runs in, not with any single build.
type tlsEnv struct {
	cert   string
	key    string
	cacert string
}

// loadTLSEnv reads BUILDKIT_TLS_CERT/_KEY/_CACERT (file paths) and
// validates them as a unit:
//   - none set: TLS is not in use, returns the zero tlsEnv and no error.
//   - some (but not all three) set: config error — buildctl's mTLS needs
//     all three together.
//   - all three set but hostIsExternal is false: config error — a
//     self-spawned buildkitd (ensureDaemon, daemon.go:80-115) never starts
//     with its own --tlscert/--tlskey/--tlscacert, so presenting a client
//     cert to it can never complete a handshake.
//
// hostIsExternal should be the same condition ensureDaemon itself uses
// (os.Getenv("BUILDKIT_HOST") != "").
func loadTLSEnv(hostIsExternal bool) (tlsEnv, error) {
	cert := os.Getenv("BUILDKIT_TLS_CERT")
	key := os.Getenv("BUILDKIT_TLS_KEY")
	cacert := os.Getenv("BUILDKIT_TLS_CACERT")

	if cert == "" && key == "" && cacert == "" {
		return tlsEnv{}, nil
	}
	if cert == "" || key == "" || cacert == "" {
		return tlsEnv{}, fmt.Errorf("buildkit: BUILDKIT_TLS_CERT, BUILDKIT_TLS_KEY, and BUILDKIT_TLS_CACERT must all be set together (cert=%q, key=%q, cacert=%q)", cert, key, cacert)
	}
	if !hostIsExternal {
		return tlsEnv{}, fmt.Errorf("buildkit: BUILDKIT_TLS_CERT/_KEY/_CACERT are set but BUILDKIT_HOST is not; a self-spawned buildkitd has no TLS listener to present a client cert to")
	}
	return tlsEnv{cert: cert, key: key, cacert: cacert}, nil
}

// tlsArgs returns the --tlscert/--tlskey/--tlscacert buildctl flags for t,
// or nil for the zero value (TLS not in use).
func tlsArgs(t tlsEnv) []string {
	if t == (tlsEnv{}) {
		return nil
	}
	return []string{"--tlscert", t.cert, "--tlskey", t.key, "--tlscacert", t.cacert}
}
