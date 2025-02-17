package build

import (
	"database/sql"
	"encoding/json"
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

func Jobs(tx *sql.Tx, ws *workspace.DefaultWorkspace) {

	for _, job := range ws.GetJobs() {
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
			utils.Check(err)
		}

		manifest := ws.GetJobManifest(job)

		minCPUMhz, err := utils.ExtractCPUFrequencyInMHz(workspace.GetMinCPU(manifest))
		utils.Check(err)
		maxCPUMhz, err := utils.ExtractCPUFrequencyInMHz(workspace.GetMaxCPU(manifest))
		utils.Check(err)

		if minCPUMhz != 0 && maxCPUMhz == 0 {
			maxCPUMhz = minCPUMhz
		}
		if minCPUMhz > maxCPUMhz {
			panic(fmt.Errorf("job %s, minCPUMhz > maxCPUMhz: %f", job, minCPUMhz))
		}

		minMemoryMb, err := utils.ExtractSizeInMB(workspace.GetMinMemory(manifest))
		utils.Check(err)
		maxMemoryMb, err := utils.ExtractSizeInMB(workspace.GetMaxMemory(manifest))
		utils.Check(err)

		if minMemoryMb != 0 && maxMemoryMb == 0.0 {
			maxMemoryMb = minMemoryMb
		}
		if minMemoryMb > maxMemoryMb {
			panic(fmt.Errorf("job %s, minMemoryMb > maxMemoryMb: %f", job, minMemoryMb))
		}

		version := workspace.GetVersion(manifest)
		query := `
			INSERT OR REPLACE INTO job (job_id, name, version, min_memory_mb, max_memory_mb, min_cpu_mhz, max_cpu_mhz, update_parallel_count) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = tx.Exec(
			query, jobID, job, version,
			fmt.Sprintf("%v", minMemoryMb),
			fmt.Sprintf("%v", maxMemoryMb),
			fmt.Sprintf("%v", minCPUMhz),
			fmt.Sprintf("%v", maxCPUMhz),
			workspace.GetUpdateParallelCount(manifest),
		)
		utils.Check(err)

		for _, selector := range manifest.Selectors {
			_, err := tx.Exec("INSERT INTO job_selectors (job_id, selector) VALUES (?, ?)", jobID, selector)
			utils.Check(err)
		}

		for name, port := range manifest.Resources.Ports {

			prefix := fmt.Sprintf("%s_port_", job)
			if !strings.HasPrefix(name, prefix) {
				panic(fmt.Sprintf("invalid port name, name: %s, should have prefix : %s", name, prefix))
			}

			var portUsedJob string
			row := tx.QueryRow("SELECT (SELECT name FROM job WHERE job_id = jp.job_id) as job FROM job_ports jp WHERE port = ?", port)
			_ = row.Scan(&portUsedJob)
			if portUsedJob != "" {
				panic(fmt.Sprintf("port can't be reused, port: %d, jobs (%s, %s)", port, job, portUsedJob))
			}

			_, err = tx.Exec("INSERT INTO job_ports (job_id, name, port) VALUES (?, ?, ?)", jobID, name, port)
			utils.Check(err)
		}

		for _, command := range workspace.GetCommands(manifest) {
			if !strings.HasPrefix(command.Name, "command_") {
				panic(fmt.Sprintf("invalid command name, name: %s, should have prefix : %s", command.Name, "command_"))
			}
			if len(command.ExecutedOn) == 0 {
				panic(fmt.Sprintf("job %s, job_command %s required have a executed_on", job, command.Name))
			}
			diffs := utils.Difference(command.ExecutedOn, []string{"post_build", "health_check", "direct", "pre_deploy", "post_deploy"})
			if len(diffs) > 0 {
				panic(fmt.Sprintf("job %s, job_command %s not valid executed_on %v", job, command.Name, diffs))
			}
			if command.DependsOn.Job == job {
				panic(fmt.Sprintf("job %s, job_command %s invalid configuration, self referencing", job, command.Name))
			}

			commandPath := path.Join(bucket.WorkspaceLocation, "jobs", job, "_modules", fmt.Sprintf("%s.py", command.Name))
			if _, err := os.Stat(commandPath); os.IsNotExist(err) {
				panic(fmt.Sprintf("%s does not exist, registered for job %s", commandPath, job))
			}
			query := `
				INSERT INTO job_commands (job_id, job, name, executed_on, depend_on_job, depend_on_command, depend_on_config) 
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`
			for _, executedOn := range command.ExecutedOn {
				jsonData, err := json.Marshal(command.DependsOn.Config)
				utils.Check(err)

				_, err = tx.Exec(query, jobID, job, command.Name, executedOn, command.DependsOn.Job, command.DependsOn.Command, string(jsonData))
				utils.Check(err)
			}
		}

		if _, err := os.Stat(path.Join(workspace.GetJobFilePath(job), "Makefile")); os.IsNotExist(err) {
			utils.Check(fmt.Errorf("makefile is required, job %s", job))
		}

		query = `INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)`
		err = workspace.WalkJobFiles(job, func(path string, d fs.DirEntry, err error) error {
			utils.Check(err)

			var data = []byte("")
			if !d.IsDir() {
				data, err = os.ReadFile(workspace.GetJobFilePath(path))
				utils.Check(err)
			}
			_, err = tx.Exec(query, jobID, path, data, d.IsDir())
			utils.Check(err)
			return err
		})
		utils.Check(err)

		// hash update manifest's cert config
		certData, err := json.Marshal(manifest.Certs)
		utils.Check(err)
		currentHash, err := utils.MD5Content(certData)
		utils.Check(err)
		utils.UpdateHash(tx, "build_certs", job, currentHash)

		query = `INSERT INTO job_certs (job_id, name, pkcs8, subject) VALUES (?, ?, ?, ?)`
		for name, config := range manifest.Certs {
			_, err = tx.Exec(query, jobID, name, config.PKCS8, config.Subject)
			utils.Check(err)
		}
	}

	workspaceJobs := ws.GetJobs()
	availableJobs := data.GetJobs(tx)

	// remove missing jobs
	diffs := utils.Difference(availableJobs, workspaceJobs)
	removedJobs = []string{}
	for _, job := range diffs {
		removedJobs = append(removedJobs, job)
		_, err := tx.Exec("DELETE FROM job WHERE name = ?", job)
		utils.Check(err)
		_, err = tx.Exec("DELETE FROM hash WHERE namespace = 'build_certs' AND key = ?", job)
		utils.Check(err)
	}
}
