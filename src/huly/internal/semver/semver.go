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
