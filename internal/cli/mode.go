// SPDX-License-Identifier: MIT
package cli

import (
	"github.com/spf13/cobra"

	"github.com/apikcloud/builder/internal/launcher"
)

const modeFlagUsage = `override engine invocation: "engine" (force the local odoo-builder-engine binary, no container, no rootless retry) or "launcher" (force the container, even if the engine binary/buildctl/buildkitd are on PATH); default "auto" probes for what's on PATH, or set ODOO_BUILDER_MODE`

func resolveModeFlag(cmd *cobra.Command) (launcher.Mode, error) {
	modeFlag, err := cmd.Flags().GetString("mode")
	if err != nil {
		return "", err
	}
	return launcher.ResolveMode(modeFlag)
}
