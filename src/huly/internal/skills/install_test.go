package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// agentAt returns an Agent whose SkillsDir is a fresh temp dir.
func agentAt(t *testing.T) Agent {
	t.Helper()
	return Agent{ID: "test", Label: "Test", RootDir: t.TempDir(), SkillsDir: filepath.Join(t.TempDir(), "skills"), Present: true}
}

func seed(t *testing.T) Skill {
	t.Helper()
	sk, ok := Get("huly-issue-tracking")
	if !ok {
		t.Fatal("seed skill missing")
	}
	return sk
}

func TestInstallFreshThenIdempotent(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}

	r, err := Install(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusInstalled {
		t.Fatalf("first install status = %q", r.Status)
	}

	// Second install of the same shipped version is idempotent: up-to-date,
	// no re-copy.
	r2, err := Install(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Status != StatusUpToDate {
		t.Errorf("second install status = %q, want up-to-date", r2.Status)
	}
}

func TestUpdateAbsentIsSkipped(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	r, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusSkipped || r.Reason != "absent" {
		t.Errorf("update-absent = %q/%q, want skipped/absent", r.Status, r.Reason)
	}
}

func TestUserEditedIsConflictThenForce(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)

	// Edit the installed body -> on-disk hash diverges from stored.
	md := filepath.Join(dest, "SKILL.md")
	raw, _ := os.ReadFile(md)
	if err := os.WriteFile(md, append(raw, []byte("\nuser edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Update(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "modified" {
		t.Fatalf("edited update = %q/%q, want conflict/modified", r.Status, r.Reason)
	}

	// --force backs up and overwrites.
	rf, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.2.0", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if rf.Status != StatusUpdated {
		t.Errorf("forced update = %q, want updated", rf.Status)
	}
	baks, _ := filepath.Glob(dest + ".bak-*")
	if len(baks) == 0 {
		t.Error("force did not back up the user-edited dir")
	}
}

func TestForeignIsConflict(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// A foreign skill: SKILL.md without our managed_by marker.
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"),
		[]byte("---\nname: huly-issue-tracking\n---\nsomeone else's\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "foreign" {
		t.Errorf("foreign = %q/%q, want conflict/foreign", r.Status, r.Reason)
	}
}

func TestUnreadableIsConflictNotClobbered(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// dest exists but no SKILL.md -> cannot prove ownership -> conflict.
	if err := os.WriteFile(filepath.Join(dest, "other.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "unreadable" {
		t.Errorf("unreadable = %q/%q, want conflict/unreadable", r.Status, r.Reason)
	}
	// The foreign file must survive (not clobbered).
	if _, err := os.Stat(filepath.Join(dest, "other.txt")); err != nil {
		t.Error("unowned dir was clobbered without --force")
	}
}

func TestShippedContentChangedUpdates(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	md := filepath.Join(dest, "SKILL.md")

	// Simulate an OLDER shipped hash by rewriting the stored content_hash to a
	// value that differs from the embedded one, WITHOUT touching the body
	// (so on-disk body hash still matches this stored value's basis is the
	// point of contention). To keep the body/stored consistent we instead
	// re-stamp a bogus stored hash and also make the on-disk body match it:
	// simplest reliable check — set stored hash to embedded is up-to-date;
	// to force "updated" we edit the body AND re-stamp so on-disk==stored but
	// embedded!=stored.
	raw, _ := os.ReadFile(md)
	// Append a body line, then re-stamp content_hash to the NEW on-disk hash
	// so the skill reads as "ours, unmodified, but shipped differs".
	edited := append(raw, []byte("\nshipped-delta\n")...)
	if err := os.WriteFile(md, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	newOnDisk, err := ContentHash(os.DirFS(dest))
	if err != nil {
		t.Fatal(err)
	}
	restamped, err := Stamp(edited, "huly-cli", "0.2.0", newOnDisk)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(md, restamped, 0o644); err != nil {
		t.Fatal(err)
	}

	// Now: ours, on-disk == stored (both newOnDisk), embedded != stored -> updated.
	r, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusUpdated {
		t.Errorf("shipped-changed update = %q, want updated", r.Status)
	}
	// After update the on-disk hash equals the embedded hash again.
	emb, _ := sk.contentHash()
	after, _ := ContentHash(os.DirFS(dest))
	if after != emb {
		t.Errorf("post-update hash %s != embedded %s", after, emb)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusInstalled {
		t.Errorf("dry-run status = %q, want installed (predicted)", r.Status)
	}
	if _, err := os.Stat(filepath.Join(ag.SkillsDir, sk.Name)); !os.IsNotExist(err) {
		t.Error("dry-run wrote to disk")
	}
}

func TestUpToDateRestampsVersionHashNeutral(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	if _, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0"}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)

	// An unchanged skill under a newer binary: up-to-date, but provenance
	// version re-stamped to 0.3.0 WITHOUT moving the content hash.
	r, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusUpToDate {
		t.Fatalf("version-bump update = %q, want up-to-date", r.Status)
	}
	raw, _ := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	fm, _ := Parse(raw)
	if fm.Version != "0.3.0" {
		t.Errorf("stamped version = %q, want 0.3.0", fm.Version)
	}
	// Hash-neutral proof: a third update at 0.3.0 is still up-to-date (would be
	// "modified" if the re-stamp had perturbed the hashed body).
	r3, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r3.Status != StatusUpToDate {
		t.Fatalf("third update = %q, want up-to-date (no churn)", r3.Status)
	}
}

func TestAdoptedMissingContentHash(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	md := filepath.Join(dest, "SKILL.md")

	// Simulate an older huly that stamped managed_by/version but no content_hash,
	// with the body still matching the shipped skill.
	raw, _ := os.ReadFile(md)
	stripped, err := Stamp(raw, "huly-cli", "0.2.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(md, stripped, 0o644); err != nil {
		t.Fatal(err)
	}
	if fm, _ := Parse(stripped); fm.ContentHash != "" {
		t.Fatalf("fixture: content_hash = %q, want empty", fm.ContentHash)
	}

	r, err := Update(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusUpdated || r.Reason != "adopted" {
		t.Fatalf("adopt = %q/%q, want updated/adopted", r.Status, r.Reason)
	}
	emb, _ := sk.contentHash()
	after, _ := Parse(mustRead(t, md))
	if after.ContentHash != emb {
		t.Errorf("adopted content_hash = %q, want %q", after.ContentHash, emb)
	}
	onDisk, _ := ContentHash(os.DirFS(dest))
	if onDisk != emb {
		t.Errorf("adopted tree hash %s != embedded %s", onDisk, emb)
	}
}

// Pre-hash install whose body the user edited: adopt must NOT clobber it — the
// state is ambiguous (old shipped content vs a user edit), so it is a conflict.
func TestAdoptedEditedBodyIsConflict(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	md := filepath.Join(dest, "SKILL.md")
	raw, _ := os.ReadFile(md)
	stripped, err := Stamp(raw, "huly-cli", "0.2.0", "") // pre-hash
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(md, append(stripped, []byte("\nuser edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Update(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "modified" {
		t.Fatalf("adopt-edited = %q/%q, want conflict/modified", r.Status, r.Reason)
	}
	if !strings.Contains(string(mustRead(t, md)), "user edit") {
		t.Error("adopt-edited clobbered the user's edit without --force")
	}
}

func TestUnreadableForceRepairs(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "other.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusRepaired || r.Reason != "unreadable" {
		t.Fatalf("forced unreadable = %q/%q, want repaired/unreadable", r.Status, r.Reason)
	}
	if baks, _ := filepath.Glob(dest + ".bak-*"); len(baks) == 0 {
		t.Error("force did not back up the unreadable dir")
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Error("SKILL.md missing after forced repair")
	}
}

func TestForeignForceRepairs(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"),
		[]byte("---\nname: huly-issue-tracking\n---\nsomeone else's\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusRepaired || r.Reason != "foreign" {
		t.Fatalf("forced foreign = %q/%q, want repaired/foreign", r.Status, r.Reason)
	}
	baks, _ := filepath.Glob(dest + ".bak-*")
	if len(baks) == 0 {
		t.Fatal("force did not back up the foreign dir")
	}
	bakRaw, err := os.ReadFile(filepath.Join(baks[0], "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bakRaw), "someone else's") {
		t.Error("backup does not preserve the foreign content")
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
