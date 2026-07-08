# huly skills — Phase A1 (primitives) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the pure, UI-free primitives for the embedded-skill distributor: a lifted semver helper, the seed skill asset, frontmatter parse/stamp, the embedded catalog, and the content hash.

**Architecture:** All new code lands in a new leaf package `src/huly/internal/skills` (plus a new leaf `src/huly/internal/semver`). No Cobra, no charmbracelet — this phase is engine primitives only, so every deliverable is unit-testable with `go test`. Later phases (A2 engine state machine, B CLI, C TUI) build on these functions. Spec: `docs/superpowers/specs/2026-07-07-huly-skills-distribution-design.md`.

**Tech Stack:** Go 1.25, `gopkg.in/yaml.v3` (already a dependency), `embed`, `crypto/sha256`. Standard library otherwise.

## Global Constraints

- Module path: `github.com/kettleofketchup/huly-cli/src/huly`; module root is `src/huly/`. All Go import paths are under it.
- Go version floor: `go 1.25.0` (from `src/huly/go.mod`).
- **No Cobra and no charmbracelet imports in `internal/skills` or `internal/semver`.** These packages must stay UI-free (Phase A1/A2 rule).
- **Field ownership in `SKILL.md` frontmatter:** *authored* (shipped in the asset) = `name`, `description`, `metadata.managed_by: "huly-cli"`. *Injected at install* = `metadata.huly_cli_version`, `metadata.content_hash`. Injected metadata scalar values are always emitted **quoted**.
- **Content hash** = SHA-256 over the `SKILL.md` **body** (frontmatter excluded) plus every non-`SKILL.md` file verbatim, keyed by sorted relative path. The frontmatter is never part of the hash. Prefix the hex digest with `sha256:`.
- **Semver helper normalizes internally**: it trims a leading `v` before parsing so a `v`-prefixed value can never zero the major slot. It is display/provenance-only; it never gates upgrades.
- TDD: write the failing test first, watch it fail, implement minimally, watch it pass, commit. One logical change per commit.
- Full test suite: `just go::test huly` (runs `go test -race -cover -v ./...` in `src/huly`). Targeted test during development: `cd src/huly && go test ./internal/<pkg>/ -run <TestName> -v` — raw `go test` is allow-listed and unhooked in this repo, so it runs directly.
- Commit message convention (repo style): `feat(skills): …`, `refactor(semver): …`, etc. No Claude watermark/co-author lines.

## File Structure

- `src/huly/internal/semver/semver.go` — `Compare` + internal `parse`, lifted from `cmd/update.go`.
- `src/huly/internal/semver/semver_test.go` — semver tests (incl. the `v`-prefix/major regression).
- `src/huly/cmd/update.go` — MODIFY: delete local `compareVersions`/`parseVersion`, call `semver.Compare`.
- `src/huly/internal/skills/assets/huly-issue-tracking/SKILL.md` — the seed skill (authored asset).
- `src/huly/internal/skills/seed_test.go` — asset acceptance criteria 1 & 3 (string-level, disk-read).
- `src/huly/internal/skills/frontmatter.go` — `Split`, `Parse`, `Stamp`, the `Frontmatter` type.
- `src/huly/internal/skills/frontmatter_test.go` — split/parse/stamp tests.
- `src/huly/internal/skills/catalog.go` — `//go:embed all:assets`, `Skill`, `Catalog`, `Get`.
- `src/huly/internal/skills/catalog_test.go` — catalog integrity test.
- `src/huly/internal/skills/hash.go` — `ContentHash(fs.FS)`.
- `src/huly/internal/skills/hash_test.go` — hash invariance tests.
- `src/huly/internal/skills/testdata/…` — fixtures for frontmatter/hash tests.

---

### Task 1: Lift the semver helper into `internal/semver`

**Files:**
- Create: `src/huly/internal/semver/semver.go`
- Create: `src/huly/internal/semver/semver_test.go`
- Modify: `src/huly/cmd/update.go` (remove `compareVersions`/`parseVersion` at ~L364–412; rewire the call at L87)

