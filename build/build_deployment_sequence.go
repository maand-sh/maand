// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"fmt"
	"strings"

	"maand/bucket"
)

func BuildDeploymentSequence(tx *sql.Tx) error {
	dependencies, err := loadJobCommandDependencies(tx)
	if err != nil {
		return err
	}
	if err := detectCircularJobCommandDependencies(dependencies); err != nil {
		return err
	}

	updateDeploymentSequenceQuery := `
		 WITH RECURSIVE job_command_seq AS (
			SELECT jc.job, 0 AS level FROM job_commands jc WHERE jc.demand_job = ''

			UNION ALL

			SELECT jc.job, jcs.level + 1 AS level
			FROM job_commands jc INNER JOIN job_command_seq jcs ON jc.demand_job = jcs.job
		)
		UPDATE allocations SET deployment_seq = t.deployment_seq FROM (
		SELECT
			DISTINCT job, deployment_seq
		FROM
			(SELECT job, (SELECT MAX(level) FROM job_command_seq jcs WHERE jcs.job = t.job) as deployment_seq FROM job_command_seq t) t1 
		ORDER BY deployment_seq) t WHERE allocations.job = t.job;
	`
	_, err = tx.Exec(updateDeploymentSequenceQuery)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

func loadJobCommandDependencies(tx *sql.Tx) (map[string][]string, error) {
	rows, err := tx.Query("SELECT DISTINCT job, demand_job FROM job_commands WHERE demand_job != ''")
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	dependencies := make(map[string][]string)
	for rows.Next() {
		var jobName, demandedJobName string
		if err := rows.Scan(&jobName, &demandedJobName); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		dependencies[jobName] = append(dependencies[jobName], demandedJobName)
		if _, ok := dependencies[demandedJobName]; !ok {
			dependencies[demandedJobName] = nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, bucket.DatabaseError(err)
	}
	return dependencies, nil
}

func detectCircularJobCommandDependencies(dependencies map[string][]string) error {
	const (
		unseen = iota
		visiting
		complete
	)

	visitState := make(map[string]int, len(dependencies))

	var visitJob func(jobName string, path []string) error
	visitJob = func(jobName string, path []string) error {
		switch visitState[jobName] {
		case complete:
			return nil
		case visiting:
			for i, nameInPath := range path {
				if nameInPath == jobName {
					cycle := append(path[i:], jobName)
					return fmt.Errorf("%w: %s", bucket.ErrCircularJobCommandDependency, strings.Join(cycle, " -> "))
				}
			}
			return fmt.Errorf("%w: %s", bucket.ErrCircularJobCommandDependency, jobName)
		}

		visitState[jobName] = visiting
		path = append(path, jobName)
		for _, demandedJobName := range dependencies[jobName] {
			if err := visitJob(demandedJobName, path); err != nil {
				return err
			}
		}
		visitState[jobName] = complete
		return nil
	}

	for jobName := range dependencies {
		if visitState[jobName] == unseen {
			if err := visitJob(jobName, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
