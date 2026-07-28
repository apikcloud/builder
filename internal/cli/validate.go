package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/apikcloud/odoo-builder/internal/engine"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the repository layout and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			errs := newEngine().Validate(engine.BuildRequest{RepoRoot: cwd})
			if len(errs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "OK")
				return nil
			}

			for _, e := range errs {
				fmt.Fprintln(cmd.ErrOrStderr(), e)
			}
			return fmt.Errorf("validate: %d error(s) found", len(errs))
		},
	}
}
