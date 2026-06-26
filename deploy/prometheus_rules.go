// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"os"
	"path"

	"maand/bucket"
	"maand/data"
	"maand/promconfig"
)

func assemblePrometheusAlertRules(tx *sql.Tx, prometheusJobDir, workerIP string) error {
	port, err := data.GetJobPortNumber(tx, "prometheus", "prometheus_port_http")
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("http://%s:%d", workerIP, port)

	entries, err := data.ListPrometheusAlertFiles(tx)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		content, err := data.GetJobFileContent(tx, entry.Path)
		if err != nil {
			return err
		}
		contentBytes, err := promconfig.InjectRunbookURLs(entry.Job, []byte(content), baseURL)
		if err != nil {
			return err
		}
		dest := path.Join(prometheusJobDir, "rules", entry.Job, path.Base(entry.Rel))
		if err := os.MkdirAll(path.Dir(dest), 0o755); err != nil {
			return bucket.UnexpectedError(err)
		}
		if err := os.WriteFile(dest, contentBytes, 0o644); err != nil {
			return bucket.UnexpectedError(err)
		}
	}
	return nil
}
