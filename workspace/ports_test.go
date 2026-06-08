package workspace

import (
	"encoding/json"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestPortsUnmarshalEmptyObject(t *testing.T) {
	var manifest Manifest
	require.NoError(t, json.Unmarshal([]byte(`{
		"resources": {"ports": {"database_port": {}, "http_port": {}}}
	}`), &manifest))
	assert.Len(t, manifest.Resources.Ports, 2)
	assert.True(t, manifest.Resources.Ports["database_port"].Provisioned())
	assert.True(t, manifest.Resources.Ports["http_port"].Provisioned())
	assert.Equal(t, []string{"database_port", "http_port"}, manifest.Resources.Ports.Names())
}

func TestManifestPortsUnmarshalInteger(t *testing.T) {
	var manifest Manifest
	require.NoError(t, json.Unmarshal([]byte(`{
		"resources": {"ports": {"database_port": 5432, "http_port": {}}}
	}`), &manifest))
	require.False(t, manifest.Resources.Ports["database_port"].Provisioned())
	require.NotNil(t, manifest.Resources.Ports["database_port"].Fixed)
	assert.Equal(t, 5432, *manifest.Resources.Ports["database_port"].Fixed)
	assert.True(t, manifest.Resources.Ports["http_port"].Provisioned())
}

func TestManifestPortsRejectInvalidValue(t *testing.T) {
	var manifest Manifest
	err := json.Unmarshal([]byte(`{
		"resources": {"ports": {"database_port": "5432"}}
	}`), &manifest)
	require.Error(t, err)
	assert.ErrorIs(t, err, bucket.ErrInvalidManifestPort)
}

func TestValidatePortKey(t *testing.T) {
	require.NoError(t, ValidatePortKey("database_port"))
	require.Error(t, ValidatePortKey(""))
	require.Error(t, ValidatePortKey("Bad-Port"))
}
