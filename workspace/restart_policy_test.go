// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRestartPolicy(t *testing.T) {
	policy, err := NormalizeRestartPolicy("")
	require.NoError(t, err)
	assert.Equal(t, RestartPolicyAlways, policy)

	policy, err = NormalizeRestartPolicy("reload")
	require.NoError(t, err)
	assert.Equal(t, RestartPolicyReload, policy)

	_, err = NormalizeRestartPolicy("bounce")
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
}

func TestValidateRestartPolicy(t *testing.T) {
	require.NoError(t, ValidateRestartPolicy("api", Manifest{RestartPolicy: "never"}))
	err := ValidateRestartPolicy("api", Manifest{RestartPolicy: "invalid"})
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
}

func TestValidateRestartGlobsRequiresReloadPolicy(t *testing.T) {
	err := ValidateRestartPolicy("api", Manifest{
		RestartPolicy: "always",
		RestartGlobs:  []string{"Makefile"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifest)
}

func TestValidateRestartGlobsPatterns(t *testing.T) {
	require.NoError(t, ValidateRestartPolicy("api", Manifest{
		RestartPolicy: "reload",
		RestartGlobs:  []string{"Makefile", "bin/**"},
	}))
	err := ValidateRestartPolicy("api", Manifest{
		RestartPolicy: "reload",
		RestartGlobs:  []string{"["},
	})
	require.Error(t, err)
}

func TestEncodeParseRestartGlobs(t *testing.T) {
	encoded, err := EncodeRestartGlobs([]string{"Makefile", "bin/**"})
	require.NoError(t, err)
	globs, err := ParseRestartGlobs(encoded)
	require.NoError(t, err)
	assert.Equal(t, []string{"Makefile", "bin/**"}, globs)
}
