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
)

func assemblePrometheusAlertRules(tx *sql.Tx, prometheusJobDir string) error {
	entries, err := data.ListPrometheusAlertFiles(tx)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		content, err := data.GetJobFileContent(tx, entry.Path)
		if err != nil {
			return err
		}
		dest := path.Join(prometheusJobDir, "rules", entry.Job, path.Base(entry.Rel))
		if err := os.MkdirAll(path.Dir(dest), 0o755); err != nil {
			return bucket.UnexpectedError(err)
		}
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			return bucket.UnexpectedError(err)
		}
	}
	return nil
}
