package kv

import "testing"

import "github.com/stretchr/testify/assert"

func TestJobCatalogNamespace(t *testing.T) {
	assert.Equal(t, "maand/job/api", JobCatalogNamespace("api"))
}

func TestDeployOrderKey(t *testing.T) {
	assert.Equal(t, "deploy_order", DeployOrderKey)
}