**Interfaces:**
- Consumes: nothing.
- Produces: `func semver.Compare(a, b string) int` — returns `-1` if `a < b`, `0` if equal, `+1` if `a > b`, using `major.minor.patch`. Trims a leading `v` on each operand. `"dev"` sorts below every real version.

- [ ] **Step 1: Write the failing test**

Create `src/huly/internal/semver/semver_test.go`:

```go
package semver

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.9.0", "v2.0.0", -1}, // regression: v-prefix must not zero the major slot
		{"v2.0.0", "v1.9.0", 1},
		{"v0.1.2", "0.1.2", 0}, // leading v is irrelevant
		{"0.1.3", "0.1.3", 0},
		{"dev", "0.1.3", -1}, // dev is oldest
		{"0.1.3", "dev", 1},
		{"dev", "dev", 0},
		{"garbage", "0.0.0", 0},         // unparseable -> [0,0,0]
		{"v1.2.3-4-gabc", "v1.2.3", 0},  // git-describe suffix truncated after patch
		{"1.2.0", "1.3.0", -1},          // equal major: minor decides
		{"1.3.0", "1.2.0", 1},
		{"1.2.3", "1.2.4", -1},          // equal major+minor: patch decides
		{"1.2.4", "1.2.3", 1},
		{"2.0.0", "1.9.9", 1},           // major dominates a larger minor/patch
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/semver/ -run TestCompare -v`
Expected: FAIL — build error, package `semver` has no `Compare`.

- [ ] **Step 3: Write minimal implementation**

Create `src/huly/internal/semver/semver.go`:

```go
// Package semver provides a tiny major.minor.patch comparison used for
// display/provenance ordering. It tolerates a leading "v" and a "dev"
// sentinel. It intentionally does not implement full SemVer (pre-release,
// build metadata); git-describe suffixes after the patch are ignored.
package semver

import "strings"

// Compare returns -1 if a < b, 0 if equal, +1 if a > b. A leading "v" is
// trimmed from each operand; "dev" sorts below every real version.
func Compare(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	switch {
	case a == "dev" && b == "dev":
		return 0
	case a == "dev":
		return -1
	case b == "dev":
		return 1
	}
	pa, pb := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

// parse reads leading digits of the first three dot-separated fields into
// [major, minor, patch]. A leading "v" is trimmed. Non-numeric input yields
// zeros, so garbage compares as the lowest version.
func parse(ver string) [3]int {
	ver = strings.TrimPrefix(ver, "v")
	var parts [3]int
	for i, field := range strings.Split(ver, ".") {
		if i >= 3 {
			break
		}
		val := 0
		for _, c := range field {
			if c < '0' || c > '9' {
				break
			}
			val = val*10 + int(c-'0')
		}
		parts[i] = val
	}
	return parts
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/semver/ -run TestCompare -v`
Expected: PASS.

- [ ] **Step 5: Rewire `cmd/update.go` to the lifted helper**

In `src/huly/cmd/update.go`:
1. Add to the import block: `"github.com/kettleofketchup/huly-cli/src/huly/internal/semver"`.
2. Replace the call at L87 `if compareVersions(currentVersion, latestVersion) >= 0 {` with `if semver.Compare(currentVersion, latestVersion) >= 0 {`.
3. Delete the `compareVersions` and `parseVersion` functions (the two funcs near L362–412).

The existing `strings.TrimPrefix(..., "v")` at L80–81 stays (it feeds the printed version strings); the double-trim inside `Compare` is harmless.

- [ ] **Step 6: Verify the whole module still builds and tests pass**

Run: `cd src/huly && go build ./... && go test ./cmd/ ./internal/semver/ -v`
Expected: build succeeds; `cmd` tests (incl. `TestStagingPathSameDir`) and semver tests PASS. If `go vet` flags an unused `strings` import in `update.go`, keep it — `strings.TrimPrefix`/`strings.Split` are still used elsewhere in the file; only remove imports the compiler actually reports unused.

- [ ] **Step 7: Commit**

```bash
git add src/huly/internal/semver/ src/huly/cmd/update.go
git commit -m "refactor(semver): lift compareVersions into internal/semver leaf pkg with v-prefix fix"
```

---

### Task 2: Author the seed skill asset `huly-issue-tracking`

