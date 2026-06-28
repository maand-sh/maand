// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dashboards

import (
	"path"
	"path/filepath"
	"strings"

	"maand/utils/pathmatch"
)

// IgnoreFileName is the gitignore-style file under _prometheus/dashboards/.
// Listed patterns omit files from dashboards/index.html only; deploy still copies them.
const IgnoreFileName = ".dashboardignore"

// ParseIgnoreFile parses .dashboardignore lines into patterns.
func ParseIgnoreFile(content string) []string {
	patterns := make([]string, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// IsDashboardMetaFile reports paths that must never appear in dashboards/index.html.
func IsDashboardMetaFile(rel string) bool {
	base := path.Base(filepath.ToSlash(rel))
	return base == IgnoreFileName || strings.HasPrefix(base, ".")
}

// ShouldListInIndex reports whether rel belongs on dashboards/index.html.
func ShouldListInIndex(rel string, patterns []string) bool {
	if IsDashboardMetaFile(rel) {
		return false
	}
	return !IgnoredFromIndex(rel, patterns)
}

// IgnoredFromIndex reports whether rel (path under dashboards/) should be omitted from index.html.
func IgnoredFromIndex(rel string, patterns []string) bool {
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "/"))
	if rel == "" || IsDashboardMetaFile(rel) {
		return true
	}
	for _, pattern := range patterns {
		if pathmatch.Match(pattern, rel) {
			return true
		}
	}
	return false
}
