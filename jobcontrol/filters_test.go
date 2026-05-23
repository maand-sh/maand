package jobcontrol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFiltersTrimsAndDedupes(t *testing.T) {
	f := ParseFilters(" app , app , b ", " 10.0.0.1 , 10.0.0.1 ")
	assert.Equal(t, []string{"app", "b"}, f.Jobs)
	assert.Equal(t, []string{"10.0.0.1"}, f.Workers)
}

func TestFiltersValidateAgainst(t *testing.T) {
	f := Filters{Jobs: []string{"missing"}}
	err := f.validateAgainst([]string{"app"}, []string{"10.0.0.1"})
	require.Error(t, err)
	var invalid *InvalidFilterError
	assert.ErrorAs(t, err, &invalid)
	assert.Equal(t, "jobs", invalid.Kind)
}

func TestSelectJobs(t *testing.T) {
	got := selectJobs([]string{"a", "b", "c"}, []string{"b"})
	assert.Equal(t, []string{"b"}, got)
}

func TestWorkerSelected(t *testing.T) {
	assert.True(t, workerSelected("10.0.0.1", nil))
	assert.False(t, workerSelected("10.0.0.2", []string{"10.0.0.1"}))
}
