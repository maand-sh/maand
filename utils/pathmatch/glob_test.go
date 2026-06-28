// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package pathmatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatch(t *testing.T) {
	assert.True(t, Match("Makefile", "Makefile"))
	assert.True(t, Match("*.yaml", "rules/alerts.yaml"))
	assert.True(t, Match("bin/**", "bin/app"))
	assert.True(t, Match("docker-compose.yml", "docker-compose.yml"))
	assert.False(t, Match("bin/**", "config/app.toml"))
}

func TestMatchAny(t *testing.T) {
	matched := MatchAny(
		[]string{"Makefile", "bin/**"},
		[]string{"config/app.toml", "bin/runner", "Makefile"},
	)
	assert.Equal(t, []string{"bin/runner", "Makefile"}, matched)
}

func TestValidatePattern(t *testing.T) {
	require.NoError(t, ValidatePattern("bin/**"))
	require.Error(t, ValidatePattern(""))
	require.Error(t, ValidatePattern("["))
}
