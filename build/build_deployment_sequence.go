package build

import (
	"database/sql"
	"maand/utils"
)

func DeploymentSequence(tx *sql.Tx) {
	query := `
		 WITH RECURSIVE job_command_seq AS (
			SELECT jc.job, 0 AS level FROM job_commands jc WHERE jc.depend_on_job = ''

			UNION ALL

			SELECT jc.job, jcs.level + 1 AS level
			FROM job_commands jc INNER JOIN job_command_seq jcs ON jc.depend_on_job = jcs.job
		)
		UPDATE allocations SET deployment_seq = t.deployment_seq FROM (
		SELECT
			DISTINCT job, deployment_seq
		FROM
			(SELECT job, (SELECT MAX(level) FROM job_command_seq jcs WHERE jcs.job = t.job) as deployment_seq FROM job_command_seq t) t1 
		ORDER BY deployment_seq) t WHERE allocations.job = t.job;
	`
	_, err := tx.Exec(query)
	utils.Check(err)
}
