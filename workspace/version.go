// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"fmt"
	"strconv"
	"strings"

	"maand/bucket"
)

// Version is a semver-like job version (major.minor.patch).
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
}

// ParseVersion parses major.minor.patch with an optional -prerelease suffix.
// A leading "v" is ignored. Missing segments default to zero ("1" → 1.0.0).
func ParseVersion(raw string) (Version, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Version{}, fmt.Errorf("%w: version is empty", bucket.ErrInvalidJobVersion)
	}
	if strings.EqualFold(raw, "unknown") {
		return Version{}, fmt.Errorf("%w: version %q is not allowed", bucket.ErrInvalidJobVersion, raw)
	}
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "v"), "V")

	prerelease := ""
	if idx := strings.Index(raw, "-"); idx >= 0 {
		prerelease = strings.TrimSpace(raw[idx+1:])
		raw = raw[:idx]
		if prerelease == "" {
			return Version{}, fmt.Errorf("%w: empty prerelease suffix", bucket.ErrInvalidJobVersion)
		}
	}

	parts := strings.Split(raw, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, fmt.Errorf("%w: %q", bucket.ErrInvalidJobVersion, raw)
	}

	numbers := make([]int, 3)
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return Version{}, fmt.Errorf("%w: empty segment in %q", bucket.ErrInvalidJobVersion, raw)
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("%w: invalid segment %q in %q", bucket.ErrInvalidJobVersion, part, raw)
		}
		numbers[i] = n
	}

	return Version{
		Major:      numbers[0],
		Minor:      numbers[1],
		Patch:      numbers[2],
		Prerelease: prerelease,
	}, nil
}

// String returns major.minor.patch with optional prerelease.
func (v Version) String() string {
	if v.Prerelease != "" {
		return fmt.Sprintf("%d.%d.%d-%s", v.Major, v.Minor, v.Patch, v.Prerelease)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns -1, 0, or 1 comparing release segments only (prerelease < release).
func (v Version) Compare(other Version) int {
	for _, pair := range []struct{ a, b int }{
		{v.Major, other.Major},
		{v.Minor, other.Minor},
		{v.Patch, other.Patch},
	} {
		switch {
		case pair.a < pair.b:
			return -1
		case pair.a > pair.b:
			return 1
		}
	}

	vPre := v.Prerelease != ""
	oPre := other.Prerelease != ""
	switch {
	case vPre && !oPre:
		return -1
	case !vPre && oPre:
		return 1
	case vPre && oPre:
		if v.Prerelease < other.Prerelease {
			return -1
		}
		if v.Prerelease > other.Prerelease {
			return 1
		}
	}
	return 0
}

// JobVersionParsed returns the manifest version, defaulting empty to 0.0.0.
func (m Manifest) JobVersionParsed() (Version, error) {
	raw := strings.TrimSpace(m.Version)
	if raw == "" {
		return Version{}, nil
	}
	return ParseVersion(raw)
}

// RequiresExplicitVersion reports whether the job must declare a non-empty version string.
func (m Manifest) RequiresExplicitVersion() bool {
	for _, command := range m.ListedCommands() {
		if strings.TrimSpace(command.Demands.Job) != "" {
			return true
		}
	}
	return false
}
