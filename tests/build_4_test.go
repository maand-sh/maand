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

// valid workers.json
// no errors expected
func TestBuild_4(t *testing.T) {
	_ = os.RemoveAll(bucket.Location)

	err := initialize.Execute()
	assert.NoError(t, err)

	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte(`[{ "host": "10.0.0.1" },{ "host": "10.0.0.2" }]`), os.ModePerm)

	err = build.Execute()
	assert.NoError(t, err)

	row := GetRow("SELECT COUNT(*) FROM worker")
	var count int
	_ = row.Scan(&count)

	assert.Equal(t, count, 2)
}
