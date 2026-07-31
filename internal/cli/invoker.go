// SPDX-License-Identifier: MIT
package cli

import (
	"context"
	"io"

	"github.com/apikcloud/builder/internal/engine"
	"github.com/apikcloud/builder/internal/launcher"
)

// invokeEngine is a package-level var so tests can substitute a fake
// BuildResponse without a real odoo-builder-engine binary, docker/podman,
// or buildctl/buildkitd on PATH.
var invokeEngine = func(ctx context.Context, mode launcher.Mode, req engine.BuildRequest, stderr io.Writer) (engine.BuildResponse, error) {
	return launcher.Invoke(ctx, mode, req, stderr)
}
