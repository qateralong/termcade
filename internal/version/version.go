// Package version holds the build version and compares semantic versions.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is the running build's version without a leading "v". It is set at
// build time:
//
//	go build -ldflags "-X termcade/internal/version.Version=1.2.3"
//
// Local builds report "dev", which counts as older than any release.
var Version = "dev"

// String returns the version in display form, e.g. "v1.2.3" or "dev".
func String() string {
	if IsDev(Version) {
		return "dev"
	}
	return "v" + strings.TrimPrefix(Version, "v")
}

// IsDev reports whether v is not a release version.
func IsDev(v string) bool {
	_, err := Parse(v)
	return err != nil
}

// Semver is a parsed MAJOR.MINOR.PATCH version.
type Semver [3]int

// Parse parses "1.2.3" or "v1.2.3". Pre-release and build suffixes are not
// supported; releases are plain numbers.
func Parse(s string) (Semver, error) {
	var v Semver
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(s), "v"), ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("invalid version %q", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || (len(p) > 1 && p[0] == '0') {
			return v, fmt.Errorf("invalid version %q", s)
		}
		v[i] = n
	}
	return v, nil
}

// String formats the version as "v1.2.3".
func (v Semver) String() string { return fmt.Sprintf("v%d.%d.%d", v[0], v[1], v[2]) }

// Compare returns -1, 0 or 1 when a is older than, equal to or newer than b.
func Compare(a, b Semver) int {
	for i := range a {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

// Newer reports whether candidate is newer than current. A dev build is
// older than every release.
func Newer(candidate, current string) bool {
	c, err := Parse(candidate)
	if err != nil {
		return false
	}
	cur, err := Parse(current)
	if err != nil {
		return true
	}
	return Compare(c, cur) > 0
}
