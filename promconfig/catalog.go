// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package promconfig

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"maand/bucket"
	"maand/kv"
	"maand/workspace"
)

// BuildCatalog aggregates _prometheus/ content from all workspace jobs into KV.
func BuildCatalog(tx *sql.Tx, jobWorkspace workspace.Workspace) error {
	jobNames, err := jobWorkspace.ListJobNames()
	if err != nil {
		return err
	}
	sort.Strings(jobNames)

	allUnexpanded := make([]map[string]interface{}, 0)
	scrapeJobs := make([]string, 0)
	alertJobs := make([]string, 0)
	runbookJobs := make([]string, 0)
	alertFiles := make([]AlertFileEntry, 0)
	prometheusJobNames := make(map[string]string)
	alertNames := make(map[string]string)

	kvValues := make(map[string]string)

	for _, jobName := range jobNames {
		if err := workspace.ValidatePrometheusServerFiles(jobName); err != nil {
			return err
		}
		if err := ValidateScrapeFiles(jobName); err != nil {
			return err
		}

		runbookSlugs, err := ListRunbookSlugs(jobName)
		if err != nil {
			return err
		}
		if len(runbookSlugs) > 0 {
			runbookJobs = append(runbookJobs, jobName)
			for slug := range runbookSlugs {
				kvValues[fmt.Sprintf("runbooks/%s/%s", jobName, slug)] = fmt.Sprintf(
					`{"file":%q}`, path.Join(jobName, "_prometheus", RunbooksDir, slug+".md"))
			}
		}

		alertPaths, err := ListAlertFiles(jobName)
		if err != nil {
			return err
		}
		if len(alertPaths) > 0 {
			alertJobs = append(alertJobs, jobName)
			for _, rel := range alertPaths {
				filePath := path.Join(jobName, "_prometheus", rel)
				content, err := os.ReadFile(workspace.JobFilePath(filePath))
				if err != nil {
					return err
				}
				if err := ValidateAlertFile(jobName, rel, content, runbookSlugs); err != nil {
					return err
				}
				names, err := AlertNamesFromFile(content)
				if err != nil {
					return fmt.Errorf("job %s: %w", jobName, err)
				}
				for _, alertName := range names {
					if owner, exists := alertNames[alertName]; exists {
						return fmt.Errorf("%w: duplicate alert name %q (jobs %s and %s)",
							bucket.ErrInvalidJob, alertName, owner, jobName)
					}
					alertNames[alertName] = jobName
				}
				alertFiles = append(alertFiles, AlertFileEntry{
					Job:  jobName,
					Path: filePath,
					Rel:  rel,
				})
			}
			kvValues[fmt.Sprintf("alerts/%s/files", jobName)] = strings.Join(alertPaths, ",")
		}

		if err := ValidateScrapeFiles(jobName); err != nil {
			return err
		}

		scrapeData, err := ReadScrapeFileContent(tx, jobName)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		manifest, err := jobWorkspace.LoadManifest(jobName)
		if err != nil {
			return err
		}

		configs, err := ParseScrapeFile(scrapeData)
		if err != nil {
			return fmt.Errorf("job %s: %w", jobName, err)
		}
		if err := ValidateScrapeConfigs(jobName, configs); err != nil {
			return err
		}
		if err := ValidateScrapePortReferences(jobName, configs, manifest.Resources.Ports); err != nil {
			return err
		}

		for _, cfg := range configs {
			jobLabel, _ := cfg["job_name"].(string)
			resolved := ResolveScrapeJobName(jobName, jobLabel)
			if owner, exists := prometheusJobNames[resolved]; exists {
				return fmt.Errorf("%w: duplicate prometheus job_name %q (jobs %s and %s)",
					bucket.ErrInvalidJob, resolved, owner, jobName)
			}
			prometheusJobNames[resolved] = jobName
		}

		syncJobScrapeNameKV(jobName, configs)

		perJobJSON, err := json.Marshal(configs)
		if err != nil {
			return err
		}
		kvValues[fmt.Sprintf("scrape/%s", jobName)] = string(perJobJSON)
		scrapeJobs = append(scrapeJobs, jobName)
		allUnexpanded = append(allUnexpanded, configs...)
	}

	scrapeConfigsJSON, err := json.Marshal(allUnexpanded)
	if err != nil {
		return err
	}
	alertFilesJSON, err := json.Marshal(alertFiles)
	if err != nil {
		return err
	}

	kvValues["scrape_configs"] = string(scrapeConfigsJSON)
	kvValues["scrape_jobs"] = strings.Join(scrapeJobs, ",")
	kvValues["scrape_jobs_length"] = fmt.Sprintf("%d", len(scrapeJobs))
	kvValues["alert_jobs"] = strings.Join(alertJobs, ",")
	kvValues["alert_files"] = string(alertFilesJSON)
	kvValues["runbook_jobs"] = strings.Join(runbookJobs, ",")

	return syncPrometheusKV(kvValues)
}

func syncJobScrapeNameKV(jobName string, configs []map[string]interface{}) {
	if len(configs) == 0 {
		return
	}
	raw, _ := configs[0]["job_name"].(string)
	kv.GetKVStore().Put(
		fmt.Sprintf("maand/job/%s", jobName),
		"job_name",
		ResolveScrapeJobName(jobName, raw),
		0,
	)
}

func syncPrometheusKV(keyValues map[string]string) error {
	store := kv.GetKVStore()
	presentKeys := make([]string, 0, len(keyValues))
	for key, value := range keyValues {
		store.Put(KVNamespace, key, value, 0)
		presentKeys = append(presentKeys, key)
	}

	existingKeys, err := store.GetKeys(KVNamespace)
	if err != nil {
		return err
	}

	stale := difference(existingKeys, presentKeys)
	for _, key := range stale {
		if err := store.Delete(KVNamespace, key); err != nil {
			return err
		}
	}
	return nil
}

func difference(existing, present []string) []string {
	keep := make(map[string]struct{}, len(present))
	for _, key := range present {
		keep[key] = struct{}{}
	}
	out := make([]string, 0)
	for _, key := range existing {
		if _, ok := keep[key]; !ok {
			out = append(out, key)
		}
	}
	return out
}
