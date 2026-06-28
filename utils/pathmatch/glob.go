// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package pathmatch

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"maand/bucket"
)

// ValidatePattern reports whether pattern is a valid job-relative glob.
func ValidatePattern(pattern string) error {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return fmt.Errorf("%w: empty glob pattern", bucket.ErrInvalidManifest)
	}
	if strings.Contains(pattern, "**") {
		if err := probeDoubleStar(pattern); err != nil {
			return err
		}
		return nil
	}
	if _, err := path.Match(pattern, "probe"); err != nil {
		return fmt.Errorf("%w: invalid glob pattern %q: %w", bucket.ErrInvalidManifest, pattern, err)
	}
	return nil
}

func probeDoubleStar(pattern string) error {
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if strings.Contains(suffix, "/") {
			parts := strings.SplitN(suffix, "/", 2)
			if _, err := path.Match(parts[1], "probe"); err != nil {
				return fmt.Errorf("%w: invalid glob pattern %q: %w", bucket.ErrInvalidManifest, pattern, err)
			}
			return nil
		}
		if _, err := path.Match(suffix, "probe"); err != nil {
			return fmt.Errorf("%w: invalid glob pattern %q: %w", bucket.ErrInvalidManifest, pattern, err)
		}
		return nil
	}
	if strings.HasSuffix(pattern, "/**") {
		return nil
	}
	return nil
}

// Match reports whether rel (job-relative, forward slashes) matches pattern.
func Match(pattern, rel string) bool {
	return matchPattern(pattern, rel)
}

func matchPattern(pattern, rel string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "/"))
	if pattern == "" {
		return false
	}

	if isLiteralPattern(pattern) {
		if pattern == rel || pattern == path.Base(rel) {
			return true
		}
	}

	if strings.Contains(pattern, "**") {
		return matchDoubleStarPattern(pattern, rel)
	}

	if strings.Contains(pattern, "/") {
		if matched, _ := path.Match(pattern, rel); matched {
			return true
		}
		dir := strings.TrimSuffix(pattern, "/")
		if dir != pattern {
			return rel == dir || strings.HasPrefix(rel, dir+"/")
		}
		return false
	}

	base := path.Base(rel)
	if matched, _ := path.Match(pattern, base); matched {
		return true
	}
	if matched, _ := path.Match(pattern, rel); matched {
		return true
	}
	return false
}

func matchDoubleStarPattern(pattern, rel string) bool {
	pattern = filepath.ToSlash(pattern)
	rel = filepath.ToSlash(rel)

	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if strings.Contains(suffix, "/") {
			parts := strings.SplitN(suffix, "/", 2)
			if len(parts) != 2 {
				return false
			}
			dir, name := parts[0], parts[1]
			if !strings.HasPrefix(rel, dir+"/") {
				return false
			}
			matched, _ := path.Match(name, rel[len(dir)+1:])
			return matched
		}
		matched, _ := path.Match(suffix, path.Base(rel))
		if matched {
			return true
		}
		matched, _ = path.Match(suffix, rel)
		return matched
	}

	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return rel == prefix || strings.HasPrefix(rel, prefix+"/")
	}

	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return false
	}
	prefix, suffix := parts[0], parts[1]
	if prefix != "" && !strings.HasPrefix(rel, prefix) {
		return false
	}
	rest := strings.TrimPrefix(rel, prefix)
	rest = strings.TrimPrefix(rest, "/")
	if suffix == "" {
		return true
	}
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		return true
	}
	matched, _ := path.Match(suffix, rest)
	if matched {
		return true
	}
	matched, _ = path.Match(suffix, path.Base(rest))
	return matched
}

func isLiteralPattern(pattern string) bool {
	return !strings.ContainsAny(pattern, "*?[")
}

// MatchAny reports paths from rels that match any pattern (sorted, unique).
func MatchAny(patterns, rels []string) []string {
	if len(patterns) == 0 || len(rels) == 0 {
		return nil
	}
	matched := make([]string, 0)
	seen := make(map[string]struct{}, len(rels))
	for _, rel := range rels {
		rel = filepath.ToSlash(rel)
		for _, pattern := range patterns {
			if Match(pattern, rel) {
				if _, ok := seen[rel]; !ok {
					seen[rel] = struct{}{}
					matched = append(matched, rel)
				}
				break
			}
		}
	}
	return matched
}
