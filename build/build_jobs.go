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
	"os"
	"path"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/utils"
	"maand/workspace"
)

var removedJobs []string // global variable captures removed jobs from workspace

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
				return bucket.DatabaseError(err)
			}
		}

		manifest, err := ws.GetJobManifest(job)
		if err != nil {
			return err
		}

		minCPUMhz, err := utils.ExtractCPUFrequencyInMHz(workspace.GetMinCPU(manifest))
		if err != nil {
			return fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, job, err)
		}
		maxCPUMhz, err := utils.ExtractCPUFrequencyInMHz(workspace.GetMaxCPU(manifest))
		if err != nil {
			return fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, job, err)
		}

		if minCPUMhz != 0 && maxCPUMhz == 0 {
			maxCPUMhz = minCPUMhz
		}
		if minCPUMhz > maxCPUMhz {
			return fmt.Errorf("%w: job %s minCPUMhz > maxCPUMhz", bucket.ErrInvalidManifest, job)
		}

		minMemoryMb, err := utils.ExtractSizeInMB(workspace.GetMinMemory(manifest))
		if err != nil {
			return fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, job, err)
		}
		maxMemoryMb, err := utils.ExtractSizeInMB(workspace.GetMaxMemory(manifest))
		if err != nil {
			return fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, job, err)
		}
		if minMemoryMb != 0 && maxMemoryMb == 0.0 {
			maxMemoryMb = minMemoryMb
		}
		if minMemoryMb > maxMemoryMb {
			return fmt.Errorf("%w: job %s minMemoryMb > maxMemoryMb", bucket.ErrInvalidManifest, job)
		}

		jobConfig, err := getJobConf(job)
		if err != nil {
			return err
		}

		memory := maxMemoryMb
		if _, ok := jobConfig["memory"]; ok {
			memory, err = utils.ExtractSizeInMB(jobConfig["memory"])
			if err != nil {
				return err
			}
		}

		cpu := maxCPUMhz
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
			return bucket.DatabaseError(err)
		}

		for _, selector := range manifest.Selectors {
			_, err := tx.Exec("INSERT INTO job_selectors (job_id, selector) VALUES (?, ?)", jobID, selector)
			if err != nil {
				return bucket.DatabaseError(err)
			}
		}

		for name, port := range manifest.Resources.Ports {
			prefix := fmt.Sprintf("%s_port_", job)
			if !strings.HasPrefix(name, prefix) {
				return fmt.Errorf("%w: key %s, excepted %s", bucket.ErrPortKeyFormat, name, prefix)
			}

			var portUsedJob string
			row := tx.QueryRow("SELECT (SELECT name FROM job WHERE job_id = jp.job_id) as job FROM job_ports jp WHERE port = ?", port)
			err = row.Scan(&portUsedJob)
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: port %d, jobs (%s, %s)", bucket.ErrPortCollision, port, job, portUsedJob)
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}

			_, err = tx.Exec("INSERT INTO job_ports (job_id, name, port) VALUES (?, ?, ?)", jobID, name, port)
			if err != nil {
				return bucket.DatabaseError(err)
			}
		}

		for _, command := range workspace.GetCommands(manifest) {
			if !strings.HasPrefix(command.Name, "command_") {
				return fmt.Errorf("%w: invalid command name, name: %s, excepted prefix : %s", bucket.ErrInvalidJobCommandConfiguration, command.Name, "command_")
			}
			if len(command.ExecutedOn) == 0 {
				return fmt.Errorf("%w: job %s, job_command %s missing executed_on", bucket.ErrInvalidJobCommandConfiguration, job, command.Name)
			}
			diffs := utils.Difference(command.ExecutedOn, []string{"post_build", "health_check", "cli", "pre_deploy", "post_deploy", "job_control"})
			if len(diffs) > 0 {
				return fmt.Errorf("%w: job %s, job_command %s invalid executed_on %v", bucket.ErrInvalidJobCommandConfiguration, job, command.Name, diffs)
			}
			if command.Demands.Job == job {
				return fmt.Errorf("%w: job %s, job_command %s invalid configuration, self referencing", bucket.ErrInvalidJobCommandConfiguration, job, command.Name)
			}
			commandPath := path.Join(bucket.WorkspaceLocation, "jobs", job, "_modules", fmt.Sprintf("%s.py", command.Name))
			_, err := os.Stat(commandPath)
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s does not exist, registered for job %s", bucket.ErrJobCommandFileNotFound, commandPath, job)
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
					return bucket.DatabaseError(err)
				}
			}
		}

		_, err = os.Stat(path.Join(workspace.GetJobFilePath(job), "Makefile"))
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: job %s, Makefile not found", bucket.ErrInvalidJob, job)
		}

		query = `INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)`
		err = workspace.WalkJobFiles(job, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			content := []byte("")
			if !d.IsDir() {
				content, err = os.ReadFile(workspace.GetJobFilePath(path))
				if err != nil {
					return err
				}
			}
			_, err = tx.Exec(query, jobID, path, content, d.IsDir())
			if err != nil {
				return bucket.DatabaseError(err)
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
				return bucket.DatabaseError(err)
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

	// remove missing jobs
	diffs := utils.Difference(availableJobs, workspaceJobs)
	removedJobs = []string{}
	for _, job := range diffs {
		removedJobs = append(removedJobs, job)
		_, err := tx.Exec("DELETE FROM job WHERE name = ?", job)
		if err != nil {
			return bucket.DatabaseError(err)
		}
		_, err = tx.Exec("DELETE FROM hash WHERE namespace = 'build_certs' AND key = ?", job)
		if err != nil {
			return bucket.DatabaseError(err)
		}
	}
	return nil
}
