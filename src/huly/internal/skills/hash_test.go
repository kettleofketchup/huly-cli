package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTree lays down a minimal skill tree in dir and returns it as an fs.FS.
func writeSkill(t *testing.T, dir, body string) {
	t.Helper()
	skill := "---\nname: t\ndescription: d\nmetadata:\n  managed_by: huly-cli\n---\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "r.md"), []byte("ref\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestContentHashExcludesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "# Body\n")
	base, err := ContentHash(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}

	// Stamping metadata into SKILL.md must NOT change the hash.
	raw, _ := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	stamped, err := Stamp(raw, "huly-cli", "9.9.9", "sha256:deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), stamped, 0o644); err != nil {
		t.Fatal(err)
	}
	afterStamp, err := ContentHash(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if afterStamp != base {
		t.Errorf("frontmatter stamp changed hash: %s -> %s", base, afterStamp)
	}

	// Editing the body MUST change the hash.
	writeSkill(t, dir, "# Body edited\n")
	edited, err := ContentHash(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if edited == base {
		t.Error("body edit did not change hash")
	}
}

func TestSkillContentHashStable(t *testing.T) {
	s, ok := Get("huly-issue-tracking")
	if !ok {
		t.Fatal("seed skill missing")
	}
	h, err := s.contentHash()
	if err != nil {
		t.Fatal(err)
	}
	if len(h) < len("sha256:") || h[:7] != "sha256:" {
		t.Errorf("hash not prefixed: %q", h)
	}
}

// Sibling files must be hashed verbatim: an implementation that hashes only
// SKILL.md would pass TestContentHashExcludesFrontmatter but fail here.
func TestContentHashIncludesSiblings(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "# Body\n")
	base, err := ContentHash(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}

	// Editing a sibling file (not SKILL.md) MUST change the hash.
	if err := os.WriteFile(filepath.Join(dir, "references", "r.md"), []byte("ref edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edited, err := ContentHash(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if edited == base {
		t.Error("sibling file edit did not change hash (siblings not hashed verbatim)")
	}

	// Adding a NEW sibling file MUST also change the hash.
	if err := os.WriteFile(filepath.Join(dir, "references", "extra.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := ContentHash(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if added == edited {
		t.Error("new sibling file did not change hash")
	}
}
