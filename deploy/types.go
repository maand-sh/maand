// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

// WorkerJobs is the jobs.json entry for a worker staging directory.
type WorkerJobs struct {
	Job      string `json:"job"`
	Disabled int    `json:"disabled"`
}

// WorkerData is worker.json written before rsync.
type WorkerData struct {
	BucketID  string   `json:"bucket_id"`
	WorkerID  string   `json:"worker_id"`
	WorkerIP  string   `json:"worker_ip"`
	Labels    []string `json:"labels"`
	UpdateSeq int      `json:"update_seq"`
}

// AllocationData is template context for .tpl files under a job directory.
type AllocationData struct {
	AllocationID   string `json:"allocation_id"`
	Job            string `json:"job"`
	CurrentVersion string `json:"current_version"`
	NewVersion     string `json:"new_version"`
	WorkerData
	BucketPath string
	JobPath    string
}
