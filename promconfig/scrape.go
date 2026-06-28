// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package promconfig

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"maand/bucket"
	"maand/workspace"

	"gopkg.in/yaml.v3"
)

// ErrNoActiveScrapeTargets is returned when maand:port placeholders expand to zero targets.
var ErrNoActiveScrapeTargets = errors.New("no active allocation targets after expanding scrape config")

var forbiddenServiceDiscoveryKeys = []string{
	"kubernetes_sd_configs",
	"dns_sd_configs",
	"file_sd_configs",
	"consul_sd_configs",
	"ecs_sd_configs",
	"ec2_sd_configs",
	"openstack_sd_configs",
	"azure_sd_configs",
	"gce_sd_configs",
	"hetzner_sd_configs",
	"ionos_sd_configs",
	"linode_sd_configs",
	"digitalocean_sd_configs",
	"docker_sd_configs",
	"dockerswarm_sd_configs",
	"eureka_sd_configs",
	"http_sd_configs",
	"kuma_sd_configs",
	"marathon_sd_configs",
	"nerve_sd_configs",
	"serverset_sd_configs",
	"triton_sd_configs",
	"uyuni_sd_configs",
}

// ParseScrapeFile parses _prometheus/scrape.yaml as a YAML array of scrape_config items.
func ParseScrapeFile(data []byte) ([]map[string]interface{}, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("%w: scrape.yaml is empty", bucket.ErrInvalidJob)
	}

	var configs []map[string]interface{}
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("%w: scrape.yaml: %w", bucket.ErrInvalidJob, err)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("%w: scrape.yaml must contain at least one scrape item", bucket.ErrInvalidJob)
	}
	return configs, nil
}

// ValidateScrapeConfigs checks scrape items before target expansion.
func ValidateScrapeConfigs(jobName string, configs []map[string]interface{}) error {
	for idx, cfg := range configs {
		for _, key := range forbiddenServiceDiscoveryKeys {
			if _, ok := cfg[key]; ok {
				return fmt.Errorf("%w: job %s scrape.yaml[%d] %s is not supported (use static_configs with maand:port placeholders)",
					bucket.ErrInvalidJob, jobName, idx, key)
			}
		}

		jobLabel, ok := cfg["job_name"].(string)
		if !ok || strings.TrimSpace(jobLabel) == "" {
			return fmt.Errorf("%w: job %s scrape.yaml[%d] missing job_name", bucket.ErrInvalidJob, jobName, idx)
		}
	}
	return nil
}