**Files:**
- Create: `src/huly/internal/skills/assets/huly-issue-tracking/SKILL.md`
- Create: `src/huly/internal/skills/seed_test.go`

**Interfaces:**
- Consumes: nothing (the test reads the file from disk via a relative path — Go runs tests in the package directory).
- Produces: the embedded asset that Task 4's `//go:embed all:assets` requires in order to compile. Acceptance criteria 1 (frontmatter identity) and 3 (required commands present) are enforced here at the string level; criterion 2 (resolving command paths against `rootCmd`) is deferred to Phase B.

- [ ] **Step 1: Write the failing test**

Create `src/huly/internal/skills/seed_test.go`:

```go
package skills

import (
	"os"
	"strings"
	"testing"
)

func TestSeedSkillAsset(t *testing.T) {
	raw, err := os.ReadFile("assets/huly-issue-tracking/SKILL.md")
	if err != nil {
		t.Fatalf("read seed SKILL.md: %v", err)
	}
	s := string(raw)

	// Criterion 1: frontmatter identity.
	if !strings.Contains(s, "name: huly-issue-tracking") {
		t.Error("frontmatter must declare name: huly-issue-tracking")
	}
	if !strings.Contains(s, "managed_by: huly-cli") {
		t.Error("frontmatter metadata must declare managed_by: huly-cli")
	}
	lower := strings.ToLower(s)
	if !strings.Contains(lower, "issue") || !strings.Contains(lower, "bug") {
		t.Error("description/body should mention issue and bug tracking")
	}

	// Criterion 3: required command set present (string level).
	for _, cmd := range []string{"huly project list", "huly component create", "huly issue create"} {
		if !strings.Contains(s, cmd) {
			t.Errorf("seed skill must reference %q", cmd)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/skills/ -run TestSeedSkillAsset -v`
Expected: FAIL — cannot read `assets/huly-issue-tracking/SKILL.md` (file does not exist).

- [ ] **Step 3: Write the seed skill asset**

Create `src/huly/internal/skills/assets/huly-issue-tracking/SKILL.md` with exactly this content (the command flags match `cmd/issue.go` and `cmd/component.go`):

```markdown
---
name: huly-issue-tracking
description: >-
  Track bugs and issues for a software project with the huly CLI. Use when
  logging a bug, filing an issue, or organizing work by area of the codebase.
  Records issues in a single Huly project and groups them with components.
metadata:
  managed_by: huly-cli
---

# Tracking issues and bugs with huly

Use the `huly` CLI to record bugs and issues for this repository in Huly, and
group them by area of the codebase using **components**.

## One project per repository

Track everything for this repository in a single Huly project. Find it once:

```sh
huly project list
```

Pass the project to every command with `--project <identifier>`, or set
`defaults.project` in config once and omit the flag thereafter.

## Group the codebase with components

A **component** is a named area of the codebase (for example `cli`, `api`,
`docs`). Create one per area you want to track separately, then file issues
against it.

```sh
huly component list --project <id>
huly component create --project <id> --label "cli" --description "Command-line interface"
```

## File a bug or issue

Set `--component` so the issue is grouped, and a `--priority` of
`NoPriority`, `Urgent`, `High`, `Medium`, or `Low`.

```sh
huly issue create --project <id> \
  --title "Login fails on empty OTP" \
  --description "Steps to reproduce ..." \
  --component "cli" \
  --priority High
```

Inspect and progress issues:

```sh
huly issue list --project <id>
huly issue view <ISSUE-ID>
huly issue update <ISSUE-ID> --status "In Progress"
```

## Workflow

1. Confirm the project with `huly project list`.
2. Ensure a component exists for the area: `huly component list`, otherwise
   `huly component create`.
3. Create the issue with `huly issue create`, setting `--component` and a
   sensible `--priority`.
4. Track with `huly issue list` / `huly issue view` and change state with
   `huly issue update`.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/skills/ -run TestSeedSkillAsset -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/huly/internal/skills/assets/ src/huly/internal/skills/seed_test.go
git commit -m "feat(skills): add huly-issue-tracking seed skill asset"
```

---

### Task 3: Frontmatter split / parse / surgical stamp

