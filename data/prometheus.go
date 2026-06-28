// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"maand/bucket"
)

const prometheusAlertsMarker = "/_prometheus/alerts/"

// PrometheusJobSummary describes _prometheus/ participation for one job.
type PrometheusJobSummary struct {
	Job        string
	Scrape     bool
	Alerts     int
	Runbooks   int
	Dashboards int
}

// PrometheusFileEntry is one file stored under job/_prometheus/ in job_files.
type PrometheusFileEntry struct {
	Job  string
	Path string
	Rel  string
}

// ListPrometheusJobSummaries returns scrape/alerts/runbooks counts per job from job_files.
func ListPrometheusJobSummaries(tx *sql.Tx, jobsFilter []string) ([]PrometheusJobSummary, error) {
	entries, err := listPrometheusFiles(tx, jobsFilter)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	byJob := make(map[string]*PrometheusJobSummary)
	for _, entry := range entries {
		summary, ok := byJob[entry.Job]
		if !ok {
			summary = &PrometheusJobSummary{Job: entry.Job}
			byJob[entry.Job] = summary
		}
		switch {
		case entry.Rel == "scrape.yaml", entry.Rel == "scrape.yaml.tpl":
			summary.Scrape = true
		case strings.HasPrefix(entry.Rel, "alerts/"):
			summary.Alerts++
		case strings.HasPrefix(entry.Rel, "runbooks/"):
			summary.Runbooks++
		case strings.HasPrefix(entry.Rel, "dashboards/"):
			summary.Dashboards++
		}
	}

	out := make([]PrometheusJobSummary, 0, len(byJob))
	for _, summary := range byJob {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Job < out[j].Job })
	return out, nil
}

// ListPrometheusFiles returns _prometheus/ files from job_files, optionally filtered by job.
func ListPrometheusFiles(tx *sql.Tx, jobsFilter []string) ([]PrometheusFileEntry, error) {
	return listPrometheusFiles(tx, jobsFilter)
}

func listPrometheusFiles(tx *sql.Tx, jobsFilter []string) ([]PrometheusFileEntry, error) {
	query := `
		SELECT j.name, jf.path
		FROM job_files jf
		JOIN job j ON j.job_id = jf.job_id
		WHERE jf.isdir = 0 AND jf.path LIKE '%/_prometheus/%'
	`
	args := make([]interface{}, 0)
	if len(jobsFilter) > 0 {
		placeholders := make([]string, len(jobsFilter))
		for i, job := range jobsFilter {
			placeholders[i] = "?"
			args = append(args, job)
		}
		query += fmt.Sprintf(" AND j.name IN (%s)", strings.Join(placeholders, ","))
	}
	query += " ORDER BY j.name, jf.path"

	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	entries := make([]PrometheusFileEntry, 0)
	for rows.Next() {
		var jobName, filePath string
		if err := rows.Scan(&jobName, &filePath); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		idx := strings.Index(filePath, "/_prometheus/")
		if idx < 0 {
			continue
		}
		entries = append(entries, PrometheusFileEntry{
			Job:  jobName,
			Path: filePath,
			Rel:  filePath[idx+len("/_prometheus/"):],
		})
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetPrometheusFileContent returns one file under job/_prometheus/ by relative path.
func GetPrometheusFileContent(tx *sql.Tx, job, relPath string) (string, error) {
	relPath = normalizePrometheusRelPath(relPath)
	if relPath == "" {
		return "", bucket.NotFoundError("prometheus file")
	}
	filePath := job + "/_prometheus/" + relPath
	return GetJobFileContent(tx, filePath)
}

func normalizePrometheusRelPath(relPath string) string {
	relPath = strings.TrimPrefix(strings.TrimSpace(relPath), "/")
	switch relPath {
	case "", "scrape":
		return "scrape.yaml"
	default:
		return relPath
	}
}

// PrometheusAlertFileEntry indexes one alert YAML stored in job_files.
type PrometheusAlertFileEntry struct {
	Job  string
	Path string
	Rel  string
}

// JobHasPrometheusServerConfig reports whether the job ships prometheus.yml or prometheus.yml.tpl.
func JobHasPrometheusServerConfig(tx *sql.Tx, jobName string) (bool, error) {
	count := 0
	err := tx.QueryRow(
		`SELECT count(*) FROM job_files
		 WHERE job_id = (SELECT job_id FROM job WHERE name = ?)
		   AND isdir = 0
		   AND (path = ? OR path = ?)`,
		jobName,
		jobName+"/prometheus.yml",
		jobName+"/prometheus.yml.tpl",
	).Scan(&count)
	if err != nil {
		return false, bucket.DatabaseError(err)
	}
	return count > 0, nil
}

// ListPrometheusAlertFiles returns alert files from job_files.
func ListPrometheusAlertFiles(tx *sql.Tx) ([]PrometheusAlertFileEntry, error) {
	rows, err := tx.Query(
		`SELECT j.name, jf.path
		 FROM job_files jf
		 JOIN job j ON j.job_id = jf.job_id
		 WHERE jf.isdir = 0 AND jf.path LIKE '%/_prometheus/alerts/%'
		   AND (jf.path LIKE '%.yaml' OR jf.path LIKE '%.yml')
		 ORDER BY j.name, jf.path`,
	)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	entries := make([]PrometheusAlertFileEntry, 0)
	for rows.Next() {
		var jobName, filePath string
		if err := rows.Scan(&jobName, &filePath); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		idx := strings.Index(filePath, prometheusAlertsMarker)
		if idx < 0 {
			continue
		}
		entries = append(entries, PrometheusAlertFileEntry{
			Job:  jobName,
			Path: filePath,
			Rel:  filePath[idx+len("/_prometheus/"):],
		})
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetJobFileContent returns one file from job_files by exact path.
func GetJobFileContent(tx *sql.Tx, filePath string) (string, error) {
	var content string
	err := tx.QueryRow(
		`SELECT content FROM job_files WHERE path = ? AND isdir = 0`,
		filePath,
	).Scan(&content)
	if err == sql.ErrNoRows {
		return "", bucket.NotFoundError(filePath)
	}
	if err != nil {
		return "", bucket.DatabaseError(err)
	}
	return content, nil
}

// ListRunbookFiles returns all runbook markdown paths in the catalog.
func ListRunbookFiles(tx *sql.Tx) ([]string, error) {
	rows, err := tx.Query(
		`SELECT path FROM job_files
		 WHERE isdir = 0 AND path LIKE '%/_prometheus/runbooks/%.md'
		 ORDER BY path`,
	)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	paths := make([]string, 0)
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		paths = append(paths, filePath)
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return paths, nil
}

// PrometheusDashboardFileEntry indexes one dashboard file stored in job_files.
type PrometheusDashboardFileEntry struct {
	Path string
}

// ListDashboardFiles returns all dashboard file paths in the catalog.
func ListDashboardFiles(tx *sql.Tx) ([]PrometheusDashboardFileEntry, error) {
	rows, err := tx.Query(
		`SELECT path FROM job_files
		 WHERE isdir = 0 AND path LIKE '%/_prometheus/dashboards/%'
		 ORDER BY path`,
	)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	paths := make([]PrometheusDashboardFileEntry, 0)
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		paths = append(paths, PrometheusDashboardFileEntry{Path: filePath})
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return paths, nil
}
