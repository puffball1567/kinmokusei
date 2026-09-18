package project

import "strings"

// Compare complete versions (and Go's major.minor spelling), with prereleases
// ordered before the corresponding release. Numeric components never overflow.
func comparePackageVersions(a, b string) int {
	split := func(s string) ([]string, []string) {
		s = strings.TrimPrefix(s, "v")
		s, _, _ = strings.Cut(s, "+")
		main, pre, _ := strings.Cut(s, "-")
		var suffix []string
		if pre != "" {
			suffix = strings.Split(pre, ".")
		}
		return strings.Split(main, "."), suffix
	}
	number := func(a, b string) int {
		a = strings.TrimLeft(a, "0")
		b = strings.TrimLeft(b, "0")
		if len(a) < len(b) {
			return -1
		}
		if len(a) > len(b) {
			return 1
		}
		return strings.Compare(a, b)
	}
	x, xp := split(a)
	y, yp := split(b)
	for i := 0; i < len(x) || i < len(y); i++ {
		left, right := "0", "0"
		if i < len(x) {
			left = x[i]
		}
		if i < len(y) {
			right = y[i]
		}
		if cmp := number(left, right); cmp != 0 {
			return cmp
		}
	}
	if len(xp) == 0 && len(yp) != 0 {
		return 1
	}
	if len(yp) == 0 && len(xp) != 0 {
		return -1
	}
	numeric := func(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }
	for i := 0; i < len(xp) && i < len(yp); i++ {
		var cmp int
		switch {
		case numeric(xp[i]) && numeric(yp[i]):
			cmp = number(xp[i], yp[i])
		case numeric(xp[i]):
			cmp = -1
		case numeric(yp[i]):
			cmp = 1
		default:
			cmp = strings.Compare(xp[i], yp[i])
		}
		if cmp != 0 {
			return cmp
		}
	}
	if len(xp) < len(yp) {
		return -1
	}
	if len(xp) > len(yp) {
		return 1
	}
	return 0
}
