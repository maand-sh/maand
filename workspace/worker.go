// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"strings"
)

type Worker struct {
	Host     string            `json:"host"`
	Labels   []string          `json:"labels"`
	Memory   string            `json:"memory"`
	CPU      string            `json:"cpu"`
	Tags     map[string]string `json:"tags"`
	Position int               `json:"position"`
}

func NewWorker(host string, labels []string, memory string, cpu string, tags map[string]string, position int) Worker {
	labels = append(labels, "worker")

	for idx, label := range labels {
		labels[idx] = strings.ToLower(label)
	}

	newTags := map[string]string{}
	for key, value := range tags {
		key = strings.ToLower(key)
		value = strings.ToLower(value)
		newTags[key] = value
	}

	if memory == "" {
		memory = "0 MB"
	}

	if cpu == "" {
		cpu = "0 MHZ"
	}

	return Worker{
		Host:     host,
		Labels:   labels,
		Memory:   memory,
		CPU:      cpu,
		Tags:     newTags,
		Position: position,
	}
}
