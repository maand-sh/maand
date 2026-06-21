// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"maand/bucket"
	"maand/workspace"
)

// GetJobManifestPorts loads resources.ports from a job's manifest.json in job_files.
func GetJobManifestPorts(tx *sql.Tx, jobName string) (workspace.ManifestPorts, error) {
	content, err := GetJobFileContent(tx, jobName+"/manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest workspace.Manifest
	if err := json.Unmarshal([]byte(content), &manifest); err != nil {
		return nil, fmt.Errorf("%w: job %s manifest.json: %w", bucket.ErrInvalidJob, jobName, err)
	}
	if manifest.Resources.Ports == nil {
		return workspace.ManifestPorts{}, nil
	}
	return manifest.Resources.Ports, nil
}
