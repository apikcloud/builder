// SPDX-License-Identifier: MIT
// Command fakeengine is a test-only stand-in for odoo-builder-engine, used
// by internal/launcher/invoke_test.go to exercise Invoke's local-exec path
// without a real engine binary. It reads a BuildRequest JSON from stdin and
// selects a canned BuildResponse based on RepoRoot, a signal the real
// engine never gives special meaning to.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type request struct {
	RepoRoot string `json:"repoRoot"`
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, "fakeengine: decoding request:", err)
		os.Exit(1)
	}

	switch req.RepoRoot {
	case "ROOTLESS":
		fmt.Println(`{"apiVersion":"v1","error":"rootless needed","errorCode":"rootless_required"}`)
		os.Exit(1)
	case "FAIL":
		fmt.Println(`{"apiVersion":"v1","error":"boom","errorCode":"build_failed"}`)
		os.Exit(1)
	case "BADOUTPUT":
		fmt.Println("not json")
		os.Exit(1)
	case "GARBAGE_EXIT0":
		fmt.Println("pulling image progress line")
		os.Exit(0)
	default:
		fmt.Println(`{"apiVersion":"v1","buildDir":"/fake/build","addonCount":3}`)
	}
}
