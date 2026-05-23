package tests

import (
	"os"
	"testing"

	"maand/bucket"
)

func TestMain(m *testing.M) {
	bucket.Location = "./test_project"
	bucket.UpdatePath()

	os.Exit(m.Run())
	//_ = os.RemoveAll("./test_project")
}
