// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maand/bucket"
	"os"
	"path"
)

type DefaultWorkspace struct {
}

func (ws *DefaultWorkspace) GetWorkers() ([]Worker, error) {
	data, err := os.ReadFile(path.Join(bucket.WorkspaceLocation, "workers.json"))
	if os.IsNotExist(err) {
		return []Worker{}, nil
	}
	if err != nil {
		return nil, err
	}

	var dataWorkers []Worker
	err = json.Unmarshal(data, &dataWorkers)
	if err != nil {
		return nil, err
	}

	var workers []Worker
	for idx, dataWorker := range dataWorkers {
		worker := NewWorker(dataWorker.Host, dataWorker.Labels, dataWorker.Memory, dataWorker.CPU, dataWorker.Tags, idx)
		workers = append(workers, worker)
	}
	return workers, nil
}

func (ws *DefaultWorkspace) GetJobs() ([]string, error) {
	paths, err := fs.Glob(os.DirFS(path.Join(bucket.WorkspaceLocation, "jobs")), "*/manifest.json")
	if err != nil {
		return nil, err
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
		return Manifest{}, err
	}

	var manifest Manifest
	err = json.Unmarshal(f, &manifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("invalid manifest file format %s : %w", jobName, err)
	}
	return manifest, nil
}

func (ws *DefaultWorkspace) GetDisabled() (DisabledAllocations, error) {
	disabledFile := path.Join(bucket.WorkspaceLocation, "disabled.json")
	f, err := os.ReadFile(disabledFile)
	if os.IsNotExist(err) {
		return DisabledAllocations{}, nil
	}
	if err != nil {
		return DisabledAllocations{}, err
	}

	var disabledAllocations DisabledAllocations
	err = json.Unmarshal(f, &disabledAllocations)
	if err != nil {
		return DisabledAllocations{}, err
	}

	return disabledAllocations, nil
}

func GetWorkspace() *DefaultWorkspace {
	return &DefaultWorkspace{}
}
