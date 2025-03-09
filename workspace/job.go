// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"io/fs"
	"maand/bucket"
	"os"
	"path"
)

type DisabledAllocations struct {
	Jobs map[string]struct {
		Workers []string `json:"workers"`
	} `json:"jobs"`
	Workers []string `json:"workers"`
}

type AllocationCommand struct {
	Name       string
	ExecutedOn []string `json:"executed_on"`
	Demands    struct {
		Job     string                 `json:"job"`
		Command string                 `json:"command"`
		Config  map[string]interface{} `json:"config"`
	} `json:"demands"`
}

type CertSubject struct {
	CommonName string `json:"common_name"`
}

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
		Ports map[string]int `json:"ports"`
	} `json:"resources"`
	Commands map[string]AllocationCommand `json:"commands"`
	Certs    map[string]struct {
		PKCS8   bool        `json:"pkcs8"`
		Subject CertSubject `json:"subject"`
	} `json:"certs"`
	UpdateParallelCount int `json:"update_parallel_count"`
}

func WalkJobFiles(name string, callback func(path string, d fs.DirEntry, err error) error) error {
	return fs.WalkDir(os.DirFS(path.Join(bucket.WorkspaceLocation, "jobs")), name, callback)
}

func GetJobFilePath(fpath string) string {
	return path.Join(bucket.WorkspaceLocation, "jobs", fpath)
}
