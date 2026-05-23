// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"strings"

	"maand/utils"
)

func normalizeJobFilter(jobsFilter []string) []string {
	if len(jobsFilter) == 0 {
		return nil
	}
	return utils.Unique(jobsFilter)
}

func selectJobsForDeploy(availableJobs, jobsFilter []string) []string {
	jobsFilter = normalizeJobFilter(jobsFilter)
	if len(jobsFilter) == 0 {
		out := make([]string, len(availableJobs))
		copy(out, availableJobs)
		return out
	}

	selected := make([]string, 0)
	filterSet := make(map[string]struct{}, len(jobsFilter))
	for _, job := range jobsFilter {
		filterSet[job] = struct{}{}
	}
	for _, job := range availableJobs {
		if _, ok := filterSet[job]; ok {
			selected = append(selected, job)
		}
	}
	return selected
}

func parseJobsCSV(jobsCSV string) []string {
	jobsCSV = strings.TrimSpace(jobsCSV)
	if jobsCSV == "" {
		return nil
	}
	parts := strings.Split(jobsCSV, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return utils.Unique(out)
}
