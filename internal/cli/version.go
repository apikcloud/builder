// SPDX-License-Identifier: MIT
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/apikcloud/builder/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the builder version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return nil
		},
	}
}
