// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package promconfig

import (
	"database/sql"
	"fmt"
	"path"
	"strings"

	"maand/data"
)

// RenderRuleFilesYAML returns a rule_files YAML fragment listing assembled alert paths.
// Paths are relative to the prometheus config file: rules/<job>/<file>.yaml
func RenderRuleFilesYAML(tx *sql.Tx) (string, error) {
	entries, err := data.ListPrometheusAlertFiles(tx)
	if err != nil {
		return "", err
	}
	hasPrometheusJob, err := data.JobHasPrometheusServerConfig(tx, "prometheus")
	if err != nil {
		return "", err
	}
	if len(entries) == 0 && !hasPrometheusJob {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("rule_files:\n")
	if hasPrometheusJob {
		_, err := fmt.Fprintf(&b, "  - rules/%s/%s\n", MaandAlertsJob, MaandCertAlertsFile)
		if err != nil {
			return "", err
		}
	}
	for _, entry := range entries {
		_, err := fmt.Fprintf(&b, "  - rules/%s/%s\n", entry.Job, path.Base(entry.Rel))
		if err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

// RuleFilesGlob returns a filepath.Glob pattern for maand-assembled rules (one job directory level).
func RuleFilesGlob(absolute bool) string {
	if absolute {
		return "/etc/prometheus/rules/*/*.yaml"
	}
	return "rules/*/*.yaml"
}
