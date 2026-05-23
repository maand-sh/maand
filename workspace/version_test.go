package workspace

import (
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	v, err := ParseVersion("2.1.0")
	require.NoError(t, err)
	assert.Equal(t, 2, v.Major)
	assert.Equal(t, 1, v.Minor)
	assert.Equal(t, 0, v.Patch)

	v, err = ParseVersion("v1")
	require.NoError(t, err)
	assert.Equal(t, 1, v.Major)

	_, err = ParseVersion("unknown")
	assert.ErrorIs(t, err, bucket.ErrInvalidJobVersion)

	_, err = ParseVersion("1.2.3.4")
	assert.ErrorIs(t, err, bucket.ErrInvalidJobVersion)
}

func TestVersionCompare(t *testing.T) {
	a, _ := ParseVersion("1.2.0")
	b, _ := ParseVersion("1.10.0")
	c, _ := ParseVersion("1.2.0-rc1")
	d, _ := ParseVersion("1.2.0")

	assert.Equal(t, -1, a.Compare(b))
	assert.Equal(t, 0, a.Compare(d))
	assert.Equal(t, -1, c.Compare(d))
}

func TestVersionConstraint(t *testing.T) {
	upstream, err := ParseVersion("2.0.0")
	require.NoError(t, err)

	constraint, err := ParseVersionConstraint(map[string]interface{}{
		"min_version": "1.0.0",
		"max_version": "2.5.0",
	})
	require.NoError(t, err)
	require.NoError(t, constraint.Satisfies(upstream))

	constraint, err = ParseVersionConstraint(map[string]interface{}{"min_version": 3})
	require.NoError(t, err)
	assert.ErrorIs(t, constraint.Satisfies(upstream), bucket.ErrJobCommandDemandVersionMismatch)
}

func TestValidateDemandReference(t *testing.T) {
	cmd := JobCommand{Name: "command_x"}
	cmd.Demands.Job = "db"
	err := ValidateDemandReference("api", cmd.Name, cmd)
	assert.ErrorIs(t, err, bucket.ErrInvalidJobCommandDemand)

	cmd.Demands.Command = "command_schema"
	require.NoError(t, ValidateDemandReference("api", cmd.Name, cmd))
}
