// SPDX-License-Identifier: MIT
package cli

import "github.com/apikcloud/odoo-builder/internal/engine"

// newEngine constructs the Engine backing CLI commands. It is a
// package-level var so tests can substitute a fake buildkit.Runner without
// touching real buildctl/buildkitd.
var newEngine = engine.New
