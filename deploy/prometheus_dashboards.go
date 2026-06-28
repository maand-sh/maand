// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"os"
	"path"

	"maand/bucket"
	"maand/data"
	"maand/promconfig"
)

// assemblePrometheusDashboards copies catalog dashboard files to
// <prometheusJobDir>/consoles/dashboards/<job>/<rel> (not under workspace maand/).
// Mount ./consoles to /etc/prometheus/consoles; pages are served at
// /consoles/dashboards/<job>/<path> in the Prometheus UI.
func assemblePrometheusDashboards(tx *sql.Tx, prometheusJobDir string) error {
	files, err := data.ListDashboardFiles(tx)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	dashboardsDir := path.Join(prometheusJobDir, "consoles", "dashboards")
	for _, entry := range files {
		job, rel, ok := promconfig.DashboardRelFromPath(entry.Path)
		if !ok {
			continue
		}
		content, err := data.GetJobFileContent(tx, entry.Path)
		if err != nil {
			return err
		}
		dest := path.Join(dashboardsDir, job, rel)
		if err := os.MkdirAll(path.Dir(dest), 0o755); err != nil {
			return bucket.UnexpectedError(err)
		}
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			return bucket.UnexpectedError(err)
		}
	}
	return nil
}