**Files:**
- Create: `src/huly/internal/skills/frontmatter.go`
- Create: `src/huly/internal/skills/frontmatter_test.go`
- Create: `src/huly/internal/skills/testdata/folded.md`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Frontmatter struct { Name, Description, ManagedBy, Version, ContentHash string }`
  - `func Split(src []byte) (front, body []byte, ok bool)` — `front` is the text between the first two `---` fence lines; `body` is everything after the second fence; `ok` is false when there is no leading `---` fence pair.
  - `func Parse(src []byte) (Frontmatter, error)`
  - `func Stamp(src []byte, managedBy, version, contentHash string) ([]byte, error)` — sets `metadata.managed_by/huly_cli_version/content_hash` (quoted), preserving the body byte-for-byte and the rest of the frontmatter's key order.

- [ ] **Step 1: Write the failing test**

Create `src/huly/internal/skills/testdata/folded.md`:

```markdown
---
name: sample
description: >-
  A folded multi-line
  description value.
metadata:
  managed_by: huly-cli
---
# Body heading

Body line with trailing spaces kept.
```

Create `src/huly/internal/skills/frontmatter_test.go`:

```go
package skills

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	src := []byte("---\nname: x\n---\nbody here\n")
	front, body, ok := Split(src)
	if !ok {
		t.Fatal("expected ok")
	}
	if !strings.Contains(string(front), "name: x") {
		t.Errorf("front = %q", front)
	}
	if string(body) != "body here\n" {
		t.Errorf("body = %q", body)
	}

	if _, _, ok := Split([]byte("no frontmatter here")); ok {
		t.Error("expected ok=false when no fence")
	}
}

func TestParse(t *testing.T) {
	raw, err := os.ReadFile("testdata/folded.md")
	if err != nil {
		t.Fatal(err)
	}
	fm, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if fm.Name != "sample" {
		t.Errorf("Name = %q", fm.Name)
	}
	if !strings.Contains(fm.Description, "folded multi-line") {
		t.Errorf("Description = %q", fm.Description)
	}
	if fm.ManagedBy != "huly-cli" {
		t.Errorf("ManagedBy = %q", fm.ManagedBy)
	}
}

func TestStampPreservesBodyAndQuotes(t *testing.T) {
	raw, err := os.ReadFile("testdata/folded.md")
	if err != nil {
		t.Fatal(err)
	}
	_, wantBody, _ := Split(raw)

	out, err := Stamp(raw, "huly-cli", "0.2.0", "sha256:abc")
	if err != nil {
		t.Fatal(err)
	}

	// Body is byte-for-byte preserved.
	_, gotBody, ok := Split(out)
	if !ok {
		t.Fatal("stamped output lost its frontmatter fence")
	}
	if !bytes.Equal(gotBody, wantBody) {
		t.Errorf("body changed:\n got %q\nwant %q", gotBody, wantBody)
	}

	// Injected values are present and quoted.
	fm, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if fm.Version != "0.2.0" || fm.ContentHash != "sha256:abc" {
		t.Errorf("injected fields not read back: %+v", fm)
	}
	s := string(out)
	if !strings.Contains(s, `huly_cli_version: "0.2.0"`) {
		t.Errorf("version not quoted in output:\n%s", s)
	}
	if !strings.Contains(s, `content_hash: "sha256:abc"`) {
		t.Errorf("content_hash not quoted in output:\n%s", s)
	}

	// Re-stamping the SAME values is idempotent (stable output).
	out2, err := Stamp(out, "huly-cli", "0.2.0", "sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, out2) {
		t.Error("re-stamp not idempotent")
	}
}

// Stamp must create the metadata mapping when the authored skill has none
// (a legal shape per spec §7). Exercises the create-branch of mappingValue.
func TestStampCreatesMetadataWhenAbsent(t *testing.T) {
	src := []byte("---\nname: bare\ndescription: no metadata here\n---\n# Body\n")
	out, err := Stamp(src, "huly-cli", "1.0.0", "sha256:cafe")
	if err != nil {
		t.Fatal(err)
	}
	fm, err := Parse(out)
	if err != nil {
		t.Fatalf("parse stamped: %v", err)
	}
	if fm.ManagedBy != "huly-cli" || fm.Version != "1.0.0" || fm.ContentHash != "sha256:cafe" {
		t.Errorf("metadata not created on stamp: %+v", fm)
	}
	if !strings.Contains(string(out), `content_hash: "sha256:cafe"`) {
		t.Errorf("created content_hash not quoted:\n%s", out)
	}
	if _, body, _ := Split(out); string(body) != "# Body\n" {
		t.Errorf("body changed: %q", body)
	}
}

