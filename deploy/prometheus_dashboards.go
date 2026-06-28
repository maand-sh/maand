// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"maand/bucket"
	"maand/dashboards"
	"maand/data"
	"maand/promconfig"
	"maand/runbooks"
	"maand/workspace"
)

// assemblePrometheusDashboards copies catalog dashboard files to
// <prometheusJobDir>/consoles/dashboards/<job>/<rel> (not under workspace maand/).
// Mount ./consoles to /etc/prometheus/consoles; pages are served at
// /consoles/dashboards/<job>/<path> in the Prometheus UI.
//
// Files matching _prometheus/dashboards/.dashboardignore patterns are still copied
// but omitted from dashboards/index.html.
func assemblePrometheusDashboards(tx *sql.Tx, prometheusJobDir string) error {
	files, err := data.ListDashboardFiles(tx)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	ignoreByJob, err := loadDashboardIgnorePatterns(tx, files)
	if err != nil {
		return err
	}

	dashboardsDir := path.Join(prometheusJobDir, "consoles", "dashboards")
	if err := os.MkdirAll(dashboardsDir, 0o755); err != nil {
		return bucket.UnexpectedError(err)
	}
	if err := os.WriteFile(path.Join(dashboardsDir, "style.css"), runbooks.StyleCSS, 0o644); err != nil {
		return bucket.UnexpectedError(err)
	}

	entries := make([]dashboards.Entry, 0, len(files))
	for _, entry := range files {
		job, rel, ok := promconfig.DashboardRelFromPath(entry.Path)
		if !ok {
			continue
		}
		if rel == dashboards.IgnoreFileName {
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
		if dashboards.ShouldListInIndex(rel, ignoreByJob[job]) {
			entries = append(entries, dashboards.Entry{
				Job:   job,
				Rel:   rel,
				Label: dashboards.LinkLabel(rel, content),
			})
		}
	}

	if err := removeDeployedDashboardMetaFiles(dashboardsDir); err != nil {
		return err
	}

	indexHTML := dashboards.RenderIndexHTML(entries)
	if err := os.WriteFile(path.Join(dashboardsDir, "index.html"), []byte(indexHTML), 0o644); err != nil {
		return bucket.UnexpectedError(err)
	}
	return nil
}

func loadDashboardIgnorePatterns(tx *sql.Tx, files []data.PrometheusDashboardFileEntry) (map[string][]string, error) {
	patterns := make(map[string][]string)
	jobs := make(map[string]struct{})

	for _, entry := range files {
		job, _, ok := promconfig.DashboardRelFromPath(entry.Path)
		if !ok {
			continue
		}
		jobs[job] = struct{}{}
	}

	for job := range jobs {
		jobPatterns, err := loadJobDashboardIgnorePatterns(tx, job)
		if err != nil {
			return nil, err
		}
		patterns[job] = jobPatterns
	}
	return patterns, nil
}

func loadJobDashboardIgnorePatterns(tx *sql.Tx, job string) ([]string, error) {
	wsPath := workspace.JobFilePath(path.Join(job, "_prometheus", promconfig.DashboardsDir, dashboards.IgnoreFileName))
	if content, err := os.ReadFile(wsPath); err == nil {
		return dashboards.ParseIgnoreFile(string(content)), nil
	} else if !os.IsNotExist(err) {
		return nil, bucket.UnexpectedError(err)
	}

	catalogPath := path.Join(job, "_prometheus", promconfig.DashboardsDir, dashboards.IgnoreFileName)
	content, err := data.GetJobFileContent(tx, catalogPath)
	if err != nil {
		if errors.Is(err, bucket.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return dashboards.ParseIgnoreFile(content), nil
}

func removeDeployedDashboardMetaFiles(dashboardsDir string) error {
	return filepath.WalkDir(dashboardsDir, func(absPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == dashboards.IgnoreFileName {
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				return bucket.UnexpectedError(err)
			}
		}
		return nil
	})
}
