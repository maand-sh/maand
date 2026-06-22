// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"fmt"

	"maand/bucket"
)

// LatestSchemaVersion is the target schema version applied by MigrateSchema.
const LatestSchemaVersion = 4

// CheckSchemaVersion verifies maand.db exists and its schema version matches this binary.
// Run maand init to create or upgrade the database.
func CheckSchemaVersion() error {
	if !DatabaseExists() {
		return bucket.ErrNotInitialized
	}

	db, err := OpenDatabase(false)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	currentVersion, err := readSchemaVersion(tx)
	if err != nil {
		return err
	}

	if currentVersion < LatestSchemaVersion {
		return fmt.Errorf(
			"%w: database schema version %d, binary expects %d; run maand init to upgrade",
			bucket.ErrSchemaUpgradeRequired,
			currentVersion,
			LatestSchemaVersion,
		)
	}
	if currentVersion > LatestSchemaVersion {
		return fmt.Errorf(
			"%w: database schema version %d is newer than this binary supports (%d); upgrade the maand binary",
			bucket.ErrSchemaTooNew,
			currentVersion,
			LatestSchemaVersion,
		)
	}
	return nil
}

// MigrateSchema brings an existing or new database to LatestSchemaVersion.
// It is idempotent and safe to run on every maand init.
func MigrateSchema(tx *sql.Tx) error {
	if err := execStatements(tx, baseTableDDL()); err != nil {
		return err
	}

	currentVersion, err := readSchemaVersion(tx)
	if err != nil {
		return err
	}

	for version := currentVersion + 1; version <= LatestSchemaVersion; version++ {
		if err := applySchemaMigration(tx, version); err != nil {
			return fmt.Errorf("schema migration %d: %w", version, err)
		}
		if err := writeSchemaVersion(tx, version); err != nil {
			return err
		}
	}
	return nil
}

func readSchemaVersion(tx *sql.Tx) (int, error) {
	var tableCount int
	err := tx.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_version'`,
	).Scan(&tableCount)
	if err != nil {
		return 0, bucket.DatabaseError(err)
	}
	if tableCount == 0 {
		return 0, nil
	}

	var version int
	err = tx.QueryRow(`SELECT version FROM schema_version WHERE id = 1`).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, bucket.DatabaseError(err)
	}
	return version, nil
}

func writeSchemaVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec(
		`INSERT INTO schema_version (id, version) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET version = excluded.version`,
		version,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

func applySchemaMigration(tx *sql.Tx, version int) error {
	switch version {
	case 1:
		return migrateToV1(tx)
	case 2:
		return migrateToV2(tx)
	case 3:
		return migrateToV3(tx)
	case 4:
		return migrateToV4(tx)
	default:
		return fmt.Errorf("unsupported schema version %d", version)
	}
}

func migrateToV1(tx *sql.Tx) error {
	if err := execStatements(tx, []string{
		`CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		)`,
	}); err != nil {
		return err
	}
	if err := ensureTableColumn(tx, "allocations", "new_version", `ALTER TABLE allocations ADD COLUMN new_version TEXT`); err != nil {
		return err
	}
	if err := ensureTableColumn(tx, "hash", "current_version", `ALTER TABLE hash ADD COLUMN current_version TEXT`); err != nil {
		return err
	}
	if err := ensureTableColumn(tx, "job", "health_check", `ALTER TABLE job ADD COLUMN health_check TEXT`); err != nil {
		return err
	}
	return recreateCatalogViews(tx)
}

func migrateToV2(tx *sql.Tx) error {
	return ensureCatHashesView(tx)
}

// migrateToV3 renames the cat_hashes view to cat_deployments (backs `maand cat deployments`).
func migrateToV3(tx *sql.Tx) error {
	return ensureCatDeploymentsView(tx)
}

func migrateToV4(tx *sql.Tx) error {
	return ensureTableColumn(tx, "job", "deploy_parallel_count", `ALTER TABLE job ADD COLUMN deploy_parallel_count INT NOT NULL DEFAULT 0`)
}

func ensureCatHashesView(tx *sql.Tx) error {
	return execStatements(tx, []string{
		`DROP VIEW IF EXISTS cat_hashes`,
		`CREATE VIEW cat_hashes (
			alloc_id, worker_ip, job, disabled, removed,
			current_hash, previous_hash, current_version, new_version
		) AS
			SELECT a.alloc_id, a.worker_ip, a.job, a.disabled, a.removed,
			       ifnull(h.current_hash, ''), ifnull(h.previous_hash, ''),
			       ifnull(h.current_version, ''), ifnull(a.new_version, '')
			FROM allocations a
			LEFT JOIN hash h ON h.namespace = (a.job || '_allocation') AND h.key = a.alloc_id
			ORDER BY a.job, a.worker_ip`,
	})
}

func ensureCatDeploymentsView(tx *sql.Tx) error {
	return execStatements(tx, []string{
		`DROP VIEW IF EXISTS cat_hashes`,
		`DROP VIEW IF EXISTS cat_deployments`,
		`CREATE VIEW cat_deployments (
			alloc_id, worker_ip, job, disabled, removed,
			current_hash, previous_hash, current_version, new_version
		) AS
			SELECT a.alloc_id, a.worker_ip, a.job, a.disabled, a.removed,
			       ifnull(h.current_hash, ''), ifnull(h.previous_hash, ''),
			       ifnull(h.current_version, ''), ifnull(a.new_version, '')
			FROM allocations a
			LEFT JOIN hash h ON h.namespace = (a.job || '_allocation') AND h.key = a.alloc_id
			ORDER BY a.job, a.worker_ip`,
	})
}

func ensureTableColumn(tx *sql.Tx, table, column, alterDDL string) error {
	var columnCount int
	err := tx.QueryRow(
		`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`,
		table, column,
	).Scan(&columnCount)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	if columnCount > 0 {
		return nil
	}
	_, err = tx.Exec(alterDDL)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

func baseTableDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS bucket (bucket_id TEXT, update_seq INT)`,
		`CREATE TABLE IF NOT EXISTS worker (
			worker_id TEXT,
			worker_ip TEXT,
			available_memory_mb TEXT,
			available_cpu_mhz TEXT,
			position INT,
			PRIMARY KEY(worker_ip)
		)`,
		`CREATE TABLE IF NOT EXISTS worker_labels (worker_id TEXT, label TEXT)`,
		`CREATE TABLE IF NOT EXISTS worker_tags (worker_id TEXT, key TEXT, value TEXT)`,
		`CREATE TABLE IF NOT EXISTS allocations (
			alloc_id TEXT,
			worker_ip TEXT,
			job TEXT,
			disabled INT,
			removed INT,
			deployment_seq INT,
			new_version TEXT,
			PRIMARY KEY(worker_ip, job)
		)`,
		`CREATE TABLE IF NOT EXISTS job (
			job_id TEXT,
			name TEXT,
			version TEXT,
			min_memory_mb TEXT,
			max_memory_mb TEXT,
			current_memory_mb TEXT,
			min_cpu_mhz TEXT,
			max_cpu_mhz TEXT,
			current_cpu_mhz TEXT,
			update_parallel_count INT,
			deploy_parallel_count INT NOT NULL DEFAULT 0,
			health_check TEXT,
			PRIMARY KEY(name)
		)`,
		`CREATE TABLE IF NOT EXISTS job_selectors (job_id TEXT, selector TEXT)`,
		`CREATE TABLE IF NOT EXISTS job_ports (job_id TEXT, name TEXT, port INT)`,
		`CREATE TABLE IF NOT EXISTS job_certs (job_id TEXT, name TEXT, pkcs8 INT, one INT, subject TEXT)`,
		`CREATE TABLE IF NOT EXISTS job_files (job_id TEXT, path TEXT, content BLOB, isdir BOOL)`,
		`CREATE TABLE IF NOT EXISTS job_commands (
			job_id TEXT,
			job TEXT,
			name TEXT,
			executed_on TEXT,
			demand_job TEXT,
			demand_command TEXT,
			demand_config TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS key_value (
			key TEXT,
			value TEXT,
			namespace TEXT,
			version INT,
			ttl TEXT,
			created_date TEXT,
			deleted INT
		)`,
		`CREATE TABLE IF NOT EXISTS hash (
			namespace TEXT,
			key TEXT,
			current_hash TEXT,
			previous_hash TEXT,
			current_version TEXT,
			PRIMARY KEY(namespace, key)
		)`,
	}
}

func recreateCatalogViews(tx *sql.Tx) error {
	views := []string{
		`DROP VIEW IF EXISTS cat_allocations`,
		`DROP VIEW IF EXISTS cat_jobs`,
		`DROP VIEW IF EXISTS cat_job_commands`,
		`DROP VIEW IF EXISTS cat_kv`,
		`DROP VIEW IF EXISTS cat_workers`,
		`CREATE VIEW cat_allocations (alloc_id, worker_ip, job, disabled, removed, new_version) AS
			SELECT alloc_id, worker_ip, job, disabled, removed, new_version FROM allocations ORDER BY job`,
		`CREATE VIEW cat_jobs (job_id, name, version, disabled, deployment_seq, selectors) AS
			SELECT DISTINCT job_id, name, version,
				(CASE WHEN (SELECT COUNT(1) FROM allocations wj WHERE j.name = wj.job AND wj.disabled = 0) > 0 THEN 0 ELSE 1 END) AS disabled,
				ifnull((SELECT DISTINCT deployment_seq FROM allocations wj WHERE wj.job = j.name), 0) AS deployment_seq,
				ifnull((SELECT GROUP_CONCAT(selector) FROM job_selectors jl WHERE jl.job_id = j.job_id), '') as selectors
			FROM job j ORDER BY deployment_seq, name`,
		`CREATE VIEW cat_job_commands (job, command_name, executed_on, demand_job, demand_command, demand_config) AS
			SELECT job, name as command_name, executed_on, demand_job, demand_command, demand_config FROM job_commands ORDER BY job, name`,
		`CREATE VIEW cat_kv (namespace, key, value, version, ttl, created_date, deleted) AS
			SELECT * FROM (
				SELECT namespace, key,
					(CASE
						WHEN value LIKE 'enc:v1:%' THEN '[encrypted]'
						WHEN LENGTH(value) > 50 THEN substr(value, 1, 50) || '...'
						ELSE value
					END) as value,
					max(version) as version, ttl, created_date, deleted
				FROM key_value GROUP BY namespace, key
			) t ORDER BY namespace, key`,
		`CREATE VIEW cat_workers (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position, labels) AS
			SELECT worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position,
				(SELECT group_concat(label) AS labels FROM worker_labels WHERE worker_id = w.worker_id) AS labels
			FROM worker w ORDER BY position`,
	}
	return execStatements(tx, views)
}

func execStatements(tx *sql.Tx, statements []string) error {
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return bucket.DatabaseError(err)
		}
	}
	return nil
}
