// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"
	"maand/utils"
	"maand/workspace"
)

func BuildJobs(tx *sql.Tx, jobWorkspace *workspace.DefaultWorkspace) ([]string, error) {
	workspaceJobNames, err := jobWorkspace.GetJobs()
	if err != nil {
		return nil, err
	}
	workspaceJobNames = sortedJobNames(workspaceJobNames)

	portAllocator, err := buildPortAllocator(tx, workspaceJobNames, jobWorkspace)
	if err != nil {
		return nil, err
	}

	for _, jobName := range workspaceJobNames {
		jobID := workspace.GetHashUUID(jobName)

		purgeChildTableQueries := []string{
			"DELETE FROM job_selectors WHERE job_id = ?",
			"DELETE FROM job_commands WHERE job_id = ?",
			"DELETE FROM job_files WHERE job_id = ?",
			"DELETE FROM job_certs WHERE job_id = ?",
		}

		for _, stmt := range purgeChildTableQueries {
			_, err := tx.Exec(stmt, jobID)
			if err != nil {
				return nil, bucket.DatabaseError(err)
			}
		}

		manifest, err := jobWorkspace.GetJobManifest(jobName)
		if err != nil {
			return nil, err
		}

		minCPUMHZ, err := utils.ExtractCPUFrequencyInMHz(workspace.GetMinCPU(manifest))
		if err != nil {
			return nil, fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, jobName, err)
		}
		maxCPUMHZ, err := utils.ExtractCPUFrequencyInMHz(workspace.GetMaxCPU(manifest))
		if err != nil {
			return nil, fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, jobName, err)
		}

		if minCPUMHZ != 0 && maxCPUMHZ == 0 {
			maxCPUMHZ = minCPUMHZ
		}
		if minCPUMHZ > maxCPUMHZ {
			return nil, fmt.Errorf("%w: job %s minCPUMhz > maxCPUMhz", bucket.ErrInvalidManifest, jobName)
		}

		minMemoryMB, err := utils.ExtractSizeInMB(workspace.GetMinMemory(manifest))
		if err != nil {
			return nil, fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, jobName, err)
		}
		maxMemoryMB, err := utils.ExtractSizeInMB(workspace.GetMaxMemory(manifest))
		if err != nil {
			return nil, fmt.Errorf("%w: job %s %w", bucket.ErrInvalidManifest, jobName, err)
		}
		if minMemoryMB != 0 && maxMemoryMB == 0 {
			maxMemoryMB = minMemoryMB
		}
		if minMemoryMB > maxMemoryMB {
			return nil, fmt.Errorf("%w: job %s minMemoryMb > maxMemoryMb", bucket.ErrInvalidManifest, jobName)
		}

		bucketJobsConfigFile, jobConfig, err := loadJobBucketConfig(jobName)
		if err != nil {
			return nil, err
		}

		requestedMemoryMB := maxMemoryMB
		if _, ok := jobConfig["memory"]; ok {
			requestedMemoryMB, err = utils.ExtractSizeInMB(jobConfig["memory"])
			if err != nil {
				return nil, err
			}

			if minMemoryMB == 0 && maxMemoryMB == 0 {
				minMemoryMB = requestedMemoryMB
				maxMemoryMB = requestedMemoryMB
			}

			if requestedMemoryMB > maxMemoryMB {
				return nil, fmt.Errorf("%w: %s, job %s max_memory_mb %.2f mb, requested %.2f mb", bucket.ErrUnsupportedResourceConfiguration, bucketJobsConfigFile, jobName, maxMemoryMB, requestedMemoryMB)
			}
			if requestedMemoryMB < minMemoryMB {
				return nil, fmt.Errorf("%w: %s, job %s min_memory_mb %.2f mb, requested %.2f mb", bucket.ErrUnsupportedResourceConfiguration, bucketJobsConfigFile, jobName, minMemoryMB, requestedMemoryMB)
			}
		}

		requestedCPUMHz := maxCPUMHZ
		if _, ok := jobConfig["cpu"]; ok {
			requestedCPUMHz, err = utils.ExtractCPUFrequencyInMHz(jobConfig["cpu"])
			if err != nil {
				return nil, err
			}

			if minCPUMHZ == 0 && maxCPUMHZ == 0 {
				minCPUMHZ = requestedCPUMHz
				maxCPUMHZ = requestedCPUMHz
			}

			if requestedCPUMHz > maxCPUMHZ {
				return nil, fmt.Errorf("%w: %s, job %s max_cpu_mhz %.2f mhz, requested %.2f mhz", bucket.ErrUnsupportedResourceConfiguration, bucketJobsConfigFile, jobName, maxCPUMHZ, requestedCPUMHz)
			}
			if requestedCPUMHz < minCPUMHZ {
				return nil, fmt.Errorf("%w: %s, job %s min_cpu_mhz %.2f mhz, requested %.2f mhz", bucket.ErrUnsupportedResourceConfiguration, bucketJobsConfigFile, jobName, minCPUMHZ, requestedCPUMHz)
			}
		}

		if err := workspace.ValidateHealthCheck(jobName, manifest); err != nil {
			return nil, err
		}
		if err := workspace.ValidatePrometheusServerFiles(jobName); err != nil {
			return nil, err
		}
		healthCheckJSON := ""
		if manifest.HealthCheck != nil && len(manifest.HealthCheck.Checks) > 0 {
			encoded, err := json.Marshal(manifest.HealthCheck)
			if err != nil {
				return nil, fmt.Errorf("%w: job %s health_check: %w", bucket.ErrInvalidManifest, jobName, err)
			}
			healthCheckJSON = string(encoded)
		}

		version := workspace.GetVersion(manifest)
		upsertJobQuery := `
			INSERT OR REPLACE INTO job (job_id, name, version, min_memory_mb, max_memory_mb, current_memory_mb, min_cpu_mhz, max_cpu_mhz, current_cpu_mhz, update_parallel_count, deploy_parallel_count, health_check)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = tx.Exec(
			upsertJobQuery, jobID, jobName, version,
			fmt.Sprintf("%v", minMemoryMB),
			fmt.Sprintf("%v", maxMemoryMB),
			fmt.Sprintf("%v", requestedMemoryMB),
			fmt.Sprintf("%v", minCPUMHZ),
			fmt.Sprintf("%v", maxCPUMHZ),
			fmt.Sprintf("%v", requestedCPUMHz),
			workspace.GetUpdateParallelCount(manifest),
			workspace.GetDeployParallelCount(manifest),
			healthCheckJSON,
		)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		for _, selector := range manifest.Selectors {
			_, err := tx.Exec("INSERT INTO job_selectors (job_id, selector) VALUES (?, ?)", jobID, selector)
			if err != nil {
				return nil, bucket.DatabaseError(err)
			}
		}

		if err := syncJobPorts(tx, jobID, jobName, manifest.Resources.Ports, portAllocator); err != nil {
			return nil, err
		}

		for _, command := range workspace.GetCommands(manifest) {
			if !strings.HasPrefix(command.Name, "command_") {
				return nil, fmt.Errorf("%w: invalid command name, name: %s, excepted prefix : %s", bucket.ErrInvalidJobCommandConfiguration, command.Name, "command_")
			}
			if len(command.ExecutedOn) == 0 {
				return nil, fmt.Errorf("%w: job %s, job_command %s missing executed_on", bucket.ErrInvalidJobCommandConfiguration, jobName, command.Name)
			}
			invalidExecutedOnEvents := utils.Difference(command.ExecutedOn, []string{
				"post_build", "health_check", "cli", "pre_deploy", "post_deploy", "job_control",
				"after_allocation_started", "after_allocation_stopped",
			})
			if len(invalidExecutedOnEvents) > 0 {
				return nil, fmt.Errorf("%w: job %s, job_command %s invalid executed_on %v", bucket.ErrInvalidJobCommandConfiguration, jobName, command.Name, invalidExecutedOnEvents)
			}
			if command.Demands.Job == jobName {
				return nil, fmt.Errorf("%w: job %s, job_command %s invalid configuration, self referencing", bucket.ErrInvalidJobCommandConfiguration, jobName, command.Name)
			}
			if err := workspace.ValidateDemandReference(jobName, command.Name, command); err != nil {
				return nil, err
			}
			modulesDir := path.Join(bucket.WorkspaceLocation, "jobs", jobName, "_modules")
			if _, _, err := jobcommand.ResolveCommandScript(modulesDir, command.Name); err != nil {
				return nil, fmt.Errorf("job %s command %s: %w", jobName, command.Name, err)
			}

			insertJobCommandQuery := `
				INSERT INTO job_commands (job_id, job, name, executed_on, demand_job, demand_command, demand_config)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`
			for _, executedOn := range command.ExecutedOn {
				demandConfigJSON, err := json.Marshal(command.Demands.Config)
				if err != nil {
					return nil, err
				}

				_, err = tx.Exec(insertJobCommandQuery, jobID, jobName, command.Name, executedOn, command.Demands.Job, command.Demands.Command, string(demandConfigJSON))
				if err != nil {
					return nil, bucket.DatabaseError(err)
				}
			}
		}

		_, err = os.Stat(path.Join(workspace.GetJobFilePath(jobName), "Makefile"))
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: job %s, Makefile not found", bucket.ErrInvalidJob, jobName)
		}

		_, err = os.Stat(path.Join(workspace.GetJobFilePath(jobName), "data"))
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: job %s, data directory is reserved", bucket.ErrInvalidJob, jobName)
		}

		_, err = os.Stat(path.Join(workspace.GetJobFilePath(jobName), "logs"))
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: job %s, logs directory is reserved", bucket.ErrInvalidJob, jobName)
		}

		_, err = os.Stat(path.Join(workspace.GetJobFilePath(jobName), "bin"))
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: job %s, bin directory is reserved", bucket.ErrInvalidJob, jobName)
		}

		insertJobFileQuery := `INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)`
		err = workspace.WalkJobFiles(jobName, func(path string, d fs.DirEntry, err error) error {
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
			_, err = tx.Exec(insertJobFileQuery, jobID, path, content, d.IsDir())
			if err != nil {
				return bucket.DatabaseError(err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}

		// hash update manifest's cert config
		certData, err := json.Marshal(manifest.Certs)
		if err != nil {
			return nil, err
		}

		currentHash, err := utils.MD5Content(certData)
		if err != nil {
			return nil, err
		}

		err = data.UpdateHash(tx, "build_certs", jobName, currentHash)
		if err != nil {
			return nil, err
		}

		insertJobCertQuery := `INSERT INTO job_certs (job_id, name, pkcs8, one, subject) VALUES (?, ?, ?, ?, ?)`
		for name, config := range manifest.Certs {
			subject, err := json.Marshal(config.Subject)
			if err != nil {
				return nil, err
			}
			_, err = tx.Exec(insertJobCertQuery, jobID, name, config.PKCS8, config.One, subject)
			if err != nil {
				return nil, bucket.DatabaseError(err)
			}
		}
	}

	databaseJobNames, err := data.GetJobs(tx)
	if err != nil {
		return nil, err
	}

	jobsToRemove := utils.Difference(databaseJobNames, workspaceJobNames)
	removedJobs := make([]string, 0, len(jobsToRemove))
	for _, jobName := range jobsToRemove {
		removedJobs = append(removedJobs, jobName)
		jobID := workspace.GetHashUUID(jobName)
		for _, stmt := range []string{
			"DELETE FROM job_ports WHERE job_id = ?",
			"DELETE FROM job_selectors WHERE job_id = ?",
			"DELETE FROM job_commands WHERE job_id = ?",
			"DELETE FROM job_files WHERE job_id = ?",
			"DELETE FROM job_certs WHERE job_id = ?",
		} {
			if _, err := tx.Exec(stmt, jobID); err != nil {
				return nil, bucket.DatabaseError(err)
			}
		}
		_, err := tx.Exec("DELETE FROM job WHERE name = ?", jobName)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
		_, err = tx.Exec("DELETE FROM hash WHERE namespace = 'build_certs' AND key = ?", jobName)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}
	}
	return removedJobs, nil
}
