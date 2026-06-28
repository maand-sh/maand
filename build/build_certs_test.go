// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/initialize"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBuildCertSecrets(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	bucket.Location = dir
	bucket.UpdatePath()
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte("[]"), 0o644))
	require.NoError(t, initialize.Execute())
}

func TestGenerateCert_pkcs8(t *testing.T) {
	setupBuildCertSecrets(t)

	certDir := t.TempDir()
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "test"}, []string{"10.0.0.1"}, 30, true))

	keyPEM, err := os.ReadFile(path.Join(certDir, "tls.key"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	assert.Equal(t, "PRIVATE KEY", block.Type)
}

func seedBuildCertsDB(t *testing.T, db *sql.DB, sharedCert bool) {
	t.Helper()
	subject := `{"common_name":"api"}`
	one := 0
	if sharedCert {
		one = 1
	}
	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 0);
		INSERT INTO worker (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position)
		VALUES ('w1', '10.0.0.1', '1024', '2000', 0),
		       ('w2', '10.0.0.2', '1024', '2000', 1);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_certs (job_id, name, pkcs8, one, subject)
		VALUES ('job-api', 'tls', 1, ?, ?);
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'api', 0, 0, 0, '0.0.0'),
		       ('a2', '10.0.0.2', 'api', 0, 0, 0, '0.0.0');
	`, one, subject)
	require.NoError(t, err)
}

func TestBuildCerts_generatesPerWorkerCerts(t *testing.T) {
	setupBuildCertSecrets(t)
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	seedBuildCertsDB(t, db, false)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, BuildCerts(tx))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	for _, workerIP := range []string{"10.0.0.1", "10.0.0.2"} {
		ns := "maand/job/api/worker/" + workerIP
		crt, err := store.Get(ns, "certs/tls.crt")
		require.NoError(t, err)
		assert.NotEmpty(t, crt.Value)
		key, err := store.Get(ns, "certs/tls.key")
		require.NoError(t, err)
		assert.NotEmpty(t, key.Value)
	}
}

func TestCertNeedsRegeneration_jobRegenerate(t *testing.T) {
	needs, err := certNeedsRegeneration("api", "tls", true, []string{"10.0.0.1"}, 30)
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestBuildSharedCertPEM_reusesStoredKV(t *testing.T) {
	setupBuildCertSecrets(t)
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	certDir := t.TempDir()
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "api"}, []string{"10.0.0.1", "10.0.0.2"}, 365, true))
	certPEM, err := os.ReadFile(path.Join(certDir, "tls.crt"))
	require.NoError(t, err)
	keyPEM, err := os.ReadFile(path.Join(certDir, "tls.key"))
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	ns := "maand/job/api/worker/10.0.0.1"
	kv.GetKVStore().Put(ns, "certs/tls.crt", string(certPEM), 0)
	kv.GetKVStore().Put(ns, "certs/tls.key", string(keyPEM), 0)
	require.NoError(t, tx.Commit())

	spec := jobCertSpec{name: "tls", pkcs8: true, one: true, subject: pkix.Name{CommonName: "api"}}
	gotCert, gotKey, err := buildSharedCertPEM("api", "jobs/api", spec, []string{"10.0.0.1", "10.0.0.2"}, false, 365)
	require.NoError(t, err)
	assert.Contains(t, string(gotCert), "BEGIN CERTIFICATE")
	assert.Contains(t, string(gotKey), "BEGIN PRIVATE KEY")
}

func TestBuildCerts_purgesCertKVWhenJobHasNoSpecs(t *testing.T) {
	setupBuildCertSecrets(t)
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 0);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'api', 0, 0, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.crt", "OLD", 0)
	require.NoError(t, BuildCerts(tx))
	require.NoError(t, tx.Commit())

	keys, err := kv.GetKVStore().GetKeys("maand/job/api/worker/10.0.0.1")
	require.NoError(t, err)
	assert.NotContains(t, keys, "certs/tls.crt")
}

func TestBuildCerts_purgesCertKVWhenAllAllocationsRemoved(t *testing.T) {
	setupBuildCertSecrets(t)
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.Exec(`
		INSERT INTO bucket (bucket_id, update_seq) VALUES ('bucket-1', 0);
		INSERT INTO job (
			job_id, name, version,
			min_memory_mb, max_memory_mb, current_memory_mb,
			min_cpu_mhz, max_cpu_mhz, current_cpu_mhz,
			update_parallel_count, health_check
		) VALUES ('job-api', 'api', '1.0.0', '0', '0', '0', '0', '0', '0', 1, '');
		INSERT INTO job_certs (job_id, name, pkcs8, one, subject)
		VALUES ('job-api', 'tls', 1, 0, '{"common_name":"api"}');
		INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq, new_version)
		VALUES ('a1', '10.0.0.1', 'api', 0, 1, 0, '0.0.0');
	`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.crt", "OLD", 0)
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.key", "OLD-KEY", 0)
	require.NoError(t, BuildCerts(tx))
	require.NoError(t, tx.Commit())

	keys, err := kv.GetKVStore().GetKeys("maand/job/api/worker/10.0.0.1")
	require.NoError(t, err)
	assert.NotContains(t, keys, "certs/tls.crt")
	assert.NotContains(t, keys, "certs/tls.key")
}

func TestLoadJobCertSpecs(t *testing.T) {
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	seedBuildCertsDB(t, db, false)

	tx, err := db.Begin()
	require.NoError(t, err)
	specs, err := loadJobCertSpecs(tx, "api")
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, "tls", specs[0].name)
	assert.True(t, specs[0].pkcs8)
	require.NoError(t, tx.Rollback())
}

func TestWorkerCertNeedsRegeneration_missingKey(t *testing.T) {
	t.Cleanup(kv.ResetStoreForTest)
	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, tx.Commit())

	needs, err := workerCertNeedsRegeneration("api", "10.0.0.1", "tls", 30)
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestCertPEMExpiringSoon(t *testing.T) {
	expiring, err := certPEMExpiringSoon([]byte("not-a-pem"), 30)
	require.NoError(t, err)
	assert.True(t, expiring)

	setupBuildCertSecrets(t)
	certDir := t.TempDir()
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "test"}, []string{"10.0.0.1"}, 365, true))
	certPEM, err := os.ReadFile(path.Join(certDir, "tls.crt"))
	require.NoError(t, err)
	expiring, err = certPEMExpiringSoon(certPEM, 30)
	require.NoError(t, err)
	assert.False(t, expiring)
}

func TestBuildWorkerCertPEM_regenerates(t *testing.T) {
	setupBuildCertSecrets(t)

	spec := jobCertSpec{name: "tls", pkcs8: true, one: false, subject: pkix.Name{CommonName: "api"}}
	certPEM, keyPEM, err := buildWorkerCertPEM("api", "jobs/api", "10.0.0.1", spec, true, 365)
	require.NoError(t, err)
	assert.Contains(t, string(certPEM), "BEGIN CERTIFICATE")
	assert.Contains(t, string(keyPEM), "BEGIN PRIVATE KEY")

	certPath := workerCertDir("10.0.0.1", "jobs/api")
	assert.FileExists(t, path.Join(certPath, "tls.crt"))
	assert.FileExists(t, path.Join(certPath, "tls.key"))
}

func TestWorkerCertNeedsRegeneration_expiringCert(t *testing.T) {
	setupBuildCertSecrets(t)
	t.Cleanup(kv.ResetStoreForTest)

	certDir := t.TempDir()
	// Expired cert: NotAfter + renewalBufferDays (30) must be before now.
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "api"}, []string{"10.0.0.1"}, -60, true))
	certPEM, err := os.ReadFile(path.Join(certDir, "tls.crt"))
	require.NoError(t, err)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.crt", string(certPEM), 0)
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.key", "KEY", 0)
	require.NoError(t, tx.Commit())

	needs, err := workerCertNeedsRegeneration("api", "10.0.0.1", "tls", 30)
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestIsCertExpiringSoon(t *testing.T) {
	setupBuildCertSecrets(t)
	certDir := t.TempDir()
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "test"}, []string{"10.0.0.1"}, 365, true))

	expiring, err := IsCertExpiringSoon(path.Join(certDir, "tls.crt"), 30)
	require.NoError(t, err)
	assert.False(t, expiring)

	expiredDir := t.TempDir()
	require.NoError(t, GenerateCert(expiredDir, "tls", pkix.Name{CommonName: "test"}, []string{"10.0.0.1"}, -60, true))
	expiring, err = IsCertExpiringSoon(path.Join(expiredDir, "tls.crt"), 30)
	require.NoError(t, err)
	assert.True(t, expiring)
}

func TestCertPEMExpiringSoon_invalidCertificate(t *testing.T) {
	expiring, err := certPEMExpiringSoon([]byte("-----BEGIN CERTIFICATE-----\nZm9v\n-----END CERTIFICATE-----"), 30)
	require.Error(t, err)
	assert.True(t, expiring)
}

func TestCertNeedsRegeneration_expiredWorkerCert(t *testing.T) {
	setupBuildCertSecrets(t)
	t.Cleanup(kv.ResetStoreForTest)

	certDir := t.TempDir()
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "api"}, []string{"10.0.0.1"}, -60, true))
	certPEM, err := os.ReadFile(path.Join(certDir, "tls.crt"))
	require.NoError(t, err)
	keyPEM, err := os.ReadFile(path.Join(certDir, "tls.key"))
	require.NoError(t, err)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.crt", string(certPEM), 0)
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.key", string(keyPEM), 0)
	require.NoError(t, tx.Commit())

	needs, err := certNeedsRegeneration("api", "tls", false, []string{"10.0.0.1"}, 30)
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestBuildSharedCertPEM_regeneratesAllWorkers(t *testing.T) {
	setupBuildCertSecrets(t)

	spec := jobCertSpec{name: "tls", pkcs8: true, one: true, subject: pkix.Name{CommonName: "api"}}
	certPEM, keyPEM, err := buildSharedCertPEM("api", "jobs/api", spec, []string{"10.0.0.1", "10.0.0.2"}, true, 365)
	require.NoError(t, err)
	assert.Contains(t, string(certPEM), "BEGIN CERTIFICATE")
	assert.Contains(t, string(keyPEM), "BEGIN PRIVATE KEY")

	for _, workerIP := range []string{"10.0.0.1", "10.0.0.2"} {
		certPath := workerCertDir(workerIP, "jobs/api")
		assert.FileExists(t, path.Join(certPath, "tls.crt"))
		assert.FileExists(t, path.Join(certPath, "tls.key"))
	}
}

func TestBuildWorkerCertPEM_reusesStoredKV(t *testing.T) {
	setupBuildCertSecrets(t)
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.crt", "STORED-CERT", 0)
	kv.GetKVStore().Put("maand/job/api/worker/10.0.0.1", "certs/tls.key", "STORED-KEY", 0)
	require.NoError(t, tx.Commit())

	spec := jobCertSpec{name: "tls", pkcs8: true, one: false, subject: pkix.Name{CommonName: "api"}}
	certPEM, keyPEM, err := buildWorkerCertPEM("api", "jobs/api", "10.0.0.1", spec, false, 365)
	require.NoError(t, err)
	assert.Equal(t, "STORED-CERT", string(certPEM))
	assert.Equal(t, "STORED-KEY", string(keyPEM))
}

func TestWriteCertFiles(t *testing.T) {
	certDir := t.TempDir()
	certPEM := []byte("CERT-DATA")
	keyPEM := []byte("KEY-DATA")
	require.NoError(t, writeCertFiles(certDir, "tls", certPEM, keyPEM))

	gotCert, err := os.ReadFile(path.Join(certDir, "tls.crt"))
	require.NoError(t, err)
	assert.Equal(t, certPEM, gotCert)

	gotKey, err := os.ReadFile(path.Join(certDir, "tls.key"))
	require.NoError(t, err)
	assert.Equal(t, keyPEM, gotKey)

	info, err := os.Stat(path.Join(certDir, "tls.key"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestBuildCerts_generatesSharedCert(t *testing.T) {
	setupBuildCertSecrets(t)
	t.Cleanup(kv.ResetStoreForTest)

	db := openBuildAllocationsTestDB(t)
	defer func() { _ = db.Close() }()
	seedBuildCertsDB(t, db, true)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))
	require.NoError(t, BuildCerts(tx))
	require.NoError(t, tx.Commit())

	store := kv.GetKVStore()
	for _, workerIP := range []string{"10.0.0.1", "10.0.0.2"} {
		crt, err := store.Get("maand/job/api/worker/"+workerIP, "certs/tls.crt")
		require.NoError(t, err)
		assert.NotEmpty(t, crt.Value)
		key, err := store.Get("maand/job/api/worker/"+workerIP, "certs/tls.key")
		require.NoError(t, err)
		assert.NotEmpty(t, key.Value)
	}

	crt1, err := store.Get("maand/job/api/worker/10.0.0.1", "certs/tls.crt")
	require.NoError(t, err)
	crt2, err := store.Get("maand/job/api/worker/10.0.0.2", "certs/tls.crt")
	require.NoError(t, err)
	assert.Equal(t, crt1.Value, crt2.Value)
}

func TestGenerateCert_pkcs1(t *testing.T) {
	setupBuildCertSecrets(t)

	certDir := t.TempDir()
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "test"}, []string{"10.0.0.1"}, 30, false))

	keyPEM, err := os.ReadFile(path.Join(certDir, "tls.key"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	assert.Equal(t, "RSA PRIVATE KEY", block.Type)

	info, err := os.Stat(path.Join(certDir, "tls.key"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
