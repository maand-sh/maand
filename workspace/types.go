// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

// DisabledAllocations lists jobs and workers excluded from scheduling.
type DisabledAllocations struct {
	Jobs map[string]struct {
		Allocations []string `json:"allocations"`
	} `json:"jobs"`
	Workers []string `json:"workers"`
}

// JobCommand is a named command entry from a job manifest.
type JobCommand struct {
	Name       string
	ExecutedOn []string `json:"executed_on"`
	Demands    struct {
		Job     string                 `json:"job"`
		Command string                 `json:"command"`
		Config  map[string]interface{} `json:"config"`
	} `json:"demands"`
}

// AllocationCommand is deprecated; use JobCommand.
type AllocationCommand = JobCommand

// CertSubject describes certificate subject fields in a manifest.
type CertSubject struct {
	CommonName string `json:"common_name"`
}

// Manifest is the jobs/<name>/manifest.json schema.
type Manifest struct {
	Version   string   `json:"version"`
	Selectors []string `json:"selectors"`
	Resources struct {
		Memory struct {
			Min string `json:"min"`
			Max string `json:"max"`
		} `json:"memory"`
		CPU struct {
			Min string `json:"min"`
			Max string `json:"max"`
		} `json:"cpu"`
		Ports ManifestPorts `json:"ports"`
	} `json:"resources"`
	Commands map[string]JobCommand `json:"commands"`
	Certs    map[string]struct {
		PKCS8   bool        `json:"pkcs8"`
		One     bool        `json:"one"`
		Subject CertSubject `json:"subject"`
	} `json:"certs"`
	HealthCheck         *ManifestHealthCheck `json:"health_check,omitempty"`
	UpdateParallelCount int                  `json:"update_parallel_count"`
	DeployParallelCount int                  `json:"deploy_parallel_count"`
	RestartPolicy       string               `json:"restart_policy,omitempty"`
	RestartGlobs        []string             `json:"restart_globs,omitempty"`
}
