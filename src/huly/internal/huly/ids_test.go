package huly

import "testing"

func TestNewRefUniqueAndLength(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 1000; i++ {
		id := NewRef()
		if len(id) < 24 {
			t.Fatalf("ref too short: %q (len %d)", id, len(id))
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ref: %q", id)
		}
		seen[id] = struct{}{}
	}
}
