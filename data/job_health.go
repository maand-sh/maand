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

// GetJobHealthCheck loads the built-in health_check spec for a job (nil when unset).
func GetJobHealthCheck(tx *sql.Tx, jobName string) (*workspace.ManifestHealthCheck, error) {
	var raw sql.NullString
	err := tx.QueryRow(`SELECT health_check FROM job WHERE name = ?`, jobName).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}

	var spec workspace.ManifestHealthCheck
	if err := json.Unmarshal([]byte(raw.String), &spec); err != nil {
		return nil, fmt.Errorf("%w: job %s health_check: %w", bucket.ErrInvalidManifest, jobName, err)
	}
	if len(spec.Checks) == 0 {
		return nil, nil
	}
	return &spec, nil
}

// GetJobPortNumber returns the assigned port number for one job port name.
func GetJobPortNumber(tx *sql.Tx, jobName, portName string) (int, error) {
	var port int
	err := tx.QueryRow(
		`SELECT port FROM job_ports WHERE job_id = (SELECT job_id FROM job WHERE name = ?) AND name = ?`,
		jobName, portName,
	).Scan(&port)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("%w: job %s port %q", bucket.ErrInvalidManifest, jobName, portName)
	}
	if err != nil {
		return 0, bucket.DatabaseError(err)
	}
	return port, nil
}
