// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcontrol

import (
	"strings"

	"maand/utils"
)

// Filters holds optional job and worker IP filters from CLI CSV input.
type Filters struct {
	Jobs    []string
	Workers []string
}

// ParseFilters splits comma-separated job and worker filters and deduplicates them.
func ParseFilters(jobsCSV, workersCSV string) Filters {
	return Filters{
		Jobs:    parseCSV(jobsCSV),
		Workers: parseCSV(workersCSV),
	}
}

func parseCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			trimmed = append(trimmed, part)
		}
	}
	return utils.Unique(trimmed)
}

func (f Filters) validateAgainst(allJobs, allWorkers []string) error {
	if len(f.Jobs) > 0 && len(intersect(allJobs, f.Jobs)) == 0 {
		return &InvalidFilterError{Kind: "jobs", Values: f.Jobs}
	}
	if len(f.Workers) > 0 && len(intersect(allWorkers, f.Workers)) == 0 {
		return &InvalidFilterError{Kind: "workers", Values: f.Workers}
	}
	return nil
}

func selectJobs(jobsAtSeq, jobFilter []string) []string {
	if len(jobFilter) == 0 {
		out := make([]string, len(jobsAtSeq))
		copy(out, jobsAtSeq)
		return out
	}
	selected := make([]string, 0)
	for _, job := range jobsAtSeq {
		if contains(jobFilter, job) {
			selected = append(selected, job)
		}
	}
	return selected
}

func workerSelected(workerIP string, workerFilter []string) bool {
	return len(workerFilter) == 0 || contains(workerFilter, workerIP)
}

func intersect(a, b []string) []string {
	return utils.Intersection(a, b)
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
