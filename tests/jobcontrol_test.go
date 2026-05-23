package tests

import (
	"testing"

	"maand/jobcontrol"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobControlRejectsInvalidTarget(t *testing.T) {
	err := jobcontrol.Execute("app", "", "bad target", false)
	require.Error(t, err)
	var invalid *jobcontrol.InvalidTargetError
	assert.ErrorAs(t, err, &invalid)
}

func TestJobControlRejectsUnknownJobFilter(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "app", `{"selectors":["worker"]}`)
	runBuild(t)

	err := jobcontrol.Execute("missing-job", "", "start", false)
	require.Error(t, err)
	var invalid *jobcontrol.InvalidFilterError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "jobs", invalid.Kind)
}

func TestJobControlRejectsUnknownWorkerFilter(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)
	writeMinimalJob(t, "app", `{"selectors":["worker"]}`)
	runBuild(t)

	err := jobcontrol.Execute("app", "192.0.0.99", "start", false)
	require.Error(t, err)
	var invalid *jobcontrol.InvalidFilterError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "workers", invalid.Kind)
}
