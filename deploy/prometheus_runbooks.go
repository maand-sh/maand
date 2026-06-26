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
	"maand/runbooks"
)

// assemblePrometheusRunbooks renders catalog runbooks to HTML under
// <prometheusJobDir>/consoles/runbooks/<job>/<slug>.html (not under workspace maand/).
// Mount ./consoles to /etc/prometheus/consoles; pages are served at
// /consoles/runbooks/<job>/<slug>.html in the Prometheus UI.
func assemblePrometheusRunbooks(tx *sql.Tx, prometheusJobDir string) error {
	runbooksDir := path.Join(prometheusJobDir, "consoles", "runbooks")
	paths, err := data.ListRunbookFiles(tx)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}

	if err := os.MkdirAll(runbooksDir, 0o755); err != nil {
		return bucket.UnexpectedError(err)
	}
	if err := os.WriteFile(path.Join(runbooksDir, "style.css"), runbooks.StyleCSS, 0o644); err != nil {
		return bucket.UnexpectedError(err)
	}

	entries := make([]runbooks.Entry, 0, len(paths))
	for _, filePath := range paths {
		job, slug, ok := promconfig.RunbookSlugFromPath(filePath)
		if !ok {
			continue
		}
		content, err := data.GetJobFileContent(tx, filePath)
		if err != nil {
			return err
		}
		htmlPage, err := runbooks.RenderRunbookHTML(job, slug, content)
		if err != nil {
			return err
		}
		dest := path.Join(runbooksDir, job, slug+".html")
		if err := os.MkdirAll(path.Dir(dest), 0o755); err != nil {
			return bucket.UnexpectedError(err)
		}
		if err := os.WriteFile(dest, []byte(htmlPage), 0o644); err != nil {
			return bucket.UnexpectedError(err)
		}
		entries = append(entries, runbooks.Entry{Job: job, Slug: slug})
	}

	indexHTML := runbooks.RenderIndexHTML(entries)
	if err := os.WriteFile(path.Join(runbooksDir, "index.html"), []byte(indexHTML), 0o644); err != nil {
		return bucket.UnexpectedError(err)
	}
	return nil
}
