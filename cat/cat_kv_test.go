package cat

import (
	"io"
	"os"
	"testing"
	"time"

	"maand/bucket"
	"maand/data"
	"maand/initialize"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKVGetRevealSecret(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())
	require.NoError(t, kv.EnsureEncryptionKey())
	kv.ResetEncryptionKeyCacheForTest()
	t.Cleanup(kv.ResetEncryptionKeyCacheForTest)

	encrypted, err := kv.EncryptPlaintext("root-token-secret")
	require.NoError(t, err)

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(
		`INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
		 VALUES (?, ?, ?, 1, 0, ?, 0)`,
		"secrets/job/vault", "root_token", encrypted, time.Now().Unix(),
	)
	require.NoError(t, err)

	stdout := captureStdout(t, func() {
		require.NoError(t, KVGet("secrets/job/vault", "root_token", true))
	})
	assert.Contains(t, stdout, "value: root-token-secret")

	stdout = captureStdout(t, func() {
		require.NoError(t, KVGet("secrets/job/vault", "root_token", false))
	})
	assert.Contains(t, stdout, "value: [encrypted]")
}

func TestKVListWhere(t *testing.T) {
	where, err := kvListWhere(false, false, nil)
	require.NoError(t, err)
	assert.Equal(t, "", where)

	where, err = kvListWhere(true, false, nil)
	require.NoError(t, err)
	assert.Equal(t, " WHERE deleted = 0", where)

	where, err = kvListWhere(false, true, nil)
	require.NoError(t, err)
	assert.Equal(t, " WHERE deleted = 1", where)

	where, err = kvListWhere(false, false, []string{"vars/job/vault", "secrets/job/vault"})
	require.NoError(t, err)
	assert.Contains(t, where, "namespace IN ('vars/job/vault','secrets/job/vault')")

	where, err = kvListWhere(true, false, []string{"vars/job/vault"})
	require.NoError(t, err)
	assert.Contains(t, where, "namespace IN ('vars/job/vault')")
	assert.Contains(t, where, "deleted = 0")

	_, err = kvListWhere(true, true, nil)
	assert.Error(t, err)
}

func TestKVActiveOnlyNotFoundWhenAllDeleted(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(
		`INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
		 VALUES (?, ?, ?, 1, 0, ?, 1)`,
		"maand/job/gone", "name", "gone", time.Now().Unix(),
	)
	require.NoError(t, err)

	assert.Error(t, KV("", true, false))
	assert.NoError(t, KV("", false, true))
}

func TestKVJobFilter(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-vault', 'vault', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '')`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '0', '0', 0)`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('alloc-1', '10.0.0.1', 'vault', 0, 0, 0)`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO job_commands (job, name, executed_on, demand_job, demand_command, demand_config)
		VALUES ('vault', 'init', 'pre_deploy', 'postgres', 'status', '')`)
	require.NoError(t, err)

	now := time.Now().Unix()
	for _, row := range []struct{ ns, key, val string }{
		{"vars/job/vault", "cluster_initialized", "true"},
		{"secrets/job/vault", "root_token", "enc:v1:x"},
		{"maand/job/vault", "version", "1.0.0"},
		{"maand/job/vault/worker/10.0.0.1", "vault_allocation_index", "0"},
		{"maand/bucket", "bucket_id", "bucket-1"},
		{"vars/bucket", "port_min", "1024"},
		{"maand/worker/10.0.0.1", "worker_ip", "10.0.0.1"},
		{"vars/job/postgres", "version", "2.0.0"},
		{"vars/job/api", "name", "api"},
	} {
		_, err = db.Exec(`
			INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
			VALUES (?, ?, ?, 1, 0, ?, 0)`, row.ns, row.key, row.val, now)
		require.NoError(t, err)
	}

	stdout := captureStdout(t, func() {
		require.NoError(t, KV("vault", false, false))
	})
	assert.Contains(t, stdout, "vars/job/vault")
	assert.Contains(t, stdout, "maand")
	assert.Contains(t, stdout, "vars/bucket")
	assert.Contains(t, stdout, "maand/worker/10.0.0.1")
	assert.Contains(t, stdout, "vars/job/postgres")
	assert.NotContains(t, stdout, "vars/job/api")
	assert.Error(t, KV("missing", false, false))
}

func TestKVJobFilterDisabledJob(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-vault', 'vault', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '')`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('alloc-1', '10.0.0.1', 'vault', 1, 0, 0)`)
	require.NoError(t, err)

	now := time.Now().Unix()
	_, err = db.Exec(`
		INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
		VALUES ('vars/job/vault', 'cluster_initialized', 'true', 1, 0, ?, 0),
		       ('maand/bucket', 'bucket_id', 'bucket-1', 1, 0, ?, 0)`, now, now)
	require.NoError(t, err)

	stdout := captureStdout(t, func() {
		require.NoError(t, KV("vault", false, false))
	})
	assert.Contains(t, stdout, "vars/job/vault")
	assert.Contains(t, stdout, "maand")
	assert.NotContains(t, stdout, "vars/job/api")
}

func TestKVJobFilterRemovedOnlyReturnsNotFound(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-gone', 'gone', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '')`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		VALUES ('alloc-1', '10.0.0.1', 'gone', 0, 1, 0)`)
	require.NoError(t, err)

	now := time.Now().Unix()
	_, err = db.Exec(`
		INSERT INTO key_value (namespace, key, value, version, ttl, created_date, deleted)
		VALUES ('maand/bucket', 'bucket_id', 'bucket-1', 1, 0, ?, 0)`, now)
	require.NoError(t, err)

	assert.Error(t, KV("gone", false, false))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	require.NoError(t, w.Close())
	os.Stdout = old

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	_ = r.Close()
	return string(out)
}
