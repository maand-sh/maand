// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package workspace reads job manifests and worker definitions from disk.
package workspace

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"maand/bucket"
)

// Workspace loads on-disk workspace configuration.
type Workspace interface {
	ListWorkers() ([]Worker, error)
	ListJobNames() ([]string, error)
	LoadManifest(jobName string) (Manifest, error)
	LoadDisabled() (DisabledAllocations, error)
}

// DefaultWorkspace reads from bucket.WorkspaceLocation.
type DefaultWorkspace struct{}

// Default returns the standard workspace reader.
func Default() *DefaultWorkspace {
	return &DefaultWorkspace{}
}

// GetWorkspace is deprecated; use Default.
func GetWorkspace() *DefaultWorkspace {
	return Default()
}

func (ws *DefaultWorkspace) ListWorkers() ([]Worker, error) {
	return ws.GetWorkers()
}

func (ws *DefaultWorkspace) GetWorkers() ([]Worker, error) {
	data, err := os.ReadFile(path.Join(bucket.WorkspaceLocation, "workers.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return []Worker{}, nil
		}
		return nil, err
	}

	var rawWorkers []WorkerRecord
	if err = json.Unmarshal(data, &rawWorkers); err != nil {
		return nil, fmt.Errorf("%w: %w", bucket.ErrInvalidWorkerJSON, err)
	}

	workers := make([]Worker, 0, len(rawWorkers))
	seenHosts := make(map[string]struct{}, len(rawWorkers))
	for idx, raw := range rawWorkers {
		host := strings.TrimSpace(raw.Host)
		if host == "" {
			return nil, fmt.Errorf("%w: host attribute can't be empty", bucket.ErrInvalidWorkerJSON)
		}
		if _, dup := seenHosts[host]; dup {
			return nil, fmt.Errorf("%w: duplicate host %q", bucket.ErrInvalidWorkerJSON, host)
		}
		seenHosts[host] = struct{}{}
		position := idx
		if raw.Position != nil {
			position = *raw.Position
		}
		w := NewWorker(host, raw.Labels, raw.Memory, raw.CPU, raw.Tags, position)
		if hn := strings.TrimSpace(raw.Hostname); hn != "" {
			w.Hostname = hn
		}
		workers = append(workers, w)
	}
	return workers, nil
}

func (ws *DefaultWorkspace) ListJobNames() ([]string, error) {
	return ws.GetJobs()
}

func (ws *DefaultWorkspace) GetJobs() ([]string, error) {
	paths, err := fs.Glob(os.DirFS(path.Join(bucket.WorkspaceLocation, "jobs")), "*/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}

	jobs := make([]string, 0, len(paths))
	for _, manifestFile := range paths {
		jobs = append(jobs, path.Dir(manifestFile))
	}
	return jobs, nil
}

func (ws *DefaultWorkspace) LoadManifest(jobName string) (Manifest, error) {
	return ws.GetJobManifest(jobName)
}

func (ws *DefaultWorkspace) GetJobManifest(jobName string) (Manifest, error) {
	manifestFile := path.Join(bucket.WorkspaceLocation, "jobs", jobName, "manifest.json")
	data, err := os.ReadFile(manifestFile)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w:%w", bucket.ErrUnexpectedError, err)
	}

	var manifest Manifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: job %s\n%w", bucket.ErrInvalidManifest, jobName, err)
	}
	return manifest, nil
}

func (ws *DefaultWorkspace) LoadDisabled() (DisabledAllocations, error) {
	return ws.GetDisabled()
}

func (ws *DefaultWorkspace) GetDisabled() (DisabledAllocations, error) {
	disabledFile := path.Join(bucket.WorkspaceLocation, "disabled.json")
	data, err := os.ReadFile(disabledFile)
	if err != nil {
		if os.IsNotExist(err) {
			return DisabledAllocations{}, nil
		}
		return DisabledAllocations{}, err
	}

	var disabled DisabledAllocations
	if err = json.Unmarshal(data, &disabled); err != nil {
		return DisabledAllocations{}, err
	}
	return disabled, nil
}
