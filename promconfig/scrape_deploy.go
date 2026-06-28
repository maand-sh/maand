// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package promconfig

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/utils"
	"maand/workspace"
)

// RenderScrapeConfigsYAML expands maand:port placeholders using live allocations and ports,
// then returns an indented YAML fragment for prometheus job templates.
// jobsFilter limits output to named maand jobs; nil or empty means all scrape jobs.
// Uses the build KV catalog when loaded; otherwise reads scrape.yaml from the workspace.
func RenderScrapeConfigsYAML(tx *sql.Tx, jobsFilter []string) (string, error) {
	yamlFragment, err := renderScrapeConfigsFromKV(tx, jobsFilter)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(yamlFragment) != "" {
		return yamlFragment, nil
	}
	return renderScrapeConfigsFromWorkspace(tx, jobsFilter)
}

func renderScrapeConfigsFromKV(tx *sql.Tx, jobsFilter []string) (string, error) {
	store, err := kv.RequireStore()
	if err != nil {
		if errors.Is(err, kv.ErrStoreNotInitialized) {
			return "", nil
		}
		return "", err
	}

	jobsValue, err := store.Get(KVNamespace, "scrape_jobs")
	if err != nil || strings.TrimSpace(jobsValue.Value) == "" {
		return "", nil
	}

	scrapeJobs := strings.Split(jobsValue.Value, ",")
	if len(jobsFilter) > 0 {
		scrapeJobs = utils.Intersection(scrapeJobs, jobsFilter)
	}

	allExpanded := make([]map[string]interface{}, 0)
	for _, jobName := range scrapeJobs {
		jobName = strings.TrimSpace(jobName)
		if jobName == "" {
			continue
		}

		perJobValue, err := store.Get(KVNamespace, "scrape/"+jobName)
		if err != nil {
			return "", err
		}

		var configs []map[string]interface{}
		if err := json.Unmarshal([]byte(perJobValue.Value), &configs); err != nil {
			return "", err
		}

		expanded, err := expandScrapeConfigsForJob(tx, jobName, configs)
		if err != nil {
			if errors.Is(err, ErrNoActiveScrapeTargets) {
				continue
			}
			return "", err
		}
		allExpanded = append(allExpanded, expanded...)
	}

	return ScrapeConfigsYAML(allExpanded)
}

func renderScrapeConfigsFromWorkspace(tx *sql.Tx, jobsFilter []string) (string, error) {
	scrapeJobs, err := ListJobsWithWorkspaceScrape(jobsFilter)
	if err != nil {
		return "", err
	}

	allExpanded := make([]map[string]interface{}, 0)
	for _, jobName := range scrapeJobs {
		scrapeData, err := ReadScrapeFileContent(tx, jobName)
		if err != nil {
			return "", err
		}
		configs, err := ParseScrapeFile(scrapeData)
		if err != nil {
			return "", fmt.Errorf("job %s: %w", jobName, err)
		}
		expanded, err := expandScrapeConfigsForJob(tx, jobName, configs)
		if err != nil {
			if errors.Is(err, ErrNoActiveScrapeTargets) {
				continue
			}
			return "", err
		}
		allExpanded = append(allExpanded, expanded...)
	}

	return ScrapeConfigsYAML(allExpanded)
}

func expandScrapeConfigsForJob(tx *sql.Tx, jobName string, configs []map[string]interface{}) ([]map[string]interface{}, error) {
	ports, err := jobManifestPorts(tx, jobName)
	if err != nil {
		return nil, err
	}
	portNumbers, err := data.GetJobPortMapInt(tx, jobName)
	if err != nil {
		return nil, err
	}
	activeWorkers, err := data.GetActiveAllocations(tx, jobName)
	if err != nil {
		return nil, err
	}
	return ExpandScrapeConfigs(jobName, configs, ports, portNumbers, activeWorkers)
}

func jobManifestPorts(tx *sql.Tx, jobName string) (workspace.ManifestPorts, error) {
	ports, err := data.GetJobManifestPorts(tx, jobName)
	if err == nil {
		return ports, nil
	}
	if !isNotFoundErr(err) {
		return nil, err
	}
	manifest, err := workspace.Default().LoadManifest(jobName)
	if err != nil {
		return nil, err
	}
	if manifest.Resources.Ports == nil {
		return workspace.ManifestPorts{}, nil
	}
	return manifest.Resources.Ports, nil
}

func isNotFoundErr(err error) bool {
	return errors.Is(err, bucket.ErrNotFound) || errors.Is(err, sql.ErrNoRows)
}
