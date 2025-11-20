package tests

import (
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/build"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
)

// without workers.json
// no errors expected
func TestBuild_2(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.Remove(path.Join(bucket.WorkspaceLocation, "workers.json"))
	err = build.Execute()

	assert.NoError(t, err)
}
