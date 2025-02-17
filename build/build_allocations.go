package build

import (
	"database/sql"
	"maand/data"
	"maand/utils"
	"maand/workspace"
)

func Allocations(tx *sql.Tx, ws *workspace.DefaultWorkspace) {
	for _, workerIP := range data.GetWorkers(tx, nil) {

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
		utils.Check(err)

		var assignedJobs []string
		for rows.Next() {
			var job string
			err = rows.Scan(&job)
			utils.Check(err)
			assignedJobs = append(assignedJobs, job)

			allocID := data.GetAllocationID(tx, workerIP, job)
			if allocID == "" {
				allocID = workspace.GetHashUUID(job + "|" + workerIP)
			}
			query = "INSERT OR REPLACE INTO allocations (alloc_id, job, worker_ip, disabled, removed, deployment_seq) VALUES (?, ?, ?, ?, ?, ?)"
			_, err := tx.Exec(query, allocID, job, workerIP, 0, 0, 0)
			utils.Check(err)
		}

		// handle missing allocations
		allocatedJobs := data.GetAllocatedJobs(tx, workerIP)
		diffs := utils.Difference(allocatedJobs, assignedJobs)
		for _, deletedJob := range diffs {
			_, err := tx.Exec("UPDATE allocations SET removed = 1 WHERE job = ? AND worker_ip = ?", deletedJob, workerIP)
			utils.Check(err)
		}
	}

	_, err := tx.Exec("UPDATE allocations SET removed = 1 WHERE worker_ip NOT IN (SELECT worker_ip FROM worker)")
	utils.Check(err)

	disabledAllocations := ws.GetDisabled()
	for _, workerIP := range disabledAllocations.Workers {
		_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE worker_ip = ?", workerIP)
		utils.Check(err)
	}

	for job, obj := range disabledAllocations.Jobs {
		if len(obj.Workers) == 0 {
			_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE job = ?", job)
			utils.Check(err)
		} else {
			for _, workerIP := range obj.Workers {
				_, err := tx.Exec("UPDATE allocations SET disabled = 1 WHERE job = ? AND worker_ip = ?", job, workerIP)
				utils.Check(err)
			}
		}
	}

	//for _, workerIP := range data.GetWorkers(tx, nil) {
	//	for _, job := range data.GetAllocatedJobs(tx, workerIP) {
	//		allocID := data.GetAllocationID(tx, workerIP, job)
	//		if data.IsAllocationDisabled(tx, workerIP, job) == 1 {
	//			_, err := tx.Exec("UPDATE hash SET previous_hash = NULL WHERE namespace = ? AND key = ?", fmt.Sprintf("%s_allocation", job), allocID)
	//			utils.Check(err)
	//		}
	//	}
	//}
}