// Stamp uses a yaml.Node (not a struct round-trip) precisely to keep keys it
// does not model. A struct round-trip would silently drop license:/extra:.
func TestStampPreservesUnmodeledFields(t *testing.T) {
	src := []byte("---\n" +
		"# top comment\n" +
		"name: sample\n" +
		"license: MIT\n" + // unmodeled top-level key
		"description: d\n" +
		"metadata:\n" +
		"  managed_by: huly-cli\n" +
		"  extra: keep-me\n" + // unmodeled metadata key
		"---\n# Body\n")
	out, err := Stamp(src, "huly-cli", "0.2.0", "sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"license: MIT", "extra: keep-me"} {
		if !strings.Contains(s, want) {
			t.Errorf("Stamp dropped unmodeled field %q:\n%s", want, s)
		}
	}
	// yaml.v3 comment round-tripping is position-sensitive; if this proves
	// flaky in practice, keep the unmodeled-field asserts above and relax
	// this one — those are the load-bearing checks.
	if !strings.Contains(s, "# top comment") {
		t.Errorf("Stamp dropped comment:\n%s", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/skills/ -run 'TestSplit|TestParse|TestStamp' -v`
Expected: FAIL — build error, `Split`/`Parse`/`Stamp`/`Frontmatter` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `src/huly/internal/skills/frontmatter.go`:

```go
package skills

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds the SKILL.md fields huly reads or writes.
type Frontmatter struct {
	Name        string
	Description string
	ManagedBy   string // metadata.managed_by
	Version     string // metadata.huly_cli_version
	ContentHash string // metadata.content_hash
}

var fence = []byte("---")

// Split separates a SKILL.md into its YAML frontmatter block and the body.
// front is the bytes between the first two "---" fence lines (no fences);
// body is everything after the line following the second fence. ok is false
// when src does not begin with a "---" fence.
func Split(src []byte) (front, body []byte, ok bool) {
	lines := bytes.SplitAfter(src, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimRight(lines[0], "\r\n"), fence) {
		return nil, nil, false
	}
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimRight(lines[i], "\r\n"), fence) {
			front = bytes.Join(lines[1:i], nil)
			body = bytes.Join(lines[i+1:], nil)
			return front, body, true
		}
	}
	return nil, nil, false
}

// fmYAML mirrors the subset of frontmatter we parse.
type fmYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Metadata    struct {
		ManagedBy   string `yaml:"managed_by"`
		Version     string `yaml:"huly_cli_version"`
		ContentHash string `yaml:"content_hash"`
	} `yaml:"metadata"`
}

// Parse reads the fields huly cares about from a SKILL.md's raw bytes.
func Parse(src []byte) (Frontmatter, error) {
	front, _, ok := Split(src)
	if !ok {
		return Frontmatter{}, fmt.Errorf("no frontmatter fence")
	}
	var y fmYAML
	if err := yaml.Unmarshal(front, &y); err != nil {
		return Frontmatter{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	return Frontmatter{
		Name:        y.Name,
		Description: y.Description,
		ManagedBy:   y.Metadata.ManagedBy,
		Version:     y.Metadata.Version,
		ContentHash: y.Metadata.ContentHash,
	}, nil
}

// Stamp sets metadata.managed_by/huly_cli_version/content_hash on a SKILL.md,
// emitting the values quoted, preserving the body byte-for-byte and the rest
// of the frontmatter's key order. It rebuilds only the frontmatter via a
// yaml.Node so authored keys keep their order.
//
// NOTE: only the BODY is byte-preserved. The frontmatter is re-emitted by the
// yaml encoder, so a hand-wrapped folded ('>-') description collapses onto one
// line on the first install. This is intentional and harmless: the content
// hash excludes the frontmatter entirely (see hash.go), so the reflow is
// cosmetic and never triggers a false "modified".
func Stamp(src []byte, managedBy, version, contentHash string) ([]byte, error) {
	front, body, ok := Split(src)
	if !ok {
		return nil, fmt.Errorf("no frontmatter fence")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(front, &doc); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter is not a mapping")
	}
	root := doc.Content[0]
	meta := mappingValue(root, "metadata")
	setScalar(meta, "managed_by", managedBy)
	setScalar(meta, "huly_cli_version", version)
	setScalar(meta, "content_hash", contentHash)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("encode frontmatter: %w", err)
	}
	_ = enc.Close()

	out := make([]byte, 0, len(buf.Bytes())+len(body)+8)
	out = append(out, "---\n"...)
	out = append(out, buf.Bytes()...)
	out = append(out, "---\n"...)
	out = append(out, body...)
	return out, nil
}

// mappingValue returns the value node for key in a mapping node, creating an
// empty mapping value if the key is absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = append(m.Content, k, v)
	return v
}

