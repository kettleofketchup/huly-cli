package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTreeStampsAndCopies(t *testing.T) {
	sk, ok := Get("huly-issue-tracking")
	if !ok {
		t.Fatal("seed skill missing")
	}
	dest := filepath.Join(t.TempDir(), "skills", sk.Name)

	if err := writeTree(sk, dest, "0.2.0"); err != nil {
		t.Fatal(err)
	}

	// SKILL.md present and stamped with all three metadata keys.
	raw, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	fm, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if fm.ManagedBy != "huly-cli" || fm.Version != "0.2.0" || fm.ContentHash == "" {
		t.Errorf("SKILL.md not fully stamped: %+v", fm)
	}

	// The on-disk content hash equals the embedded hash (frontmatter excluded),
	// i.e. a fresh install is NOT misread as modified.
	emb, err := sk.contentHash()
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := ContentHash(os.DirFS(dest))
	if err != nil {
		t.Fatal(err)
	}
	if onDisk != emb {
		t.Errorf("fresh install hash %s != embedded %s", onDisk, emb)
	}
	if fm.ContentHash != emb {
		t.Errorf("stamped content_hash %s != embedded %s", fm.ContentHash, emb)
	}
}

func TestWriteTreeReplacesPopulatedDest(t *testing.T) {
	sk, _ := Get("huly-issue-tracking")
	dest := filepath.Join(t.TempDir(), "skills", sk.Name)

	// Pre-populate dest with junk so the swap must replace a non-empty dir
	// (regression for the os.Rename-over-non-empty-dir bug).
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "stale.txt"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeTree(sk, dest, "0.2.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "stale.txt")); !os.IsNotExist(err) {
		t.Error("stale file survived replacement")
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Error("SKILL.md missing after replacement")
	}
}

func TestSweepStale(t *testing.T) {
	parent := t.TempDir()
	orphan := filepath.Join(parent, ".huly-issue-tracking.new-XXXX")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(parent, "real-skill")
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	sweepStale(parent)
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphan .new- dir not swept")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("real skill dir wrongly swept")
	}
	// Sanity: the orphan name matches the pattern we sweep.
	if !strings.Contains(filepath.Base(orphan), ".new-") {
		t.Fatal("test fixture wrong")
	}
}

// Integration: writeTree must sweep a prior crash's orphan .new- dir and still
// succeed (the crash-recovery path end to end, not sweepStale in isolation).
func TestWriteTreeSweepsOrphanBeforeWriting(t *testing.T) {
	sk, _ := Get("huly-issue-tracking")
	parent := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, sk.Name)
	orphan := filepath.Join(parent, "."+sk.Name+".new-DEAD")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphan, "leftover.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeTree(sk, dest, "0.2.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphaned .new- dir from a prior crash was not swept")
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Error("write incomplete with a pre-existing orphan present")
	}
}
