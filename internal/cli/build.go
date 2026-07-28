// SPDX-License-Identifier: MIT
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/apikcloud/odoo-builder/internal/engine"
	"github.com/apikcloud/odoo-builder/internal/launcher"
)

// launcherDetectRuntime and launcherLoad are package-level vars (mirroring
// invokeEngine in internal/cli/invoker.go) so tests can bypass or fake the
// post-build --load step without touching real docker/podman.
var (
	launcherDetectRuntime = launcher.DetectRuntime
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

			mode, err := resolveModeFlag(cmd)
			if err != nil {
				return err
			}

			loadFlag, err := cmd.Flags().GetBool("load")
			if err != nil {
				return err
			}

			if loadFlag && mode == launcher.ModeLauncher {
				return fmt.Errorf("builder: --load requires Engine Mode (buildctl/buildkitd on PATH) — pass --mode engine, install BuildKit locally, or drop --load and run `docker load -i` on the OCI/docker tarball manually")
			}
			if loadFlag && mode == launcher.ModeAuto && launcher.Needed(engine.CommandBuild) {
				return fmt.Errorf("builder: --load requires Engine Mode (buildctl/buildkitd on PATH) — pass --mode engine, install BuildKit locally, or drop --load and run `docker load -i` on the OCI/docker tarball manually")
			}

			req := engine.BuildRequest{APIVersion: engine.APIVersion, Command: engine.CommandBuild, RepoRoot: cwd}
			if loadFlag {
				req.Load = true
				req.Output.Type = "docker"
			}

			resp, err := invokeEngine(cmd.Context(), mode, req, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			if loadFlag {
				runtime, err := launcherDetectRuntime()
				if err != nil {
					return err
				}
				if err := launcherLoad(cmd.Context(), runtime, resp.ImagePath, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
					return fmt.Errorf("builder: built %s but loading into %s failed: %w (archive kept at %s — retry manually with `%s load -i %s`)",
						resp.ImageRef, runtime, err, resp.ImagePath, runtime, resp.ImagePath)
				}
				_ = os.Remove(resp.ImagePath)
				fmt.Fprintf(cmd.OutOrStdout(), "loaded image %s into %s (%d addon(s))\n", resp.ImageRef, runtime, resp.AddonCount)
				return nil
			}

			if resp.ImageRef != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "pushed image %s (%d addon(s))\n", resp.ImageRef, resp.AddonCount)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "built image at %s (%d addon(s))\n", resp.ImagePath, resp.AddonCount)
			}
			return nil
		},
	}
	cmd.Flags().String("mode", "", modeFlagUsage)
	cmd.Flags().Bool("load", false, `build and load the image into the local Docker/Podman image store instead of pushing or writing an OCI tarball (docker load/podman load) — no push happens; requires odoo-builder.yaml's image.name and Engine Mode (errors under --mode launcher or when the engine binary/buildctl/buildkitd are missing)`)
	return cmd
}
