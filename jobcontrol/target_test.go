package jobcontrol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTargetValid(t *testing.T) {
	for _, raw := range []string{"start", "STOP", " Restart ", "migrate", "pause", "db-migrate.v2"} {
		target, err := ParseTarget(raw)
		require.NoError(t, err)
		assert.NotEmpty(t, target)
	}
}

func TestParseTargetCustomPreservesCase(t *testing.T) {
	target, err := ParseTarget("Migrate")
	require.NoError(t, err)
	assert.Equal(t, Target("Migrate"), target)
}

func TestParseTargetInvalid(t *testing.T) {
	for _, raw := range []string{"", " ", "bad target", "foo;bar", "rm -rf"} {
		_, err := ParseTarget(raw)
		require.Error(t, err, raw)
		var invalid *InvalidTargetError
		assert.ErrorAs(t, err, &invalid)
	}
}