// ValidateScrapePortReferences checks maand:port placeholders against manifest ports at build time.
func ValidateScrapePortReferences(jobName string, configs []map[string]interface{}, ports workspace.ManifestPorts) error {
	for idx, cfg := range configs {
		rawStatic, ok := cfg["static_configs"]
		if !ok {
			continue
		}
		staticSlice, ok := rawStatic.([]interface{})
		if !ok {
			continue
		}
		for blockIdx, block := range staticSlice {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			rawTargets, ok := blockMap["targets"]
			if !ok {
				continue
			}
			targetList, ok := rawTargets.([]interface{})
			if !ok {
				continue
			}
			for targetIdx, rawTarget := range targetList {
				target, ok := rawTarget.(string)
				if !ok || !strings.HasPrefix(target, PortPlaceholderPrefix) {
					continue
				}
				if err := validatePortPlaceholder(jobName, idx, blockIdx, targetIdx, target, ports); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ScrapeConfigsRequireActiveAllocations reports whether any static_configs target uses maand:port placeholders.
func ScrapeConfigsRequireActiveAllocations(configs []map[string]interface{}) bool {
	for _, cfg := range configs {
		rawStatic, ok := cfg["static_configs"]
		if !ok {
			continue
		}
		staticSlice, ok := rawStatic.([]interface{})
		if !ok {
			continue
		}
		for _, block := range staticSlice {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			rawTargets, ok := blockMap["targets"]
			if !ok {
				continue
			}
			targetList, ok := rawTargets.([]interface{})
			if !ok {
				continue
			}
			for _, rawTarget := range targetList {
				target, ok := rawTarget.(string)
				if !ok {
					continue
				}
				if strings.HasPrefix(target, PortPlaceholderPrefix) {
					return true
				}
			}
		}
	}
	return false
}

func validatePortPlaceholder(jobName string, cfgIdx, blockIdx, targetIdx int, target string, ports workspace.ManifestPorts) error {
	rest := strings.TrimPrefix(target, PortPlaceholderPrefix)
	parts := strings.Split(rest, "/")
	portKey := parts[0]
	if portKey == "" {
		return fmt.Errorf("%w: job %s scrape.yaml[%d].static_configs[%d].targets[%d] invalid placeholder %q",
			bucket.ErrInvalidJob, jobName, cfgIdx, blockIdx, targetIdx, target)
	}
	if portKey == ImplicitPortKey {
		var err error
		portKey, err = ResolveMetricsPortKey(jobName, ports)
		if err != nil {
			return fmt.Errorf("job %s scrape.yaml[%d]: %w", jobName, cfgIdx, err)
		}
	}
	if _, ok := ports[portKey]; !ok {
		return fmt.Errorf("%w: job %s scrape.yaml[%d].static_configs[%d].targets[%d] port %q is not declared in manifest resources.ports",
			bucket.ErrInvalidJob, jobName, cfgIdx, blockIdx, targetIdx, portKey)
	}
	return nil
}

// ResolveMetricsPortKey picks the manifest port key for implicit scrape targets.
func ResolveMetricsPortKey(jobName string, ports workspace.ManifestPorts) (string, error) {
	candidates := []string{
		jobName + "_metrics_port",
		"metrics_port",
	}
	for _, name := range candidates {
		if _, ok := ports[name]; ok {
			return name, nil
		}
	}

	var suffixMatches []string
	for name := range ports {
		if strings.HasSuffix(name, "_metrics_port") {
			suffixMatches = append(suffixMatches, name)
		}
	}
	sort.Strings(suffixMatches)
	switch len(suffixMatches) {
	case 0:
		return "", fmt.Errorf("%w: job %s scrape.yaml uses maand:port/_implicit but no metrics port is declared in manifest",
			bucket.ErrInvalidJob, jobName)
	case 1:
		return suffixMatches[0], nil
	default:
		return "", fmt.Errorf("%w: job %s ambiguous metrics ports %v; set an explicit port in scrape.yaml",
			bucket.ErrInvalidJob, jobName, suffixMatches)
	}
}

// ResolveScrapeJobName expands maand:job to the maand job folder name.
func ResolveScrapeJobName(maandJob, value string) string {
	if value == JobPlaceholder {
		return maandJob
	}
	return value
}

// ExpandScrapeConfigs resolves maand:port and maand:job placeholders using active allocations and assigned ports.
func ExpandScrapeConfigs(
	jobName string,
	configs []map[string]interface{},
	ports workspace.ManifestPorts,
	portNumbers map[string]int,
	activeWorkers []string,
) ([]map[string]interface{}, error) {
	expanded := make([]map[string]interface{}, 0, len(configs))
	for idx, cfg := range configs {
		copyCfg, err := deepCopyMap(cfg)
		if err != nil {
			return nil, err
		}
		expandScrapeJobReferences(jobName, copyCfg)
		if err := expandStaticConfigs(jobName, copyCfg, ports, portNumbers, activeWorkers); err != nil {
			return nil, fmt.Errorf("job %s scrape.yaml[%d]: %w", jobName, idx, err)
		}
		expanded = append(expanded, copyCfg)
	}
	return expanded, nil
}

func expandScrapeJobReferences(maandJob string, cfg map[string]interface{}) {
	if raw, ok := cfg["job_name"].(string); ok {
		cfg["job_name"] = ResolveScrapeJobName(maandJob, raw)
	}

	rawStatic, ok := cfg["static_configs"]
	if !ok {
		return
	}
	staticSlice, ok := rawStatic.([]interface{})
	if !ok {
		return
	}
	for _, block := range staticSlice {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		rawLabels, ok := blockMap["labels"]
		if !ok {
			continue
		}
		labelMap, ok := rawLabels.(map[string]interface{})
		if !ok {
			continue
		}
		for key, rawValue := range labelMap {
			value, ok := rawValue.(string)
			if !ok {
				continue
			}
			labelMap[key] = ResolveScrapeJobName(maandJob, value)
		}
	}
}

func expandStaticConfigs(
	jobName string,
	cfg map[string]interface{},
	ports workspace.ManifestPorts,
	portNumbers map[string]int,
	activeWorkers []string,
) error {
	rawStatic, ok := cfg["static_configs"]
	if !ok {
		return fmt.Errorf("%w: static_configs is required", bucket.ErrInvalidJob)
	}
	staticSlice, ok := rawStatic.([]interface{})
	if !ok || len(staticSlice) == 0 {
		return fmt.Errorf("%w: static_configs must be a non-empty list", bucket.ErrInvalidJob)
	}

	newStatic := make([]interface{}, 0, len(staticSlice))
	for blockIdx, block := range staticSlice {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%w: static_configs[%d] must be a mapping", bucket.ErrInvalidJob, blockIdx)
		}
		copyBlock, err := deepCopyMap(blockMap)
		if err != nil {
			return err
		}

		rawTargets, ok := copyBlock["targets"]
		if !ok {
			return fmt.Errorf("%w: static_configs[%d].targets is required", bucket.ErrInvalidJob, blockIdx)
		}
		targetList, ok := rawTargets.([]interface{})
		if !ok || len(targetList) == 0 {
			return fmt.Errorf("%w: static_configs[%d].targets must be a non-empty list", bucket.ErrInvalidJob, blockIdx)
		}

		expandedTargets := make([]interface{}, 0)
		for _, rawTarget := range targetList {
			target, ok := rawTarget.(string)
			if !ok {
				return fmt.Errorf("%w: static_configs[%d].targets entries must be strings", bucket.ErrInvalidJob, blockIdx)
			}
			resolved, err := expandTarget(jobName, target, ports, portNumbers, activeWorkers)
			if err != nil {
				return err
			}
			for _, item := range resolved {
				expandedTargets = append(expandedTargets, item)
			}
		}
		if len(expandedTargets) == 0 {
			return fmt.Errorf("%w: job %s", ErrNoActiveScrapeTargets, jobName)
		}
		copyBlock["targets"] = expandedTargets
		newStatic = append(newStatic, copyBlock)
	}
	cfg["static_configs"] = newStatic
	return nil
}

func expandTarget(
	jobName, target string,
	ports workspace.ManifestPorts,
	portNumbers map[string]int,
	activeWorkers []string,
) ([]string, error) {
	if !strings.HasPrefix(target, PortPlaceholderPrefix) {
		return []string{target}, nil
	}

	rest := strings.TrimPrefix(target, PortPlaceholderPrefix)
	parts := strings.Split(rest, "/")
	portKey := parts[0]
	if portKey == "" {
		return nil, fmt.Errorf("%w: invalid maand port placeholder %q", bucket.ErrInvalidJob, target)
	}

	pinnedWorker := ""
	if len(parts) > 1 {
		pinnedWorker = parts[1]
	}

	if portKey == ImplicitPortKey {
		var err error
		portKey, err = ResolveMetricsPortKey(jobName, ports)
		if err != nil {
			return nil, err
		}
	}
	if _, ok := ports[portKey]; !ok {
		return nil, fmt.Errorf("%w: port %q is not declared in manifest resources.ports", bucket.ErrInvalidJob, portKey)
	}
	portNumber, ok := portNumbers[portKey]
	if !ok {
		return nil, fmt.Errorf("%w: port %q has no assigned number (run maand build)", bucket.ErrInvalidJob, portKey)
	}

	workers := activeWorkers
	if pinnedWorker != "" {
		found := false
		for _, workerIP := range activeWorkers {
			if workerIP == pinnedWorker {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: worker %q is not an active allocation for job %s", bucket.ErrInvalidJob, pinnedWorker, jobName)
		}
		workers = []string{pinnedWorker}
	}

	targets := make([]string, 0, len(workers))
	for _, workerIP := range workers {
		targets = append(targets, fmt.Sprintf("%s:%d", workerIP, portNumber))
	}
	return targets, nil
}

func deepCopyMap(in map[string]interface{}) (map[string]interface{}, error) {
	raw, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := make(map[string]interface{})
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScrapeConfigsYAML renders expanded scrape configs as an indented YAML fragment for templates.
func ScrapeConfigsYAML(configs []map[string]interface{}) (string, error) {
	if len(configs) == 0 {
		return "", nil
	}
	raw, err := yaml.Marshal(configs)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	var b strings.Builder
	for _, line := range lines {
		if line == "" {
			continue
		}
		b.WriteString("\n  ")
		b.WriteString(line)
	}
	return b.String(), nil
}
