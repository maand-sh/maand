package workspace

import (
	"encoding/json"
	"io/fs"
	"maand/bucket"
	"maand/utils"
	"os"
	"path"
)

type DefaultWorkspace struct {
}

func (ws *DefaultWorkspace) GetWorkers() []Worker {
	data, err := os.ReadFile(path.Join(bucket.WorkspaceLocation, "workers.json"))
	if os.IsNotExist(err) {
		return []Worker{}
	}
	utils.Check(err)

	var dataWorkers []Worker
	err = json.Unmarshal(data, &dataWorkers)
	utils.Check(err)

	var workers []Worker
	for idx, dataWorker := range dataWorkers {
		worker := NewWorker(dataWorker.Host, dataWorker.Labels, dataWorker.Memory, dataWorker.CPU, dataWorker.Tags, idx)
		workers = append(workers, worker)
	}
	return workers
}

func (ws *DefaultWorkspace) GetJobs() []string {
	paths, err := fs.Glob(os.DirFS(path.Join(bucket.WorkspaceLocation, "jobs")), "*/manifest.json")
	utils.Check(err)

	var jobs []string
	for idx := range paths {
		manifestFile := paths[idx]
		jobName := path.Dir(manifestFile)
		jobs = append(jobs, jobName)
	}
	return jobs
}

func (ws *DefaultWorkspace) GetJobManifest(jobName string) Manifest {
	manifestFile := path.Join(bucket.WorkspaceLocation, "jobs", jobName, "manifest.json")
	f, err := os.ReadFile(manifestFile)
	utils.Check(err)

	var manifest Manifest
	err = json.Unmarshal(f, &manifest)
	utils.Check(err)

	return manifest
}

func (ws *DefaultWorkspace) GetDisabled() DisabledAllocations {
	disabledFile := path.Join(bucket.WorkspaceLocation, "disabled.json")
	f, err := os.ReadFile(disabledFile)
	if os.IsNotExist(err) {
		return DisabledAllocations{}
	}
	utils.Check(err)

	var disabledAllocations DisabledAllocations
	err = json.Unmarshal(f, &disabledAllocations)
	utils.Check(err)

	return disabledAllocations
}

func GetWorkspace() *DefaultWorkspace {
	return &DefaultWorkspace{}
}