// setScalar sets key=val (double-quoted string) in a mapping node.
func setScalar(m *yaml.Node, key, val string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			v := m.Content[i+1]
			v.Kind, v.Tag, v.Value, v.Style = yaml.ScalarNode, "!!str", val, yaml.DoubleQuotedStyle
			return
		}
	}
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	v := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val, Style: yaml.DoubleQuotedStyle}
	m.Content = append(m.Content, k, v)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/skills/ -run 'TestSplit|TestParse|TestStamp' -v`
Expected: PASS. If `TestStampPreservesBodyAndQuotes` fails on the idempotency check, inspect the printed output — the most likely cause is `SetIndent` differing from the authored indent; the test only requires `Stamp(Stamp(x))==Stamp(x)`, which holds regardless of the authored indent, so a failure here means a real bug in node reuse.

- [ ] **Step 5: Commit**

```bash
git add src/huly/internal/skills/frontmatter.go src/huly/internal/skills/frontmatter_test.go src/huly/internal/skills/testdata/
git commit -m "feat(skills): frontmatter split/parse/surgical stamp"
```

---

### Task 4: Embedded catalog

**Files:**
- Create: `src/huly/internal/skills/catalog.go`
- Create: `src/huly/internal/skills/catalog_test.go`

**Interfaces:**
- Consumes: `Parse` and `Frontmatter` from Task 3; the asset from Task 2.
- Produces:
  - `type Skill struct { Name, Description string; fsPath string }`
  - `func Catalog() ([]Skill, error)` — one entry per non-dot directory under `assets/`.
  - `func Get(name string) (Skill, bool)`
  - `var assetsFS embed.FS` (unexported) — the embedded `assets/` tree, reused by Task 5 via `fs.Sub`.

- [ ] **Step 1: Write the failing test**

Create `src/huly/internal/skills/catalog_test.go`:

```go
package skills

import "testing"

