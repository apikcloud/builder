// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/apikcloud/odoo-builder/internal/buildkit"
)

// Execute runs req.Command against e and returns a BuildResponse — the
// engine binary's sole entry point (see cmd/odoo-builder-engine). Never
// returns a Go error: every failure is captured in BuildResponse.Error/
// ErrorCode so the caller (possibly across a process/JSON boundary) can
// always read a well-formed response.
func (e *Engine) Execute(ctx context.Context, req BuildRequest) BuildResponse {
	resp := BuildResponse{APIVersion: APIVersion}

	if req.APIVersion != APIVersion {
		resp.Error = fmt.Sprintf("engine: unsupported apiVersion %q (engine supports %q)", req.APIVersion, APIVersion)
		resp.ErrorCode = ErrorCodeUnsupportedAPIVersion
		return resp
	}

	switch req.Command {
	case CommandValidate:
		errs := e.Validate(req)
		resp.ValidationErrors = make([]string, len(errs))
		for i, err := range errs {
			resp.ValidationErrors[i] = err.Error()
		}
		if len(errs) > 0 {
			resp.ErrorCode = ErrorCodeValidationFailed
		}
		return resp

	case CommandPrepare:
		result, err := e.Prepare(req)
		if err != nil {
			resp.Error = err.Error()
			resp.ErrorCode = ErrorCodePrepareFailed
			return resp
		}
		resp.BuildDir, resp.AddonCount = result.BuildDir, result.AddonCount
		return resp

	case CommandInspect:
		resolvedReq, err := e.Inspect(req)
		if err != nil {
			resp.Error = err.Error()
			resp.ErrorCode = ErrorCodePrepareFailed
			return resp
		}
		resp.Resolved = resolvedReq.Resolved
		return resp

	case CommandBuild:
		printBuildSummary(req.Normalize())
		result, err := e.Build(ctx, req)
		if err != nil {
			resp.Error = err.Error()
			switch {
			case errors.Is(err, buildkit.ErrRootlessRequired):
				// Detected here, in-process, via errors.Is — before the
				// error is ever stringified. Never re-derived from
				// message text on the other side of the JSON boundary.
				resp.ErrorCode = ErrorCodeRootlessRequired
			case errors.Is(err, errValidationFailed):
				resp.ErrorCode = ErrorCodeValidationFailed
			default:
				resp.ErrorCode = ErrorCodeBuildFailed
			}
			return resp
		}
		resp.BuildDir, resp.AddonCount = result.BuildDir, result.AddonCount
		resp.ImagePath, resp.ImageRef = result.ImagePath, result.ImageRef
		return resp

	default:
		resp.Error = fmt.Sprintf("engine: unknown command %q", req.Command)
		resp.ErrorCode = ErrorCodeUnknownCommand
		return resp
	}
}

// printBuildSummary writes a one-line-per-field summary of req to stderr
// right before Engine.Build starts — a human watching the build knows what
// was actually requested before BuildKit output (or Enterprise addon
// retrieval, see prepare.Prepare) starts streaming.
func printBuildSummary(req BuildRequest) {
	fmt.Fprintf(os.Stderr, "odoo-builder: starting build (repoRoot=%s, buildDir=%s", req.RepoRoot, req.BuildDir)
	if req.Load {
		fmt.Fprint(os.Stderr, ", load=true")
	}
	if req.Output.Type != "" {
		fmt.Fprintf(os.Stderr, ", outputType=%s", req.Output.Type)
	}
	if req.Output.Image != "" {
		fmt.Fprintf(os.Stderr, ", outputImage=%s", req.Output.Image)
	}
	fmt.Fprintln(os.Stderr, ")")
}
