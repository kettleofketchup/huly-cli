package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestTableRendersRows(t *testing.T) {
	var b bytes.Buffer
	Table(&b, []string{"ID", "TITLE"}, [][]string{{"PROJ-1", "Hello"}})
	out := b.String()
	if !strings.Contains(out, "ID") || !strings.Contains(out, "PROJ-1") || !strings.Contains(out, "Hello") {
		t.Fatalf("table missing content:\n%s", out)
	}
}

func TestJSONRoundTrips(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, map[string]string{"a": "1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"a": "1"`) {
		t.Fatalf("json = %s", b.String())
	}
}

func TestQuietSuppresses(t *testing.T) {
	Quiet = true
	defer func() { Quiet = false }()
	var b bytes.Buffer
	Table(&b, []string{"X"}, [][]string{{"y"}})
	_ = JSON(&b, map[string]int{"n": 1})
	if b.Len() != 0 {
		t.Fatalf("quiet should suppress, got: %s", b.String())
	}
}
