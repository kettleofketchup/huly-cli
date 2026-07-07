package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// writeTree copies the embedded subtree for sk to dest, stamping the root
// SKILL.md with managed_by/huly_cli_version=version/content_hash=<embedded>.
// It writes into a temp dir under dest's parent (same filesystem), then
// RemoveAll(dest)+Rename to swap it in. Crash-recoverable, not atomic:
// os.Rename cannot replace a non-empty directory, hence the RemoveAll.
func writeTree(sk Skill, dest, version string) error {
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", parent, err)
	}
	sweepStale(parent)

	embHash, err := sk.contentHash()
	if err != nil {
		return err
	}
	sub, err := fs.Sub(assetsFS, sk.fsPath)
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp(parent, "."+filepath.Base(dest)+".new-*")
	if err != nil {
		return fmt.Errorf("mkdtemp: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmp)
		}
	}()

	err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		target := filepath.Join(tmp, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, rerr := fs.ReadFile(sub, p)
		if rerr != nil {
			return rerr
		}
		if p == "SKILL.md" {
			raw, rerr = Stamp(raw, "huly-cli", version, embHash)
			if rerr != nil {
				return rerr
			}
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		return fmt.Errorf("copy tree: %w", err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("remove old %s: %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename into place: %w", err)
	}
	success = true
	return nil
}

// sweepStale removes orphaned ".<name>.new-*" temp dirs left by a crashed
// writeTree. Called before creating a fresh temp dir, so it never removes the
// in-flight one.
func sweepStale(parent string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, ".") && strings.Contains(n, ".new-") {
			_ = os.RemoveAll(filepath.Join(parent, n))
		}
	}
}
