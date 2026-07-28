package addons

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InitSubmodules runs `git submodule update --init` (and `--recursive` if
// recursive is true) in repoRoot. A no-op (returns nil immediately) if
// repoRoot has no .gitmodules file, so callers can invoke it
// unconditionally once Submodules.Init is true without pre-checking.
func InitSubmodules(repoRoot string, recursive bool) error {
	if _, err := os.Stat(filepath.Join(repoRoot, ".gitmodules")); os.IsNotExist(err) {
		return nil
	}

	args := []string{"submodule", "update", "--init"}
	if recursive {
		args = append(args, "--recursive")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("addons: git submodule update --init: %w: %s", err, stderr.String())
	}
	return nil
}