func TestCatalogIntegrity(t *testing.T) {
	cat, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) == 0 {
		t.Fatal("catalog is empty")
	}
	for _, s := range cat {
		if s.Name == "" {
			t.Errorf("skill at %s has empty frontmatter name", s.fsPath)
		}
		if s.Description == "" {
			t.Errorf("skill %q has empty description", s.Name)
		}
		// Directory name must match frontmatter name.
		if got := s.fsPath; got != "assets/"+s.Name {
			t.Errorf("dir %q does not match frontmatter name %q", got, s.Name)
		}
	}
	if _, ok := Get("huly-issue-tracking"); !ok {
		t.Error("Get(huly-issue-tracking) not found")
	}
	if _, ok := Get("does-not-exist"); ok {
		t.Error("Get(does-not-exist) should be false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/skills/ -run TestCatalogIntegrity -v`
Expected: FAIL — `Catalog`/`Get`/`Skill` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `src/huly/internal/skills/catalog.go`:

```go
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed all:assets
var assetsFS embed.FS

// Skill is one entry in the embedded catalog.
type Skill struct {
	Name        string
	Description string
	fsPath      string // path within assetsFS, e.g. "assets/huly-issue-tracking"
}

// Catalog returns every embedded skill, one per non-dot directory under
// assets/. Dot-prefixed entries are ignored.
func Catalog() ([]Skill, error) {
	entries, err := fs.ReadDir(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("read embedded assets: %w", err)
	}
	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := "assets/" + e.Name()
		raw, err := assetsFS.ReadFile(path + "/SKILL.md")
		if err != nil {
			return nil, fmt.Errorf("skill %q has no SKILL.md: %w", e.Name(), err)
		}
		fm, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("skill %q: %w", e.Name(), err)
		}
		skills = append(skills, Skill{Name: fm.Name, Description: fm.Description, fsPath: path})
	}
	return skills, nil
}

// Get returns the catalog skill with the given name.
func Get(name string) (Skill, bool) {
	cat, err := Catalog()
	if err != nil {
		return Skill{}, false
	}
	for _, s := range cat {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/skills/ -run TestCatalogIntegrity -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/huly/internal/skills/catalog.go src/huly/internal/skills/catalog_test.go
git commit -m "feat(skills): embed assets and expose catalog"
```

---

### Task 5: Content hash

**Files:**
- Create: `src/huly/internal/skills/hash.go`
- Create: `src/huly/internal/skills/hash_test.go`

**Interfaces:**
- Consumes: `Split` from Task 3; `assetsFS` from Task 4 (for the embedded-tree helper).
- Produces:
  - `func ContentHash(tree fs.FS) (string, error)` — `sha256:<hex>` over the `SKILL.md` body (frontmatter excluded) plus every other file verbatim, keyed by sorted relative path.
  - `func (s Skill) contentHash() (string, error)` — `ContentHash` over the embedded skill subtree (used by later phases).

- [ ] **Step 1: Write the failing test**

Create `src/huly/internal/skills/hash_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/skills/ -run 'TestContentHash|TestSkillContentHash' -v`
Expected: FAIL — `ContentHash`/`contentHash` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `src/huly/internal/skills/hash.go`:

```go
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
		if d.Name() == "SKILL.md" {
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/skills/ -run 'TestContentHash|TestSkillContentHash' -v`
Expected: PASS.

- [ ] **Step 5: Run the whole package + module test suite**

Run: `cd src/huly && go test ./... 2>&1 | tail -20`
Expected: all packages PASS (`internal/skills`, `internal/semver`, `cmd`, and the pre-existing packages). Or run the full suite via `just go::test huly`.

- [ ] **Step 6: Commit**

```bash
git add src/huly/internal/skills/hash.go src/huly/internal/skills/hash_test.go
git commit -m "feat(skills): content hash over body + sibling files"
```

---

## Self-Review

**Spec coverage (Phase A1 scope):**
- Semver lift (§8) → Task 1. ✓ (leaf pkg, TrimPrefix baked in, `v1.9.0`/`v2.0.0` regression tested, update.go rewired)
- Seed skill asset authored first, criteria 1 & 3 (§7) → Task 2. ✓ (criterion 2 correctly deferred to Phase B, noted)
- Frontmatter split/parse/surgical stamp, quoted values, byte-preserved body (§2) → Task 3. ✓
- Embed + catalog + integrity test, dot-dir skip, empty-dir/compile caveat (§1) → Task 4. ✓
- Content hash = body + sibling files, frontmatter excluded; exclusion + invariance tests (§3) → Task 5. ✓

Not in this plan (correctly — later phases): detection/`Dirs` (A2), install state machine/backups/dir-replace (A2), cobra commands/flags/output (B), TUI (C). The plan's scope matches the spec's A1 bullet exactly.

**Placeholder scan:** No TBD/TODO; every code step shows complete code; every command shows the expected result. No "add error handling" hand-waving.

**Type consistency:** `Frontmatter{Name,Description,ManagedBy,Version,ContentHash}`, `Skill{Name,Description,fsPath}`, `Split`, `Parse`, `Stamp`, `Catalog`, `Get`, `ContentHash`, `assetsFS` are named identically wherever referenced across Tasks 2–5. `semver.Compare` matches its use in Task 1's `update.go` rewire. The seed skill's command strings (`huly project list`, `huly component create`, `huly issue create`) match both the seed test (Task 2) and the real flags in `cmd/issue.go`/`cmd/component.go`.
