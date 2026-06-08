package tests

import (
	"os"
	"testing"

	"maand/bucket"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("MAAND_TEST", "1")
	_ = os.Setenv("MAAND_QUIET", "1")

	dir, err := os.MkdirTemp("", "maand-tests-*")
	if err != nil {
		panic(err)
	}
	bucket.Location = dir
	bucket.UpdatePath()

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
