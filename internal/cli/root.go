// SPDX-License-Identifier: MIT
// Package cli wires the builder CLI's Cobra commands.
package cli

import (
	"context"

	"github.com/spf13/cobra"
)

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

// Execute runs the builder CLI's root command against ctx — cancelling ctx
// (main.go ties it to SIGINT/SIGTERM) unblocks whichever RunE is in
// flight via its cmd.Context(), which every subprocess call in this
// package (invokeEngine, launcherLoad) threads straight down to an
// exec.CommandContext, so the in-flight buildctl/buildkitd/docker-or-podman
// child is killed instead of being left running after the CLI itself
// returns.
func Execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}
