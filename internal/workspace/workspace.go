// Package workspace provides filesystem primitives for building a
// deterministic .build/ context.
package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Workspace is a build context rooted at a directory that is wiped and
// recreated on New, so repeated runs start from a clean, deterministic
// state.
type Workspace struct {
	Root string
}

// New removes root (if it exists) and recreates it empty.
func New(root string) (*Workspace, error) {
	if err := os.RemoveAll(root); err != nil {
		return nil, fmt.Errorf("workspace: removing %s: %w", root, err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("workspace: creating %s: %w", root, err)
	}
	return &Workspace{Root: root}, nil
}

// EnsureDir creates path (and any parents) if it does not already exist.
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("workspace: creating %s: %w", path, err)
	}
	return nil
}

// CopyFile copies src to dst, preserving the source file's permission
// bits. File modification time is not preserved, so repeated copies of
// unchanged content are byte-identical regardless of source mtime.
func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("workspace: stat %s: %w", src, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("workspace: opening %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("workspace: creating parent of %s: %w", dst, err)
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("workspace: creating %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("workspace: copying %s to %s: %w", src, dst, err)
	}

	return nil
}

// gitMetadataNames are skipped by CopyDir at every level, so a submodule's
// working tree (which carries a .git file/directory and, if it is itself a
// superproject, a .gitmodules file) flattens into plain addon content with
// no git metadata, per the "no nested repositories, no git metadata"
// flattening requirement.
var gitMetadataNames = map[string]bool{
	".git":        true,
	".gitmodules": true,
}

// CopyDir recursively copies src into dst. Regular files are copied via
// CopyFile (mode preserved, mtime not). Symlinks are recreated verbatim
// via os.Symlink with their original (possibly relative, possibly
// dangling) target — CopyDir does not resolve nested symlinks, only the
// caller-supplied src root is assumed already resolved. Directory entries
// are visited in sorted order for determinism. Entries named .git or
// .gitmodules are skipped at every level.
func CopyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("workspace: reading %s: %w", src, err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("workspace: stat %s: %w", src, err)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return fmt.Errorf("workspace: creating %s: %w", dst, err)
	}

	for _, entry := range entries {
		if gitMetadataNames[entry.Name()] {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		lst, err := os.Lstat(srcPath)
		if err != nil {
			return fmt.Errorf("workspace: stat %s: %w", srcPath, err)
		}

		switch {
		case lst.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("workspace: reading link %s: %w", srcPath, err)
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return fmt.Errorf("workspace: creating symlink %s: %w", dstPath, err)
			}
		case lst.IsDir():
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		default:
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
