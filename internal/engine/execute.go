// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"errors"
	"fmt"

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
