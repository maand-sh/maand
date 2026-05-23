package deploy

import "testing"

func TestSelectJobsForDeploy(t *testing.T) {
	got := selectJobsForDeploy([]string{"a", "b", "c"}, []string{"b"})
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("got %#v", got)
	}
}

func TestSelectJobsForDeployAll(t *testing.T) {
	got := selectJobsForDeploy([]string{"a", "b"}, nil)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestNormalizeJobFilter_dedupes(t *testing.T) {
	got := normalizeJobFilter([]string{"a", "a", "b"})
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestParseJobsCSV(t *testing.T) {
	got := parseJobsCSV(" a , b ,, c ")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("got %#v", got)
	}
	if parseJobsCSV("  ") != nil {
		t.Fatal("expected nil for blank csv")
	}
}
