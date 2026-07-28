// SPDX-License-Identifier: MIT
// Package cli wires the builder CLI's Cobra commands.
package cli

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "odoo-builder",
		Short:         "Reproducible OCI image builder for Odoo deployments",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newPrepareCmd())
	root.AddCommand(newBuildCmd())
	root.AddCommand(newInspectCmd())

	return root
}

// Execute runs the builder CLI's root command.
func Execute() error {
	return newRootCmd().Execute()
}
