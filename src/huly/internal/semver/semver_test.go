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
		{"garbage", "0.0.0", 0},        // unparseable -> [0,0,0]
		{"v1.2.3-4-gabc", "v1.2.3", 0}, // git-describe suffix truncated after patch
		{"1.2.0", "1.3.0", -1},         // equal major: minor decides
		{"1.3.0", "1.2.0", 1},
		{"1.2.3", "1.2.4", -1}, // equal major+minor: patch decides
		{"1.2.4", "1.2.3", 1},
		{"2.0.0", "1.9.9", 1}, // major dominates a larger minor/patch
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}
