package cmd

import (
	"path/filepath"
	"testing"
)

func TestStagingPathSameDir(t *testing.T) {
	target := filepath.FromSlash("/opt/tools/bin/huly")
	got := stagingPath(target)
	if filepath.Dir(got) != filepath.Dir(target) {
		t.Fatalf("staging path %q not in target dir %q", got, filepath.Dir(target))
	}
	if got == target {
		t.Fatal("staging path must differ from target")
	}
}
