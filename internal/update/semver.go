// Package update checks GitHub for a newer Docklet release. It only reports; it never downloads.
package update

import (
	"strconv"
	"strings"
)

type semver struct {
	core [3]int
	pre  []string
}

func parse(v string) (semver, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexByte(v, '+'); i >= 0 { // build metadata does not affect ordering
		v = v[:i]
	}
	var s semver
	core, pre, hasPre := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return s, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return s, false
		}
		s.core[i] = n
	}
	if hasPre {
		if pre == "" {
			return s, false
		}
		s.pre = strings.Split(pre, ".")
	}
	return s, true
}

// Compare orders two semantic versions (leading "v" allowed): -1, 0 or 1. ok=false when either is invalid.
func Compare(a, b string) (cmp int, ok bool) {
	x, okA := parse(a)
	y, okB := parse(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range x.core {
		if x.core[i] != y.core[i] {
			return sign(x.core[i] - y.core[i]), true
		}
	}
	return comparePre(x.pre, y.pre), true
}

// comparePre: a release (no pre-release part) is newer than any pre-release of the same core.
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := compareIdent(a[i], b[i]); c != 0 {
			return c
		}
	}
	return sign(len(a) - len(b))
}

func compareIdent(a, b string) int {
	na, errA := strconv.Atoi(a)
	nb, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		return sign(na - nb)
	case errA == nil: // numeric identifiers sort before alphanumeric ones
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(a, b)
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// IsNewer reports whether latest is a strictly newer version than current.
// Development builds ("dev") and anything that is not a valid version never report an update.
func IsNewer(latest, current string) bool {
	c, ok := Compare(latest, current)
	return ok && c > 0
}
