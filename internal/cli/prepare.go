// SPDX-License-Identifier: MIT
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/apikcloud/odoo-builder/internal/engine"
)

func newPrepareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare the deterministic .build/ context",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			mode, err := resolveModeFlag(cmd)
			if err != nil {
				return err
			}

			req := engine.BuildRequest{APIVersion: engine.APIVersion, Command: engine.CommandPrepare, RepoRoot: cwd}
			resp, err := invokeEngine(cmd.Context(), mode, req, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "prepared build context at %s (%d addon(s))\n", resp.BuildDir, resp.AddonCount)
			return nil
		},
	}
	cmd.Flags().String("mode", "", modeFlagUsage)
	return cmd
}
