// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import "strings"

// workerJSON is the on-disk workers.json shape (position omitted => array index).
type workerJSON struct {
	Host     string            `json:"host"`
	Hostname string            `json:"hostname,omitempty"`
	Labels   []string          `json:"labels"`
	Memory   string            `json:"memory"`
	CPU      string            `json:"cpu"`
	Tags     map[string]string `json:"tags"`
	Position *int              `json:"position"`
}

// Worker describes a cluster node from workers.json.
type Worker struct {
	Host     string            `json:"host"`
	Hostname string            `json:"hostname,omitempty"`
	Labels   []string          `json:"labels"`
	Memory   string            `json:"memory"`
	CPU      string            `json:"cpu"`
	Tags     map[string]string `json:"tags"`
	Position int               `json:"position"`
}

// NewWorker normalizes labels/tags and applies defaults for empty resources.
func NewWorker(host string, labels []string, memory string, cpu string, tags map[string]string, position int) Worker {
	labels = append([]string(nil), labels...)
	labels = append(labels, "worker")

	for idx, label := range labels {
		labels[idx] = strings.ToLower(strings.TrimSpace(label))
	}

	normalizedTags := make(map[string]string, len(tags))
	for key, value := range tags {
		normalizedTags[strings.ToLower(strings.TrimSpace(key))] = strings.ToLower(strings.TrimSpace(value))
	}

	if strings.TrimSpace(memory) == "" {
		memory = "0 MB"
	}
	if strings.TrimSpace(cpu) == "" {
		cpu = "0 MHZ"
	}

	return Worker{
		Host:     strings.TrimSpace(host),
		Labels:   labels,
		Memory:   memory,
		CPU:      cpu,
		Tags:     normalizedTags,
		Position: position,
	}
}
