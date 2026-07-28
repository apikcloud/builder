package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/apikcloud/odoo-builder/internal/engine"
)

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Print the resolved BuildRequest a build would use, without building",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			req, err := newEngine().Inspect(engine.BuildRequest{RepoRoot: cwd})
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(req)
		},
	}
}
