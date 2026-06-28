// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"encoding/json"
	"fmt"
	"strings"

	"maand/bucket"
	"maand/utils/pathmatch"
)

const (
	RestartPolicyAlways = "always"
	RestartPolicyReload = "reload"
	RestartPolicyNever  = "never"
)

// EffectiveRestartPolicy returns the manifest restart_policy, defaulting to always.
func (m Manifest) EffectiveRestartPolicy() string {
	policy, err := NormalizeRestartPolicy(m.RestartPolicy)
	if err != nil {
		return RestartPolicyAlways
	}
	return policy
}

// NormalizeRestartPolicy validates restart_policy and applies the default.
func NormalizeRestartPolicy(raw string) (string, error) {
	if raw == "" {
		return RestartPolicyAlways, nil
	}
	switch raw {
	case RestartPolicyAlways, RestartPolicyReload, RestartPolicyNever:
		return raw, nil
	default:
		return "", fmt.Errorf(
			"%w: restart_policy must be one of always, reload, never (got %q)",
			bucket.ErrInvalidManifest, raw,
		)
	}
}

// ParseRestartGlobs decodes restart_globs stored on the job row.
func ParseRestartGlobs(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var globs []string
	if err := json.Unmarshal([]byte(raw), &globs); err != nil {
		return nil, fmt.Errorf("%w: invalid restart_globs JSON: %w", bucket.ErrInvalidManifest, err)
	}
	return globs, nil
}

// EncodeRestartGlobs serializes restart_globs for the job row.
func EncodeRestartGlobs(globs []string) (string, error) {
	if len(globs) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(globs)
	if err != nil {
		return "", fmt.Errorf("%w: restart_globs: %w", bucket.ErrInvalidManifest, err)
	}
	return string(encoded), nil
}

// ValidateRestartPolicy validates restart_policy and restart_globs on a job manifest.
func ValidateRestartPolicy(jobName string, manifest Manifest) error {
	if _, err := NormalizeRestartPolicy(manifest.RestartPolicy); err != nil {
		return fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, jobName, err)
	}
	return ValidateRestartGlobs(jobName, manifest)
}

// ValidateRestartGlobs validates restart_globs on a job manifest.
func ValidateRestartGlobs(jobName string, manifest Manifest) error {
	if len(manifest.RestartGlobs) == 0 {
		return nil
	}

	policy, err := NormalizeRestartPolicy(manifest.RestartPolicy)
	if err != nil {
		return fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, jobName, err)
	}
	if policy != RestartPolicyReload {
		return fmt.Errorf(
			"%w: job %s restart_globs requires restart_policy reload",
			bucket.ErrInvalidManifest, jobName,
		)
	}

	for _, pattern := range manifest.RestartGlobs {
		if err := pathmatch.ValidatePattern(pattern); err != nil {
			return fmt.Errorf("%w: job %s restart_globs: %w", bucket.ErrInvalidManifest, jobName, err)
		}
	}
	return nil
}
