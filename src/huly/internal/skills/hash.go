package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
)

// ContentHash returns "sha256:<hex>" over a skill tree: for SKILL.md the body
// after its frontmatter (frontmatter excluded), and every other file
// verbatim, keyed by sorted relative path. The hash is invariant across the
// authored->installed transform because the frontmatter (which carries the
// injected metadata) never contributes.
func ContentHash(tree fs.FS) (string, error) {
	type entry struct {
		path    string
		content []byte
	}
	var entries []entry
	err := fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, rerr := fs.ReadFile(tree, p)
		if rerr != nil {
			return rerr
		}
		content := raw
		if p == "SKILL.md" {
			if _, body, ok := Split(raw); ok {
				content = body
			}
		}
		entries = append(entries, entry{path: p, content: content})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("hash tree: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.path))
		h.Write([]byte{0})
		h.Write(e.content)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// contentHash hashes the embedded subtree for this skill.
func (s Skill) contentHash() (string, error) {
	sub, err := fs.Sub(assetsFS, s.fsPath)
	if err != nil {
		return "", err
	}
	return ContentHash(sub)
}
