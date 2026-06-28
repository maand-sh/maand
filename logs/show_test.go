// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package logs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchesLineKV(t *testing.T) {
	line := `ts=2026-06-02T12:00:00.000Z event=command_begin run=abc worker=10.0.0.1 job=postgres phase=reconcile action=stop`
	assert.True(t, matchesLine(line, ShowOptions{Worker: "10.0.0.1", Job: "postgres", Phase: "reconcile"}))
	assert.False(t, matchesLine(line, ShowOptions{Event: "deploy_skip"}))
}

func TestMatchesLineJSON(t *testing.T) {
	line := `{"ts":"2026-06-02T12:00:00.000Z","event":"deploy_skip","job":"api","run":"abc"}`
	assert.True(t, matchesLine(line, ShowOptions{RunID: "abc", Job: "api", Event: "deploy_skip"}))
}

func TestParsePayloadLine(t *testing.T) {
	entry := ParseLine("| deleting jobs/postgres/patroni.yml")
	assert.Equal(t, "stdout", field(entry, "stream"))
	assert.Equal(t, "deleting jobs/postgres/patroni.yml", field(entry, "msg"))
}

func TestMatchesLineStructuredStream(t *testing.T) {
	line := `ts=2026-06-02T12:00:00.000Z run=r1 maand=deploy worker=10.0.0.1 job=cassandra phase=rsync stream=stdout msg="Transfer starting: 18 files"`
	assert.True(t, matchesLine(line, ShowOptions{Worker: "10.0.0.1", Job: "cassandra", Phase: "rsync"}))
	assert.False(t, matchesLine(line, ShowOptions{Worker: "10.0.0.2"}))
}

func TestGroupEntries(t *testing.T) {
	entries := []Entry{
		ParseLine(`ts=2026-06-02T12:00:00.000Z event=command_begin run=r1 maand=deploy worker=10.0.0.3 job=postgres phase=reconcile action=stop cmd="runner stop"`),
		ParseLine("| Container postgres  Stopping"),
		ParseLine(`ts=2026-06-02T12:00:01.000Z event=command_end run=r1 worker=10.0.0.3 job=postgres phase=reconcile action=stop exit=0 duration_ms=1500`),
	}
	blocks, events := groupEntries(entries)
	require.Len(t, blocks, 1)
	require.Empty(t, events)
	require.Len(t, blocks[0].Streams, 1)
	require.NotNil(t, blocks[0].End)
}

func TestRenderHuman(t *testing.T) {
	block := CommandBlock{
		Begin: ParseLine(`ts=2026-06-02T22:07:01.123Z event=command_begin run=abcdef12-0000 maand=deploy seq=17 worker=10.48.200.3 job=postgres phase=reconcile action=stop cmd="python3 runner.py stop postgres"`),
		Streams: []Entry{
			ParseLine("| Container postgres  Stopping"),
		},
		End: ptrEntry(ParseLine(`ts=2026-06-02T22:07:02.456Z event=command_end run=abcdef12-0000 worker=10.48.200.3 exit=0 duration_ms=1333`)),
	}
	out := RenderHuman([]CommandBlock{block}, nil)
	assert.Contains(t, out, "deploy")
	assert.Contains(t, out, "run=abcdef12")
	assert.Contains(t, out, "reconcile  stop  postgres@10.48.200.3")
	assert.Contains(t, out, "$ python3 runner.py stop postgres")
	assert.Contains(t, out, "postgres@10.48.200.3 | Container postgres  Stopping")
	assert.Contains(t, out, "ok")
	assert.Contains(t, out, "1.3s")
}

func ptrEntry(entry Entry) *Entry {
	return &entry
}

func TestContainsFieldQuotedJob(t *testing.T) {
	line := `job=postgres phase=rsync`
	assert.True(t, containsField(line, "job", "postgres"))
	assert.False(t, containsField(line, "job", "zookeeper"))
}

func TestShowJobFilterWithoutWorker(t *testing.T) {
	root := t.TempDir()
	origLocation := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = origLocation
		bucket.UpdatePath()
	})

	require.NoError(t, os.MkdirAll(bucket.LogLocation, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(bucket.LogLocation, "10.0.0.1.log"),
		[]byte(`ts=2026-06-02T12:00:00.000Z event=command_begin run=r1 maand=deploy worker=10.0.0.1 job=cassandra phase=rollout action=start cmd="make start"
| started
ts=2026-06-02T12:00:01.000Z event=command_end run=r1 worker=10.0.0.1 job=cassandra exit=0 duration_ms=100
`),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(bucket.LogLocation, "10.0.0.2.log"),
		[]byte(`ts=2026-06-02T12:00:02.000Z event=command_begin run=r1 maand=deploy worker=10.0.0.2 job=postgres phase=rollout action=start cmd="make start"
`),
		0o644,
	))

	paths, err := logPaths(ShowOptions{Job: "cassandra"})
	require.NoError(t, err)
	require.Len(t, paths, 2)

	var buf strings.Builder
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	require.NoError(t, Show(ShowOptions{Job: "cassandra", Format: FormatHuman}))
	require.NoError(t, w.Close())
	os.Stdout = oldStdout
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "cassandra@10.0.0.1")
	assert.NotContains(t, out, "postgres@10.0.0.2")
}
