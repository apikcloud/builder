// SPDX-License-Identifier: MIT
// Command odoo-builder-engine executes a single BuildRequest read from
// stdin (or ODOO_BUILDER_REQUEST_FILE) and writes its BuildResponse to
// stdout (or ODOO_BUILDER_RESPONSE_FILE). It is invoked by odoo-builder
// (the launcher), locally or inside the distributable container image, and
// can also be invoked directly by callers that skip the launcher entirely
// (Kubernetes Jobs, CI steps).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/apikcloud/odoo-builder/internal/engine"
	"github.com/apikcloud/odoo-builder/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.String())
		return
	}

	// SIGINT/SIGTERM here — whether from the terminal's own foreground
	// process-group delivery (invokeLocal) or forwarded by entrypoint.sh
	// (invokeContainer) — cancels ctx, which internal/engine.Execute
	// threads down to buildkit.Build's exec.CommandContext calls for
	// buildctl/buildkitd. That's what makes the buildkitd cleanup() in
	// buildkit.execRunner.Build actually run instead of buildkitd being
	// orphaned mid-build (the "random printouts" the caller keeps seeing
	// after Ctrl+C, until it finishes on its own).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	req, err := readRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "odoo-builder-engine: reading request: %v\n", err)
		os.Exit(1)
	}

	resp := engine.New().Execute(ctx, req)

	if err := writeResponse(resp); err != nil {
		fmt.Fprintf(os.Stderr, "odoo-builder-engine: writing response: %v\n", err)
		os.Exit(1)
	}

	if resp.Error != "" {
		os.Exit(1)
	}
}

// readRequest reads a BuildRequest from ODOO_BUILDER_REQUEST_FILE if set,
// otherwise from stdin.
func readRequest() (engine.BuildRequest, error) {
	var r io.Reader = os.Stdin
	if path := os.Getenv("ODOO_BUILDER_REQUEST_FILE"); path != "" {
		f, err := os.Open(path)
		if err != nil {
			return engine.BuildRequest{}, fmt.Errorf("opening %s: %w", path, err)
		}
		defer f.Close()
		r = f
	}
	var req engine.BuildRequest
	err := json.NewDecoder(r).Decode(&req)
	return req, err
}

// writeResponse writes resp as JSON to ODOO_BUILDER_RESPONSE_FILE if set,
// otherwise to stdout.
func writeResponse(resp engine.BuildResponse) error {
	var w io.Writer = os.Stdout
	if path := os.Getenv("ODOO_BUILDER_RESPONSE_FILE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("creating %s: %w", path, err)
		}
		defer f.Close()
		w = f
	}
	return json.NewEncoder(w).Encode(resp)
}
