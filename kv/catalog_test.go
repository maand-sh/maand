package kv

import "testing"

import "github.com/stretchr/testify/assert"

func TestJobCatalogNamespace(t *testing.T) {
	assert.Equal(t, "maand/job/api", JobCatalogNamespace("api"))
}

func TestRolloutOrderKey(t *testing.T) {
	assert.Equal(t, "rollout_order", RolloutOrderKey)
}
