// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package promconfig

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/workspace"
)

// JobHasWorkspacePrometheus reports whether workspace/jobs/<job>/_prometheus exists.
func JobHasWorkspacePrometheus(jobName string) bool {
	_, err := os.Stat(JobPrometheusDir(jobName))
	return err == nil
}

// ListWorkspacePrometheusSummaries scans workspace/jobs/*/_prometheus on disk.
func ListWorkspacePrometheusSummaries(jobsFilter []string) ([]data.PrometheusJobSummary, error) {
	jobNames, err := ListWorkspaceJobNames()
	if err != nil {
		return nil, err
	}
	filter := make(map[string]struct{}, len(jobsFilter))
	for _, job := range jobsFilter {
		filter[job] = struct{}{}
	}

	out := make([]data.PrometheusJobSummary, 0)
	for _, jobName := range jobNames {
		if len(filter) > 0 {
			if _, ok := filter[jobName]; !ok {
				continue
			}
		}
		summary, ok, err := workspacePrometheusSummary(jobName)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, summary)
		}
	}
	return out, nil
}

func workspacePrometheusSummary(jobName string) (data.PrometheusJobSummary, bool, error) {
	promDir := JobPrometheusDir(jobName)
	if _, err := os.Stat(promDir); err != nil {
		if os.IsNotExist(err) {
			return data.PrometheusJobSummary{}, false, nil
		}
		return data.PrometheusJobSummary{}, false, err
	}

	summary := data.PrometheusJobSummary{Job: jobName}
	if HasScrapeWorkspaceFile(jobName) {
		summary.Scrape = true
	}

	alertPaths, err := ListAlertFiles(jobName)
	if err != nil {
		return data.PrometheusJobSummary{}, false, err
	}
	summary.Alerts = len(alertPaths)

	runbookSlugs, err := ListRunbookSlugs(jobName)
	if err != nil {
		return data.PrometheusJobSummary{}, false, err
	}
	summary.Runbooks = len(runbookSlugs)

	dashboardFiles, err := ListDashboardFiles(jobName)
	if err != nil {
		return data.PrometheusJobSummary{}, false, err
	}
	summary.Dashboards = len(dashboardFiles)

	if !summary.Scrape && summary.Alerts == 0 && summary.Runbooks == 0 && summary.Dashboards == 0 {
		return data.PrometheusJobSummary{}, false, nil
	}
	return summary, true, nil
}

// ReadWorkspacePrometheusFile reads job/_prometheus/<relPath> from the workspace.
func ReadWorkspacePrometheusFile(job, relPath string) (string, error) {
	relPath = NormalizePrometheusRelPath(relPath)
	if relPath == "" {
		return "", bucket.NotFoundError("prometheus file")
	}

	promDir := JobPrometheusDir(job)
	absPromDir, err := filepath.Abs(promDir)
	if err != nil {
		return "", bucket.UnexpectedError(err)
	}
	target := filepath.Join(absPromDir, relPath)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", bucket.UnexpectedError(err)
	}
	if absTarget != absPromDir && !strings.HasPrefix(absTarget, absPromDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: invalid prometheus path %q", bucket.ErrInvalidJob, relPath)
	}

	content, err := os.ReadFile(absTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s/_prometheus/%s (expected %s)", bucket.ErrNotFound, job, relPath, absTarget)
		}
		return "", err
	}
	return string(content), nil
}

// NormalizePrometheusRelPath maps common shorthand paths to files under _prometheus/.
func NormalizePrometheusRelPath(relPath string) string {
	relPath = strings.TrimPrefix(strings.TrimSpace(relPath), "/")
	switch relPath {
	case "", "scrape", "scrape.yaml":
		return ScrapeFileName
	case "scrape.yaml.tpl", "scrape.tpl":
		return ScrapeFileTplName
	default:
		return relPath
	}
}

// ListWorkspaceJobNames returns job folder names under workspace/jobs (manifest or not).
func ListWorkspaceJobNames() ([]string, error) {
	names := make(map[string]struct{})

	manifestJobs, err := workspace.Default().ListJobNames()
	if err != nil {
		return nil, err
	}
	for _, job := range manifestJobs {
		names[job] = struct{}{}
	}

	jobsDir := path.Join(bucket.WorkspaceLocation, "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return sortedJobNames(names), nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			names[entry.Name()] = struct{}{}
		}
	}
	return sortedJobNames(names), nil
}

// ListJobsWithWorkspaceScrape returns jobs that have _prometheus/scrape.yaml on disk.
func ListJobsWithWorkspaceScrape(jobsFilter []string) ([]string, error) {
	jobNames, err := ListWorkspaceJobNames()
	if err != nil {
		return nil, err
	}

	filter := make(map[string]struct{}, len(jobsFilter))
	for _, job := range jobsFilter {
		filter[job] = struct{}{}
	}

	out := make([]string, 0)
	for _, jobName := range jobNames {
		if len(filter) > 0 {
			if _, ok := filter[jobName]; !ok {
				continue
			}
		}
		if !HasScrapeWorkspaceFile(jobName) {
			continue
		}
		out = append(out, jobName)
	}
	return out, nil
}

func sortedJobNames(names map[string]struct{}) []string {
	out := make([]string, 0, len(names))
	for job := range names {
		out = append(out, job)
	}
	sort.Strings(out)
	return out
}

// MergePrometheusSummaries combines DB and workspace summaries per job.
func MergePrometheusSummaries(dbList, wsList []data.PrometheusJobSummary) []data.PrometheusJobSummary {
	byJob := make(map[string]data.PrometheusJobSummary)
	for _, summary := range dbList {
		byJob[summary.Job] = summary
	}
	for _, summary := range wsList {
		existing, ok := byJob[summary.Job]
		if !ok {
			byJob[summary.Job] = summary
			continue
		}
		existing.Scrape = existing.Scrape || summary.Scrape
		if summary.Alerts > existing.Alerts {
			existing.Alerts = summary.Alerts
		}
		if summary.Runbooks > existing.Runbooks {
			existing.Runbooks = summary.Runbooks
		}
		if summary.Dashboards > existing.Dashboards {
			existing.Dashboards = summary.Dashboards
		}
		byJob[summary.Job] = existing
	}

	out := make([]data.PrometheusJobSummary, 0, len(byJob))
	for _, summary := range byJob {
		out = append(out, summary)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Job < out[j].Job })
	return out
}
