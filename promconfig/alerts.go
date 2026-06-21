// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package promconfig

import (
	"fmt"
	"os"
	"path"
	"strings"

	"maand/bucket"

	"gopkg.in/yaml.v3"
)

type alertRuleFile struct {
	Groups []alertRuleGroup `yaml:"groups"`
}

type alertRuleGroup struct {
	Name  string      `yaml:"name"`
	Rules []alertRule `yaml:"rules"`
}

type alertRule struct {
	Alert       string            `yaml:"alert"`
	Record      string            `yaml:"record"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

// AlertFileEntry indexes one alert YAML stored in job_files.
type AlertFileEntry struct {
	Job  string `json:"job"`
	Path string `json:"path"`
	Rel  string `json:"rel"`
}

func ValidateAlertFile(jobName, relPath string, data []byte, runbookSlugs map[string]struct{}) error {
	parsed, err := parseAlertRuleFile(jobName, relPath, data)
	if err != nil {
		return err
	}
	return validateAlertRuleFile(jobName, relPath, parsed, runbookSlugs)
}

// AlertNamesFromFile returns alert rule names declared in one alert file.
func AlertNamesFromFile(data []byte) ([]string, error) {
	var parsed alertRuleFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, group := range parsed.Groups {
		for _, rule := range group.Rules {
			if name := strings.TrimSpace(rule.Alert); name != "" {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

func parseAlertRuleFile(jobName, relPath string, data []byte) (alertRuleFile, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return alertRuleFile{}, fmt.Errorf("%w: job %s _prometheus/%s is empty", bucket.ErrInvalidJob, jobName, relPath)
	}

	var parsed alertRuleFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return alertRuleFile{}, fmt.Errorf("%w: job %s _prometheus/%s: %w", bucket.ErrInvalidJob, jobName, relPath, err)
	}
	if len(parsed.Groups) == 0 {
		return alertRuleFile{}, fmt.Errorf("%w: job %s _prometheus/%s must define at least one group", bucket.ErrInvalidJob, jobName, relPath)
	}
	return parsed, nil
}

func validateAlertRuleFile(jobName, relPath string, parsed alertRuleFile, runbookSlugs map[string]struct{}) error {
	for groupIdx, group := range parsed.Groups {
		if strings.TrimSpace(group.Name) == "" {
			return fmt.Errorf("%w: job %s _prometheus/%s groups[%d] missing name", bucket.ErrInvalidJob, jobName, relPath, groupIdx)
		}
		for ruleIdx, rule := range group.Rules {
			if err := validateAlertRule(jobName, relPath, groupIdx, ruleIdx, rule, runbookSlugs); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAlertRule(
	jobName, relPath string,
	groupIdx, ruleIdx int,
	rule alertRule,
	runbookSlugs map[string]struct{},
) error {
	alertName := strings.TrimSpace(rule.Alert)
	recordName := strings.TrimSpace(rule.Record)
	hasAlert := alertName != ""
	hasRecord := recordName != ""

	switch {
	case hasAlert && hasRecord:
		return fmt.Errorf("%w: job %s _prometheus/%s groups[%d].rules[%d] must define either alert or record, not both",
			bucket.ErrInvalidJob, jobName, relPath, groupIdx, ruleIdx)
	case !hasAlert && !hasRecord:
		return fmt.Errorf("%w: job %s _prometheus/%s groups[%d].rules[%d] must define alert or record",
			bucket.ErrInvalidJob, jobName, relPath, groupIdx, ruleIdx)
	}

	if strings.TrimSpace(rule.Expr) == "" {
		ruleLabel := alertName
		if hasRecord {
			ruleLabel = recordName
		}
		return fmt.Errorf("%w: job %s _prometheus/%s groups[%d].rules[%d] %q missing expr",
			bucket.ErrInvalidJob, jobName, relPath, groupIdx, ruleIdx, ruleLabel)
	}

	if hasAlert {
		if runbook := strings.TrimSpace(rule.Annotations["runbook"]); runbook != "" {
			if _, ok := runbookSlugs[runbook]; !ok {
				return fmt.Errorf("%w: job %s _prometheus/%s alert %q references missing runbook %q",
					bucket.ErrInvalidJob, jobName, relPath, alertName, runbook)
			}
		}
	}
	return nil
}

// ListRunbookSlugs returns basenames of markdown files in _prometheus/runbooks/.
func ListRunbookSlugs(jobName string) (map[string]struct{}, error) {
	dir := path.Join(JobPrometheusDir(jobName), RunbooksDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}

	slugs := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			slugs[strings.TrimSuffix(name, ".md")] = struct{}{}
		}
	}
	return slugs, nil
}

// ListAlertFiles returns relative paths under _prometheus/ for alert YAML files.
func ListAlertFiles(jobName string) ([]string, error) {
	dir := path.Join(JobPrometheusDir(jobName), AlertsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	paths := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			paths = append(paths, path.Join(AlertsDir, name))
		}
	}
	return paths, nil
}
