package health_check

import (
	"database/sql"
	"log"
	"maand/data"
	"maand/job_command"
	"time"
)

func Execute(tx *sql.Tx, wait bool, job string) error {
	commands := data.GetJobCommands(tx, job, "health_check")
	if len(commands) == 0 {
		log.Printf("health check is undefined, job %s", job)
		return nil
	}

	healthCheckFunc := func() error {
		for _, cmd := range commands {
			err := job_command.Execute(tx, job, cmd, "health_check", 1)
			if err != nil {
				return err
			}
		}
		return nil
	}

	var err error

	if wait {
		for i := 0; i < 30; i++ {
			time.Sleep(2 * time.Second)
			err = healthCheckFunc()
			if err == nil {
				log.Printf("health check is passed, job %s", job)
				return nil
			}
			log.Printf("health check is failed, job %s, retrying...", job)
		}
	} else {
		err = healthCheckFunc()
		if err == nil {
			log.Printf("health check is passed, job %s", job)
		} else {
			log.Printf("health check is failed, job %s", job)
		}
	}

	return err
}
