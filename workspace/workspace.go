// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package workspace provides interfaces for workspace
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

type DefaultWorkspace struct{}

func (ws *DefaultWorkspace) GetWorkers() ([]Worker, error) {
	data, err := os.ReadFile(path.Join(bucket.WorkspaceLocation, "workers.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return []Worker{}, nil
		}
		return nil, err
	}

	var dataWorkers []Worker
	if err = json.Unmarshal(data, &dataWorkers); err != nil {
		return nil, fmt.Errorf("%w: %w", bucket.ErrInvaildWorkerJSON, err)
	}

	var workers []Worker
	for idx, dataWorker := range dataWorkers {
		dataWorker.Host = strings.Trim(dataWorker.Host, " ")
		if dataWorker.Host == "" {
			return nil, fmt.Errorf("%w: host attribute can't be empty", bucket.ErrInvaildWorkerJSON)
		}
		worker := NewWorker(dataWorker.Host, dataWorker.Labels, dataWorker.Memory, dataWorker.CPU, dataWorker.Tags, idx)
		workers = append(workers, worker)
	}
	return workers, nil
}

func (ws *DefaultWorkspace) GetJobs() ([]string, error) {
	paths, err := fs.Glob(os.DirFS(path.Join(bucket.WorkspaceLocation, "jobs")), "*/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}

	var jobs []string
	for idx := range paths {
		manifestFile := paths[idx]
		jobName := path.Dir(manifestFile)
		jobs = append(jobs, jobName)
	}
	return jobs, nil
}

func (ws *DefaultWorkspace) GetJobManifest(jobName string) (Manifest, error) {
	manifestFile := path.Join(bucket.WorkspaceLocation, "jobs", jobName, "manifest.json")
	f, err := os.ReadFile(manifestFile)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w:%w", bucket.ErrUnexpectedError, err)
	}

	var manifest Manifest
	if err = json.Unmarshal(f, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: job %s\n%w", bucket.ErrInvalidManifest, jobName, err)
	}
	return manifest, nil
}

func (ws *DefaultWorkspace) GetDisabled() (DisabledAllocations, error) {
	disabledFile := path.Join(bucket.WorkspaceLocation, "disabled.json")
	f, err := os.ReadFile(disabledFile)
	if err != nil {
		if os.IsNotExist(err) {
			return DisabledAllocations{}, nil
		}
		return DisabledAllocations{}, err
	}

	var disabledAllocations DisabledAllocations
	if err = json.Unmarshal(f, &disabledAllocations); err != nil {
		return DisabledAllocations{}, err
	}

	return disabledAllocations, nil
}

func GetWorkspace() *DefaultWorkspace {
	return &DefaultWorkspace{}
}
