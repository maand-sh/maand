// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"strconv"

	"maand/bucket"
)

// JobPortAssignments maps job name → port key → assigned number.
type JobPortAssignments map[string]map[string]int

// GetAllJobPortAssignments loads current job_ports rows keyed by job and port name.
func GetAllJobPortAssignments(tx *sql.Tx) (JobPortAssignments, error) {
	rows, err := tx.Query(`
		SELECT j.name, jp.name, jp.port
		FROM job_ports jp
		JOIN job j ON j.job_id = jp.job_id
	`)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	assignments := make(JobPortAssignments)
	for rows.Next() {
		var jobName, portName string
		var port int
		if err := rows.Scan(&jobName, &portName, &port); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		if assignments[jobName] == nil {
			assignments[jobName] = make(map[string]int)
		}
		assignments[jobName][portName] = port
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return assignments, nil
}

// GetJobPortMap returns port name → number for one job.
func GetJobPortMap(tx *sql.Tx, jobName string) (map[string]string, error) {
	rows, err := tx.Query(
		`SELECT name, port FROM job_ports WHERE job_id = (SELECT job_id FROM job WHERE name = ?)`,
		jobName,
	)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	ports := make(map[string]string)
	for rows.Next() {
		var name string
		var port int
		if err := rows.Scan(&name, &port); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		ports[name] = strconv.Itoa(port)
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return ports, nil
}

// GetJobPortMapInt returns port name → number for one job.
func GetJobPortMapInt(tx *sql.Tx, jobName string) (map[string]int, error) {
	stringPorts, err := GetJobPortMap(tx, jobName)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(stringPorts))
	for name, value := range stringPorts {
		port, err := strconv.Atoi(value)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		out[name] = port
	}
	return out, nil
}
