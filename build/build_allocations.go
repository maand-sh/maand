// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"errors"
	"maand/data"
	"maand/utils"
	"maand/workspace"
)

func Allocations(tx *sql.Tx, ws *workspace.DefaultWorkspace) error {
	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	for _, workerIP := range workers {

		query := `
			SELECT DISTINCT j.name
            FROM job j
            JOIN job_selectors js ON js.job_id = j.job_id
            JOIN worker_labels al ON al.label = js.selector
            WHERE (
                SELECT COUNT(DISTINCT js_sub.selector)
                FROM job_selectors js_sub
                WHERE js_sub.job_id = j.job_id
            ) = (
                SELECT COUNT(DISTINCT wl_sub.label)
                FROM worker_labels wl_sub
                JOIN worker a ON wl_sub.worker_id = a.worker_id
                WHERE wl_sub.label IN (
                    SELECT jl_sub.selector
                    FROM job_selectors jl_sub
                    WHERE jl_sub.job_id = j.job_id
                ) AND a.worker_ip = ?
			) 
		`

		rows, err := tx.Query(query, workerIP)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		var assignedJobs []string
		for rows.Next() {
			var job string
			err = rows.Scan(&job)
			if err != nil {
				return data.NewDatabaseError(err)
			}

			assignedJobs = append(assignedJobs, job)

			allocID, err := data.GetAllocationID(tx, workerIP, job)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}

			if allocID == "" {
				allocID = workspace.GetHashUUID(job + "|" + workerIP)
			}

			query = "INSERT OR REPLACE INTO allocations (alloc_id, job, worker_ip, disabled, removed, deployment_seq) VALUES (?, ?, ?, ?, ?, ?)"
			_, err = tx.Exec(query, allocID, job, workerIP, 0, 0, 0)
			if err != nil {
				return data.NewDatabaseError(err)
			}
		}

		// handle missing allocations
		allocatedJobs, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}

		diffs := utils.Difference(allocatedJobs, assignedJobs)
		for _, deletedJob := range diffs {
			_, err := tx.Exec("UPDATE allocations SET removed = 1 WHERE job = ? AND worker_ip = ?", deletedJob, workerIP)
			if err != nil {
				return data.NewDatabaseError(err)
			}
		}
	}

	_, err = tx.Exec("UPDATE allocations SET removed = 1 WHERE worker_ip NOT IN (SELECT worker_ip FROM worker)")
	if err != nil {
		return data.NewDatabaseError(err)
	}

	disabledAllocations, err := ws.GetDisabled()
	if err != nil {
		return err
	}

	for _, workerIP := range disabledAllocations.Workers {
		_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE worker_ip = ?", workerIP)
		if err != nil {
			return data.NewDatabaseError(err)
		}
	}

	for job, obj := range disabledAllocations.Jobs {
		if len(obj.Workers) == 0 {
			_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE job = ?", job)
			if err != nil {
				return data.NewDatabaseError(err)
			}
		} else {
			for _, workerIP := range obj.Workers {
				_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE job = ? AND worker_ip = ?", job, workerIP)
				if err != nil {
					return data.NewDatabaseError(err)
				}
			}
		}
	}

	return nil
}
