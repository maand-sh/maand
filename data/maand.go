package data

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"maand/bucket"
	"maand/utils"
	"os"
	"path"
)

func GetDatabase(failIfNotFound bool) (*sql.DB, error) {
	var DbFile = path.Join(bucket.Location, "data/maand.db")
	if failIfNotFound {
		if _, err := os.Stat(DbFile); os.IsNotExist(err) {
			return nil, errors.New("maand is not initialized in this dictionary")
		}
	}
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_busy_timeout=5000&_locking_mode=NORMAL&_txlock=deferred", DbFile))
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("PRAGMA journal_mode = WAL;")
	utils.Check(err)
	return db, nil
}

func SetupMaand(tx *sql.Tx) error {
	tables := []string{
		"CREATE TABLE IF NOT EXISTS bucket (bucket_id TEXT, update_seq INT)",
		"CREATE TABLE IF NOT EXISTS worker (worker_id TEXT, worker_ip TEXT, available_memory_mb TEXT, available_cpu_mhz TEXT, position INT, PRIMARY KEY(worker_ip))",
		"CREATE TABLE IF NOT EXISTS worker_labels (worker_id TEXT, label TEXT)",
		"CREATE TABLE IF NOT EXISTS worker_tags (worker_id TEXT, key TEXT, value TEXT)",
		"CREATE TABLE IF NOT EXISTS allocations (alloc_id TEXT, worker_ip TEXT, job TEXT, disabled INT, removed INT, deployment_seq INT, PRIMARY KEY(worker_ip, job))",

		"CREATE TABLE IF NOT EXISTS job (job_id TEXT, name TEXT, version TEXT, min_memory_mb TEXT, max_memory_mb TEXT, min_cpu_mhz TEXT, max_cpu_mhz TEXT, update_parallel_count int, PRIMARY KEY(name))",
		"CREATE TABLE IF NOT EXISTS job_selectors (job_id TEXT, selector TEXT)",
		"CREATE TABLE IF NOT EXISTS job_ports (job_id TEXT, name TEXT, port INT)",
		"CREATE TABLE IF NOT EXISTS job_certs (job_id TEXT, name TEXT, pkcs8 INT, subject TEXT)",
		"CREATE TABLE IF NOT EXISTS job_files (job_id TEXT, path TEXT, content BLOB, isdir BOOL)",
		"CREATE TABLE IF NOT EXISTS job_commands (job_id TEXT, job TEXT, name TEXT, executed_on TEXT, depend_on_job TEXT, depend_on_command TEXT, depend_on_config TEXT)",

		"CREATE TABLE IF NOT EXISTS key_value (key TEXT, value TEXT, namespace TEXT, version INT, ttl TEXT, created_date TEXT, deleted INT)",
		"CREATE TABLE IF NOT EXISTS hash (namespace TEXT, key TEXT, current_hash TEXT, previous_hash TEXT, PRIMARY KEY(namespace, key))",

		`CREATE VIEW IF NOT EXISTS cat_allocations (alloc_id, worker_ip, job, disabled, removed) AS 
			SELECT alloc_id, worker_ip, job, disabled, removed FROM allocations ORDER BY job`,
		`CREATE VIEW IF NOT EXISTS cat_jobs (job_id, name, version, disabled, deployment_seq, selectors) AS 
			SELECT DISTINCT job_id, name, version, 
			                (CASE WHEN (SELECT COUNT(1) FROM allocations wj WHERE j.name = wj.job AND wj.disabled = 0) > 0 THEN 0 ELSE 1 END) AS disabled,
			                ifnull((SELECT DISTINCT deployment_seq FROM allocations wj WHERE wj.job = j.name), 0) AS deployment_seq, 
			                ifnull((SELECT GROUP_CONCAT(selector) FROM job_selectors jl WHERE jl.job_id = j.job_id), '') as selectors 
			FROM job j ORDER BY deployment_seq, name`,
		`CREATE VIEW IF NOT EXISTS cat_job_commands (job, command_name, executed_on, depend_on_job, depend_on_command, depend_on_config) AS 
			SELECT job, name as command_name, executed_on, depend_on_job, depend_on_command, depend_on_config  FROM job_commands ORDER BY job, name`,
		`CREATE VIEW IF NOT EXISTS cat_kv (namespace, key, value, version, ttl, created_date, deleted) AS 
			SELECT * FROM (
				SELECT namespace, key, (CASE WHEN LENGTH(value) > 50 THEN substr(value, 1, 50) || '...' ELSE value END) as value, 
				       max(version) as version, ttl, created_date, deleted 
				FROM key_value GROUP BY namespace, key
			) t ORDER BY namespace, key`,
		`CREATE VIEW IF NOT EXISTS cat_workers (worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position, labels) AS 
			SELECT worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position, 
			       (SELECT group_concat(label) AS labels FROM worker_labels WHERE worker_id = w.worker_id) AS labels 
			FROM worker w ORDER BY position`,
	}

	for _, query := range tables {
		if _, err := tx.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func GetBucketID(tx *sql.Tx) string {
	var bucketID string
	err := tx.QueryRow("SELECT bucket_id FROM bucket LIMIT 1").Scan(&bucketID)
	utils.Check(err)
	return bucketID
}

func GetUpdateSeq(tx *sql.Tx) int {
	var updateSeq int
	err := tx.QueryRow("SELECT update_seq FROM bucket").Scan(&updateSeq)
	utils.Check(err)
	return updateSeq
}

func UpdateSeq(tx *sql.Tx, updateSeq int) {
	_, err := tx.Exec("UPDATE bucket SET update_seq = ?", updateSeq)
	utils.Check(err)
}

func GetMaxDeploymentSeq(tx *sql.Tx) int {
	var updateSeq int
	err := tx.QueryRow("SELECT ifnull(max(deployment_seq), 0) AS max_deployment_seq FROM allocations").Scan(&updateSeq)
	utils.Check(err)
	return updateSeq
}
