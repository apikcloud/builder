// SPDX-License-Identifier: MIT
package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/apikcloud/builder/internal/engine"
)

func newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Print the resolved configuration a build would use, without building",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			mode, err := resolveModeFlag(cmd)
			if err != nil {
				return err
			}

			req := engine.BuildRequest{APIVersion: engine.APIVersion, Command: engine.CommandInspect, RepoRoot: cwd}
			resp, err := invokeEngine(cmd.Context(), mode, req, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Resolved)
		},
	}
	cmd.Flags().String("mode", "", modeFlagUsage)
	return cmd
}
