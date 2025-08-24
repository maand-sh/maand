// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maand/bucket"
	"maand/data"
	"maand/utils"
	"maand/workspace"
	"os"
	"path"
	"strings"
)

var removedJobs []string

func Jobs(tx *sql.Tx, ws *workspace.DefaultWorkspace) error {

	wsJobs, err := ws.GetJobs()
	if err != nil {
		return err
	}

	for _, job := range wsJobs {
		jobID := workspace.GetHashUUID(job)

		deletes := []string{
			"DELETE FROM job_selectors WHERE job_id = ?",
			"DELETE FROM job_commands WHERE job_id = ?",
			"DELETE FROM job_files WHERE job_id = ?",
			"DELETE FROM job_ports WHERE job_id = ?",
			"DELETE FROM job_certs WHERE job_id = ?",
		}

		for _, stmt := range deletes {
			_, err := tx.Exec(stmt, jobID)
			if err != nil {
				return err
			}
		}

		manifest, err := ws.GetJobManifest(job)
		if err != nil {
			return err
		}

		minCPUMhz, err := utils.ExtractCPUFrequencyInMHz(workspace.GetMinCPU(manifest))
		if err != nil {
			return fmt.Errorf("error extracting minimum cpu frequency in MHz, job %s: %v", job, err)
		}
		maxCPUMhz, err := utils.ExtractCPUFrequencyInMHz(workspace.GetMaxCPU(manifest))
		if err != nil {
			return fmt.Errorf("error extracting maximum cpu frequency in MHz, job %s: %v", job, err)
		}

		if minCPUMhz != 0 && maxCPUMhz == 0 {
			maxCPUMhz = minCPUMhz
		}
		if minCPUMhz > maxCPUMhz {
			return fmt.Errorf("job %s, minCPUMhz > maxCPUMhz: %f", job, minCPUMhz)
		}

		minMemoryMb, err := utils.ExtractSizeInMB(workspace.GetMinMemory(manifest))
		if err != nil {
			return fmt.Errorf("error extracting minimum memory in MB, job %s: %v", job, err)
		}
		maxMemoryMb, err := utils.ExtractSizeInMB(workspace.GetMaxMemory(manifest))
		if err != nil {
			return fmt.Errorf("error extracting maximum memory in MB, job %s: %v", job, err)
		}
		if minMemoryMb != 0 && maxMemoryMb == 0.0 {
			maxMemoryMb = minMemoryMb
		}
		if minMemoryMb > maxMemoryMb {
			return fmt.Errorf("job %s, minMemoryMb > maxMemoryMb: %f", job, minMemoryMb)
		}

		jobConfig, err := getJobConf(job)
		if err != nil {
			return err
		}

		var memory = maxMemoryMb
		if _, ok := jobConfig["memory"]; ok {
			memory, err = utils.ExtractSizeInMB(jobConfig["memory"])
			if err != nil {
				return err
			}
		}

		var cpu = maxCPUMhz
		if _, ok := jobConfig["cpu"]; ok {
			cpu, err = utils.ExtractCPUFrequencyInMHz(jobConfig["cpu"])
			if err != nil {
				return err
			}
		}

		version := workspace.GetVersion(manifest)
		query := `
			INSERT OR REPLACE INTO job (job_id, name, version, min_memory_mb, max_memory_mb, current_memory_mb, min_cpu_mhz, max_cpu_mhz, current_cpu_mhz, update_parallel_count)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = tx.Exec(
			query, jobID, job, version,
			fmt.Sprintf("%v", minMemoryMb),
			fmt.Sprintf("%v", maxMemoryMb),
			fmt.Sprintf("%v", memory),
			fmt.Sprintf("%v", minCPUMhz),
			fmt.Sprintf("%v", maxCPUMhz),
			fmt.Sprintf("%v", cpu),
			workspace.GetUpdateParallelCount(manifest),
		)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		for _, selector := range manifest.Selectors {
			_, err := tx.Exec("INSERT INTO job_selectors (job_id, selector) VALUES (?, ?)", jobID, selector)
			if err != nil {
				return data.NewDatabaseError(err)
			}
		}

		for name, port := range manifest.Resources.Ports {
			prefix := fmt.Sprintf("%s_port_", job)
			if !strings.HasPrefix(name, prefix) {
				return fmt.Errorf("invalid port name, name: %s, should have prefix : %s", name, prefix)
			}

			var portUsedJob string
			row := tx.QueryRow("SELECT (SELECT name FROM job WHERE job_id = jp.job_id) as job FROM job_ports jp WHERE port = ?", port)
			err = row.Scan(&portUsedJob)
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("port can't be reused, port: %d, jobs (%s, %s)", port, job, portUsedJob)
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}

			_, err = tx.Exec("INSERT INTO job_ports (job_id, name, port) VALUES (?, ?, ?)", jobID, name, port)
			if err != nil {
				return data.NewDatabaseError(err)
			}
		}

		for _, command := range workspace.GetCommands(manifest) {
			if !strings.HasPrefix(command.Name, "command_") {
				return fmt.Errorf("invalid command name, name: %s, should have prefix : %s", command.Name, "command_")
			}
			if len(command.ExecutedOn) == 0 {
				return fmt.Errorf("job %s, job_command %s required have a executed_on", job, command.Name)
			}
			diffs := utils.Difference(command.ExecutedOn, []string{"post_build", "health_check", "cli", "pre_deploy", "post_deploy", "job_control"})
			if len(diffs) > 0 {
				return fmt.Errorf("job %s, job_command %s not valid executed_on %v", job, command.Name, diffs)
			}
			if command.Demands.Job == job {
				return fmt.Errorf("job %s, job_command %s invalid configuration, self referencing", job, command.Name)
			}
			commandPath := path.Join(bucket.WorkspaceLocation, "jobs", job, "_modules", fmt.Sprintf("%s.py", command.Name))
			_, err := os.Stat(commandPath)
			if os.IsNotExist(err) {
				return fmt.Errorf("%s does not exist, registered for job %s", commandPath, job)
			}

			query := `
				INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`
			for _, executedOn := range command.ExecutedOn {
				jsonData, err := json.Marshal(command.Demands.Config)
				if err != nil {
					return err
				}

				_, err = tx.Exec(query, jobID, job, command.Name, executedOn, command.Demands.Job, command.Demands.Command, string(jsonData))
				if err != nil {
					return data.NewDatabaseError(err)
				}
			}
		}

		_, err = os.Stat(path.Join(workspace.GetJobFilePath(job), "Makefile"))
		if os.IsNotExist(err) {
			return fmt.Errorf("'Makefile' is required, job %s", job)
		}

		query = `INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)`
		err = workspace.WalkJobFiles(job, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			var content = []byte("")
			if !d.IsDir() {
				content, err = os.ReadFile(workspace.GetJobFilePath(path))
				if err != nil {
					return err
				}
			}
			_, err = tx.Exec(query, jobID, path, content, d.IsDir())
			if err != nil {
				return data.NewDatabaseError(err)
			}
			return err
		})
		if err != nil {
			return err
		}

		// hash update manifest's cert config
		certData, err := json.Marshal(manifest.Certs)
		if err != nil {
			return err
		}

		currentHash, err := utils.MD5Content(certData)
		if err != nil {
			return err
		}

		err = data.UpdateHash(tx, "build_certs", job, currentHash)
		if err != nil {
			return err
		}

		query = `INSERT INTO job_certs (job_id, name, pkcs8, one, subject) VALUES (?, ?, ?, ?, ?)`
		for name, config := range manifest.Certs {
			subject, err := json.Marshal(config.Subject)
			if err != nil {
				return err
			}
			_, err = tx.Exec(query, jobID, name, config.PKCS8, config.One, subject)
			if err != nil {
				return data.NewDatabaseError(err)
			}
		}
	}

	workspaceJobs, err := ws.GetJobs()
	if err != nil {
		return err
	}

	availableJobs, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
		SELECT
			a.worker_ip, w.available_memory_mb, w.available_cpu_mhz, sum(j.current_memory_mb) as needed_memory, sum(j.current_cpu_mhz) AS needed_cpu
		FROM
			allocations a JOIN job j ON j.name = a.job
				JOIN
			worker w ON w.worker_ip = a.worker_ip
		GROUP BY a.worker_ip
	`)
	if err != nil {
		return data.NewDatabaseError(err)
	}
	resourceValidationFailed := false
	for rows.Next() {
		var worker_ip string
		var avaiable_memory_mb, available_cpu_mhz, needed_memory_mb, needed_cpu_mhz float64
		err = rows.Scan(&worker_ip, &avaiable_memory_mb, &available_cpu_mhz, &needed_memory_mb, &needed_cpu_mhz)
		if err != nil {
			return err
		}

		if avaiable_memory_mb < needed_memory_mb {
			resourceValidationFailed = true
			fmt.Printf("worker_ip %s, available memory is %.2f MB, allocated memory is %.2f MB\n", worker_ip, avaiable_memory_mb, needed_memory_mb)
		}
		if available_cpu_mhz < available_cpu_mhz {
			resourceValidationFailed = true
			fmt.Printf("worker_ip %s, available cpu is %.2f MHZ, allocated cpu is %.2f MHZ\n", worker_ip, available_cpu_mhz, needed_cpu_mhz)
		}
	}

	if resourceValidationFailed {
		return errors.New("validation failed: resource allocation")
	}

	// remove missing jobs
	diffs := utils.Difference(availableJobs, workspaceJobs)
	removedJobs = []string{}
	for _, job := range diffs {
		removedJobs = append(removedJobs, job)
		_, err := tx.Exec("DELETE FROM job WHERE name = ?", job)
		if err != nil {
			return data.NewDatabaseError(err)
		}
		_, err = tx.Exec("DELETE FROM hash WHERE namespace = 'build_certs' AND key = ?", job)
		if err != nil {
			return data.NewDatabaseError(err)
		}
	}
	return nil
}
