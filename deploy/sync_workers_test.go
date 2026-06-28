package deploy

import (
	"os"
	"strings"
	"sync"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteRsyncFilter_writesMergeFile(t *testing.T) {
	env := setupDeployTestEnv(t)
	mergePath, err := writeRsyncFilter("10.0.0.1", []string{"api", "worker"}, true)
	require.NoError(t, err)
	require.Contains(t, mergePath, "10.0.0.1.rsync")

	data, err := os.ReadFile(mergePath)
	require.NoError(t, err)
	content := string(data)
	require.Contains(t, content, "+ jobs/api/")
	require.Contains(t, content, "+ jobs/worker/")
	require.Contains(t, content, "- jobs/*")
	_ = env
}

func TestSyncWorkers_usesHooks(t *testing.T) {
	env := setupDeployTestEnv(t)
	var (
		synced []string
		mu     sync.Mutex
	)
	SetTestHooks(&TestHooks{
		Rsync: func(_ *bucket.Runtime, _, workerIP string, _ []string) error {
			mu.Lock()
			synced = append(synced, workerIP)
			mu.Unlock()
			return nil
		},
		WorkerCommand: func(*bucket.Runtime, string, bucket.CommandContext, []string, []string) error { return nil },
	})
	t.Cleanup(ClearTestHooks)

	require.NoError(t, syncWorkers(nil, env.bucketID, []string{"10.0.0.1", "10.0.0.2"}, []string{"app"}, true))
	assert.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, synced)
}

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
