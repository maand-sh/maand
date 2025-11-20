package tests

import (
	"os"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
)

// empty workers.json
// no errors expected
func TestBuild_1(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	err = build.Execute()
	assert.NoError(t, err)
}
