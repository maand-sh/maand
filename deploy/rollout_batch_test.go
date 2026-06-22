package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveBatchSize(t *testing.T) {
	assert.Equal(t, 5, effectiveBatchSize(0, 5))
	assert.Equal(t, 2, effectiveBatchSize(2, 5))
	assert.Equal(t, 3, effectiveBatchSize(10, 3))
}

func TestBatchCount(t *testing.T) {
	assert.Equal(t, 0, batchCount(0, 1))
	assert.Equal(t, 1, batchCount(3, 0))
	assert.Equal(t, 3, batchCount(3, 1))
	assert.Equal(t, 2, batchCount(3, 2))
}
