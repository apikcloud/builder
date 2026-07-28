// SPDX-License-Identifier: MIT
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/apikcloud/odoo-builder/internal/engine"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the repository layout and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			mode, err := resolveModeFlag(cmd)
			if err != nil {
				return err
			}

			req := engine.BuildRequest{APIVersion: engine.APIVersion, Command: engine.CommandValidate, RepoRoot: cwd}
			resp, err := invokeEngine(cmd.Context(), mode, req, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			if len(resp.ValidationErrors) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "OK")
				return nil
			}
			for _, e := range resp.ValidationErrors {
				fmt.Fprintln(cmd.ErrOrStderr(), e)
			}
			return fmt.Errorf("validate: %d error(s) found", len(resp.ValidationErrors))
		},
	}
	cmd.Flags().String("mode", "", modeFlagUsage)
	return cmd
}
