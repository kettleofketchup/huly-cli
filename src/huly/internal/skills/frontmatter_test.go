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

func TestStampRejectsNonMappingMetadata(t *testing.T) {
	src := []byte("---\nname: x\nmetadata: not-a-map\n---\n# Body\n")
	if _, err := Stamp(src, "huly-cli", "1.0.0", "sha256:abc"); err == nil {
		t.Fatal("expected error when metadata is not a mapping")
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
