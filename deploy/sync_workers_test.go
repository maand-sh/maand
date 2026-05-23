package deploy

import (
	"strings"
	"testing"
)

func TestBuildRsyncFilterLines_singleJobFinalSync(t *testing.T) {
	lines := buildRsyncFilterLines([]string{"api"}, true)
	got := strings.Join(lines, "")
	want := "+ jobs/api/\n- jobs/*\n- jobs/**/*.tpl\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildRsyncFilterLines_stagingMultipleJobs(t *testing.T) {
	lines := buildRsyncFilterLines([]string{"api", "worker"}, true)
	got := strings.Join(lines, "")
	if !strings.Contains(got, "+ jobs/api/\n") || !strings.Contains(got, "+ jobs/worker/\n") {
		t.Fatalf("missing include rules: %q", got)
	}
	if !strings.HasSuffix(got, "- jobs/*\n- jobs/**/*.tpl\n") {
		t.Fatalf("missing exclude rules: %q", got)
	}
}

func TestBuildRsyncFilterLines_noJobRulesWhenApplyRulesFalse(t *testing.T) {
	lines := buildRsyncFilterLines([]string{"api"}, false)
	got := strings.Join(lines, "")
	want := "- jobs/**/*.tpl\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
