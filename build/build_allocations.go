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
	disabledBefore, err := loadDisabledAllocations(tx)
	if err != nil {
		return err
	}

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

			targetVersion, err := data.TargetJobVersion(tx, jobName)
			if err != nil {
				_ = rows.Close()
				return err
			}

			upsertAllocationQuery := `INSERT OR REPLACE INTO allocations (
				alloc_id, job, worker_ip, disabled, removed, deployment_seq, new_version
			) VALUES (?, ?, ?, ?, ?, ?, ?)`
			_, err = tx.Exec(upsertAllocationQuery, allocationID, jobName, workerIP, 0, 0, 0, targetVersion)
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

	return markReenabledAllocations(tx, disabledBefore)
}

func loadDisabledAllocations(tx *sql.Tx) (map[string]struct{}, error) {
	rows, err := tx.Query(`SELECT worker_ip, job FROM allocations WHERE disabled = 1 AND removed = 0`)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	disabled := make(map[string]struct{})
	for rows.Next() {
		var workerIP, job string
		if err := rows.Scan(&workerIP, &job); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		disabled[workerIP+"|"+job] = struct{}{}
	}
	if err := data.RowsErr(rows); err != nil {
		return nil, err
	}
	return disabled, nil
}

func markReenabledAllocations(tx *sql.Tx, disabledBefore map[string]struct{}) error {
	if len(disabledBefore) == 0 {
		return nil
	}

	rows, err := tx.Query(`SELECT worker_ip, job, alloc_id FROM allocations WHERE disabled = 0 AND removed = 0`)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var workerIP, job, allocID string
		if err := rows.Scan(&workerIP, &job, &allocID); err != nil {
			return bucket.DatabaseError(err)
		}
		if _, wasDisabled := disabledBefore[workerIP+"|"+job]; !wasDisabled {
			continue
		}
		if err := data.MarkAllocationStartPending(tx, job, allocID); err != nil {
			return err
		}
	}
	return data.RowsErr(rows)
}
