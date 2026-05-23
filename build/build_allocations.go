// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"errors"

	"maand/bucket"
	"maand/data"
	"maand/utils"
	"maand/workspace"
)

const matchingJobsByLabelsQuery = `
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

func BuildAllocations(tx *sql.Tx, jobWorkspace *workspace.DefaultWorkspace) error {
	workerIPs, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	for _, workerIP := range workerIPs {
		rows, err := tx.Query(matchingJobsByLabelsQuery, workerIP)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		var matchedJobNames []string
		for rows.Next() {
			var jobName string
			err = rows.Scan(&jobName)
			if err != nil {
				_ = rows.Close()
				return bucket.DatabaseError(err)
			}

			matchedJobNames = append(matchedJobNames, jobName)

			allocationID, err := data.GetAllocationID(tx, workerIP, jobName)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				_ = rows.Close()
				return err
			}

			if allocationID == "" {
				allocationID = workspace.GetHashUUID(jobName + "|" + workerIP)
			}

			upsertAllocationQuery := "INSERT OR REPLACE INTO allocations (alloc_id, job, worker_ip, disabled, removed, deployment_seq) VALUES (?, ?, ?, ?, ?, ?)"
			_, err = tx.Exec(upsertAllocationQuery, allocationID, jobName, workerIP, 0, 0, 0)
			if err != nil {
				_ = rows.Close()
				return bucket.DatabaseError(err)
			}
		}
		if err := rows.Close(); err != nil {
			return bucket.DatabaseError(err)
		}
		if err := data.RowsErr(rows); err != nil {
			return err
		}

		persistedJobNames, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}

		jobNamesToRemove := utils.Difference(persistedJobNames, matchedJobNames)
		for _, jobName := range jobNamesToRemove {
			_, err := tx.Exec("UPDATE allocations SET removed = 1 WHERE job = ? AND worker_ip = ?", jobName, workerIP)
			if err != nil {
				return bucket.DatabaseError(err)
			}
		}
	}

	_, err = tx.Exec("UPDATE allocations SET removed = 1 WHERE worker_ip NOT IN (SELECT worker_ip FROM worker)")
	if err != nil {
		return bucket.DatabaseError(err)
	}

	disableConfig, err := jobWorkspace.GetDisabled()
	if err != nil {
		return err
	}

	for _, workerIP := range disableConfig.Workers {
		_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE worker_ip = ?", workerIP)
		if err != nil {
			return bucket.DatabaseError(err)
		}
	}

	for jobName, jobDisableSpec := range disableConfig.Jobs {
		if len(jobDisableSpec.Allocations) == 0 {
			_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE job = ?", jobName)
			if err != nil {
				return bucket.DatabaseError(err)
			}
		} else {
			for _, workerIP := range jobDisableSpec.Allocations {
				_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE job = ? AND worker_ip = ?", jobName, workerIP)
				if err != nil {
					return bucket.DatabaseError(err)
				}
			}
		}
	}

	return nil
}
