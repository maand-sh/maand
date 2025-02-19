package run_command

type RunCommandError struct {
	Err map[string]error
}

func (e *RunCommandError) Error() string {
	return "Run command failed"
}
