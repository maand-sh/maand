package job_command

import "fmt"

type JobCommandNotFoundError struct {
	Job     string
	Command string
	Event   string
}

func (e *JobCommandNotFoundError) Error() string {
	return fmt.Sprintf("Job '%s' command '%s' event '%s' not found", e.Job, e.Command, e.Event)
}

type JobCommandError struct {
	Job     string
	Command string
	Err     map[string]error
}

func (e *JobCommandError) Error() string {
	return fmt.Sprintf("Job '%s' command '%s' failed", e.Job, e.Command)
}
