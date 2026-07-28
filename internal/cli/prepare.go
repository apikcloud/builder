// SPDX-License-Identifier: MIT
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/apikcloud/odoo-builder/internal/engine"
)

func newPrepareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prepare",
		Short: "Prepare the deterministic .build/ context",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			result, err := newEngine().Prepare(engine.BuildRequest{RepoRoot: cwd})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "prepared build context at %s (%d addon(s))\n", result.BuildDir, result.AddonCount)
			return nil
		},
	}
}
