package workspace

import "testing"

func TestStableJobIDDeterministic(t *testing.T) {
	a := StableJobID("my-job")
	b := StableJobID("my-job")
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
	if StableJobID("other") == a {
		t.Fatal("expected different ids for different inputs")
	}
}
