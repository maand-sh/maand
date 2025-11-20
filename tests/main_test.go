package tests

import (
	"maand/bucket"
	"testing"
)

func TestMain(m *testing.M) {
	bucket.Location = "./test_project"
	bucket.UpdatePath()
	m.Run()
	//_ = os.RemoveAll("./test_project")
}
