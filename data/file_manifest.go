// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"encoding/json"
	"sort"

	"maand/bucket"
)

// FileManifest maps job-relative paths to content MD5 hex digests.
type FileManifest map[string]string

// ParseFileManifest decodes a stored JSON manifest; NULL/empty means no manifest stored.
func ParseFileManifest(raw sql.NullString) (FileManifest, bool, error) {
	if !raw.Valid || raw.String == "" {
		return nil, false, nil
	}
	var manifest FileManifest
	if err := json.Unmarshal([]byte(raw.String), &manifest); err != nil {
		return nil, false, bucket.UnexpectedError(err)
	}
	if manifest == nil {
		manifest = FileManifest{}
	}
	return manifest, true, nil
}

// Encode serializes a manifest to stable JSON (sorted keys).
func (m FileManifest) Encode() (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(keys))
	for _, key := range keys {
		ordered[key] = m[key]
	}
	encoded, err := json.Marshal(ordered)
	if err != nil {
		return "", bucket.UnexpectedError(err)
	}
	return string(encoded), nil
}

// DiffFileManifests returns added, modified, and deleted paths between manifests.
func DiffFileManifests(previous, current FileManifest) []string {
	changed := make([]string, 0)
	seen := make(map[string]struct{})

	for path, hash := range current {
		prevHash, ok := previous[path]
		if !ok || prevHash != hash {
			changed = append(changed, path)
			seen[path] = struct{}{}
		}
	}
	for path := range previous {
		if _, ok := current[path]; !ok {
			if _, listed := seen[path]; !listed {
				changed = append(changed, path)
			}
		}
	}
	sort.Strings(changed)
	return changed
}
