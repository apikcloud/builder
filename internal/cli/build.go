package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/apikcloud/odoo-builder/internal/buildkit"
	"github.com/apikcloud/odoo-builder/internal/engine"
	"github.com/apikcloud/odoo-builder/internal/launcher"
)

// launcherNeeded, launcherDetectRuntime, and launcherRun are package-level
// vars (mirroring newEngine in internal/cli/engine.go) so tests can bypass
// or fake the container-launcher path without touching real
// docker/podman/buildctl/buildkitd.
var (
	launcherNeeded        = launcher.Needed
	launcherDetectRuntime = launcher.DetectRuntime
	launcherRun           = launcher.Run
	launcherLoad          = launcher.Load
)

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the Odoo image",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			modeFlag, err := cmd.Flags().GetString("mode")
			if err != nil {
				return err
			}
			mode, err := launcher.ResolveMode(modeFlag)
			if err != nil {
				return err
			}

			loadFlag, err := cmd.Flags().GetBool("load")
			if err != nil {
				return err
			}

			needsLauncher := mode == launcher.ModeLauncher || (mode == launcher.ModeAuto && launcherNeeded())

			if loadFlag && needsLauncher {
				return fmt.Errorf("builder: --load requires Engine Mode (buildctl/buildkitd on PATH) — pass --mode engine, install BuildKit locally, or drop --load and run `docker load -i` on the OCI/docker tarball manually")
			}

			if !loadFlag && needsLauncher {
				return runViaLauncher(cmd, cwd)
			}

			req := engine.BuildRequest{RepoRoot: cwd}
			if loadFlag {
				req.Output.Type = "docker"
			}

			result, err := newEngine().Build(cmd.Context(), req)
			if err != nil {
				if !loadFlag && mode == launcher.ModeAuto && errors.Is(err, buildkit.ErrRootlessRequired) {
					fmt.Fprintln(cmd.ErrOrStderr(), "builder: buildkitd can't run directly on this host (needs root/RootlessKit) — retrying inside the odoo-builder container image")
					return runViaLauncher(cmd, cwd)
				}
				return err
			}

			if loadFlag {
				runtime, err := launcherDetectRuntime()
				if err != nil {
					return err
				}
				if err := launcherLoad(cmd.Context(), runtime, result.ImagePath, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
					return fmt.Errorf("builder: built %s but loading into %s failed: %w (archive kept at %s — retry manually with `%s load -i %s`)",
						result.ImageRef, runtime, err, result.ImagePath, runtime, result.ImagePath)
				}
				_ = os.Remove(result.ImagePath)
				fmt.Fprintf(cmd.OutOrStdout(), "loaded image %s into %s (%d addon(s))\n", result.ImageRef, runtime, result.AddonCount)
				return nil
			}

			if result.ImageRef != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "pushed image %s (%d addon(s))\n", result.ImageRef, result.AddonCount)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "built image at %s (%d addon(s))\n", result.ImagePath, result.AddonCount)
			}
			return nil
		},
	}
	cmd.Flags().String("mode", "", `override runtime detection: "engine" (force direct BuildKit, no container, no rootless retry) or "launcher" (force the container, even if buildctl/buildkitd are on PATH); default "auto" probes buildctl/buildkitd, or set ODOO_BUILDER_MODE`)
	cmd.Flags().Bool("load", false, `build and load the image into the local Docker/Podman image store instead of pushing or writing an OCI tarball (docker load/podman load) — no push happens; requires odoo-builder.yaml's image.name and Engine Mode (errors under --mode launcher or when buildctl/buildkitd are missing)`)
	return cmd
}

func runViaLauncher(cmd *cobra.Command, cwd string) error {
	runtime, err := launcherDetectRuntime()
	if err != nil {
		return err
	}
	return launcherRun(cmd.Context(), runtime, cwd, []string{"build"}, cmd.OutOrStdout(), cmd.ErrOrStderr())
}
