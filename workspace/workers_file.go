// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"strings"

	"maand/bucket"
	"maand/utils"
)

// WorkerRecord is the on-disk workers.json shape (position omitted => array index).
type WorkerRecord struct {
	Host     string            `json:"host"`
	Hostname string            `json:"hostname,omitempty"`
	Labels   []string          `json:"labels,omitempty"`
	Memory   string            `json:"memory,omitempty"`
	CPU      string            `json:"cpu,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
	Position *int              `json:"position,omitempty"`
}

// WorkerFacts holds resource values discovered on a worker host.
type WorkerFacts struct {
	MemoryMB float64
	CPUMHz   float64
}

// WorkerFactsChange describes an update to one worker entry.
type WorkerFactsChange struct {
	Host      string
	OldMemory string
	NewMemory string
	OldCPU    string
	NewCPU    string
}

// FormatMemoryMB formats megabytes for workers.json.
func FormatMemoryMB(mb float64) string {
	return fmt.Sprintf("%d mb", int(math.Round(mb)))
}

// FormatCPUMHz formats megahertz for workers.json.
func FormatCPUMHz(mhz float64) string {
	return fmt.Sprintf("%d mhz", int(math.Round(mhz)))
}

func workersJSONPath() string {
	return path.Join(bucket.WorkspaceLocation, "workers.json")
}

// ReadWorkersFile loads workers.json without normalizing labels or defaults.
func ReadWorkersFile() ([]WorkerRecord, error) {
	data, err := os.ReadFile(workersJSONPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []WorkerRecord{}, nil
		}
		return nil, err
	}

	var workers []WorkerRecord
	if err := json.Unmarshal(data, &workers); err != nil {
		return nil, fmt.Errorf("%w: %w", bucket.ErrInvalidWorkerJSON, err)
	}
	return workers, nil
}

// WriteWorkersFile writes workers.json with stable field ordering.
func WriteWorkersFile(workers []WorkerRecord) error {
	data, err := json.MarshalIndent(workers, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(workersJSONPath(), data, 0o644)
}

// ApplyWorkerFacts updates memory and cpu on matching workers.
func ApplyWorkerFacts(updates map[string]WorkerFacts) ([]WorkerFactsChange, error) {
	workers, err := ReadWorkersFile()
	if err != nil {
		return nil, err
	}
	if len(workers) == 0 {
		return nil, fmt.Errorf("%w: workers.json is empty", bucket.ErrInvalidWorkerJSON)
	}

	changes := make([]WorkerFactsChange, 0, len(updates))
	for idx := range workers {
		host := strings.TrimSpace(workers[idx].Host)
		facts, ok := updates[host]
		if !ok {
			continue
		}

		newMemory := FormatMemoryMB(facts.MemoryMB)
		newCPU := FormatCPUMHz(facts.CPUMHz)
		change := WorkerFactsChange{
			Host:      host,
			OldMemory: workers[idx].Memory,
			NewMemory: newMemory,
			OldCPU:    workers[idx].CPU,
			NewCPU:    newCPU,
		}

		workers[idx].Memory = newMemory
		workers[idx].CPU = newCPU

		if factsChanged(change) {
			changes = append(changes, change)
		}
	}

	if len(changes) == 0 {
		return changes, nil
	}

	if err := WriteWorkersFile(workers); err != nil {
		return nil, err
	}
	return changes, nil
}

func factsChanged(change WorkerFactsChange) bool {
	if !memoryStringEqual(change.OldMemory, change.NewMemory) {
		return true
	}
	if !cpuStringEqual(change.OldCPU, change.NewCPU) {
		return true
	}
	return false
}

func memoryStringEqual(left, right string) bool {
	leftMB, leftErr := utils.ParseMemoryMB(left)
	rightMB, rightErr := utils.ParseMemoryMB(right)
	if leftErr == nil && rightErr == nil {
		return math.Round(leftMB) == math.Round(rightMB)
	}
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func cpuStringEqual(left, right string) bool {
	leftMHz, leftErr := utils.ParseCPUMHz(left)
	rightMHz, rightErr := utils.ParseCPUMHz(right)
	if leftErr == nil && rightErr == nil {
		return math.Round(leftMHz) == math.Round(rightMHz)
	}
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

// FilterWorkersByLabels returns workers whose labels intersect filterLabels.
func FilterWorkersByLabels(workers []WorkerRecord, filterLabels []string) []WorkerRecord {
	if len(filterLabels) == 0 {
		return workers
	}

	normalized := make([]string, 0, len(filterLabels))
	for _, label := range filterLabels {
		label = strings.ToLower(strings.TrimSpace(label))
		if label != "" {
			normalized = append(normalized, label)
		}
	}
	if len(normalized) == 0 {
		return workers
	}

	filtered := make([]WorkerRecord, 0, len(workers))
	for _, worker := range workers {
		labels := append([]string(nil), worker.Labels...)
		labels = append(labels, "worker")
		for idx := range labels {
			labels[idx] = strings.ToLower(strings.TrimSpace(labels[idx]))
		}
		for _, want := range normalized {
			for _, have := range labels {
				if want == have {
					filtered = append(filtered, worker)
					goto nextWorker
				}
			}
		}
	nextWorker:
	}
	return filtered
}
