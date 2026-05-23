package jobcontrol

import (
	"errors"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnerCommandFormat(t *testing.T) {
	cmd := runnerCommand("bucket-1", TargetStart, "myjob")
	assert.Contains(t, cmd, "runner.py bucket-1 start --jobs myjob")
}

func TestJobRunErrorIsRunCommand(t *testing.T) {
	err := &JobRunError{
		Job:    "app",
		Target: TargetStop,
		Failures: []WorkerFailure{
			{WorkerIP: "10.0.0.1", Err: errors.New("ssh failed")},
		},
	}
	assert.ErrorIs(t, err, bucket.ErrRunCommand)
}

func TestControlErrorAggregatesJobs(t *testing.T) {
	err := newControlError([]JobRunError{
		{Job: "a", Target: TargetStart, Err: errors.New("one")},
		{Job: "b", Target: TargetStart, Err: errors.New("two")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job a")
	assert.Contains(t, err.Error(), "job b")
}
