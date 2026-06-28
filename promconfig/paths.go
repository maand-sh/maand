// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package promconfig

import (
	"os"
	"path"
	"strings"

	"maand/workspace"
)

const (
	KVNamespace = "maand/prometheus"

	ScrapeFileName    = "scrape.yaml"
	ScrapeFileTplName = "scrape.yaml.tpl"
	AlertsDir      = "alerts"
	RunbooksDir    = "runbooks"
	DashboardsDir  = "dashboards"

	PortPlaceholderPrefix = "maand:port/"
	JobPlaceholder        = "maand:job"
	ImplicitPortKey       = "_implicit"
)

// JobPrometheusDir returns workspace/jobs/<job>/_prometheus.
func JobPrometheusDir(jobName string) string {
	return path.Join(workspace.JobFilePath(jobName), "_prometheus")
}

// JobScrapePath returns workspace/jobs/<job>/_prometheus/scrape.yaml.
func JobScrapePath(jobName string) string {
	return path.Join(JobPrometheusDir(jobName), ScrapeFileName)
}

// JobScrapeTplPath returns workspace/jobs/<job>/_prometheus/scrape.yaml.tpl.
func JobScrapeTplPath(jobName string) string {
	return path.Join(JobPrometheusDir(jobName), ScrapeFileTplName)
}

// HasScrapeWorkspaceFile reports whether scrape.yaml or scrape.yaml.tpl exists.
func HasScrapeWorkspaceFile(jobName string) bool {
	for _, scrapePath := range []string{JobScrapePath(jobName), JobScrapeTplPath(jobName)} {
		if _, err := os.Stat(scrapePath); err == nil {
			return true
		}
	}
	return false
}

// IsPrometheusWorkspacePath reports whether a job_files path is under _prometheus/.
func IsPrometheusWorkspacePath(filePath string) bool {
	return strings.Contains(filePath, "/_prometheus/")
}

// AlertRelPath returns the path under _prometheus/ for an alert file job path.
func AlertRelPath(jobFilePath string) (string, bool) {
	const marker = "/_prometheus/alerts/"
	idx := strings.Index(jobFilePath, marker)
	if idx < 0 {
		return "", false
	}
	return jobFilePath[idx+len("/_prometheus/"):], true
}

// RunbookSlugFromPath returns the runbook slug from a job_files path.
func RunbookSlugFromPath(jobFilePath string) (job, slug string, ok bool) {
	const marker = "/_prometheus/runbooks/"
	idx := strings.Index(jobFilePath, marker)
	if idx < 0 {
		return "", "", false
	}
	job = jobFilePath[:idx]
	base := jobFilePath[idx+len(marker):]
	if !strings.HasSuffix(base, ".md") {
		return "", "", false
	}
	return job, strings.TrimSuffix(base, ".md"), true
}

// DashboardRelFromPath returns the job name and path under dashboards/ from a job_files path.
func DashboardRelFromPath(jobFilePath string) (job, rel string, ok bool) {
	const marker = "/_prometheus/dashboards/"
	idx := strings.Index(jobFilePath, marker)
	if idx < 0 {
		return "", "", false
	}
	job = jobFilePath[:idx]
	rel = jobFilePath[idx+len(marker):]
	if rel == "" {
		return "", "", false
	}
	return job, rel, true
}
