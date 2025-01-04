import os
import sqlite3
import uuid

import const


def get_db(fail_if_not_found=True):
    if fail_if_not_found and (not os.path.exists(const.MAAND_DB_PATH) or not os.path.exists(const.JOBS_DB_PATH) or not os.path.exists(const.KV_DB_PATH)):
        raise Exception("maand is not initialized")
    db = sqlite3.connect(const.MAAND_DB_PATH)
    db.execute(f"ATTACH DATABASE '{const.JOBS_DB_PATH}' AS job_db;")
    db.execute(f"ATTACH DATABASE '{const.KV_DB_PATH}' AS kv_db;")
    return db


def setup_maand_database(cursor):
    cursor.execute("CREATE TABLE IF NOT EXISTS bucket (bucket_id TEXT, update_seq INT, ca_md5_hash TEXT)")
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS agent (agent_id TEXT, agent_ip TEXT, agent_memory_mb TEXT, agent_cpu TEXT, detained INT, position INT)")
    cursor.execute("CREATE TABLE IF NOT EXISTS agent_labels (agent_id TEXT, label TEXT)")
    cursor.execute("CREATE TABLE IF NOT EXISTS agent_tags (agent_id TEXT, key TEXT, value INT)")
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS agent_jobs (agent_id TEXT, job TEXT, disabled INT, removed INT, current_md5_hash TEXT, previous_md5_hash TEXT)")
    cursor.execute("SELECT bucket_id FROM bucket")
    if cursor.fetchone() is None:
        cursor.execute("INSERT INTO bucket (bucket_id, update_seq) VALUES (?, ?)", (str(uuid.uuid4()), 0))
    else:
        raise Exception("bucket is already initialized")


def get_bucket_id(cursor):
    cursor.execute("SELECT bucket_id FROM bucket")
    row = cursor.fetchone()
    return row[0]


def get_ca_md5_hash(cursor):
    cursor.execute("SELECT ca_md5_hash FROM bucket")
    row = cursor.fetchone()
    return row[0]


def get_update_seq(cursor):
    cursor.execute("SELECT update_seq FROM bucket")
    row = cursor.fetchone()
    return row[0]


def update_update_seq(cursor, seq):
    cursor.execute("UPDATE bucket SET update_seq = ?", (seq,))


def get_agent_jobs(cursor, agent_ip):
    cursor.execute(
        "SELECT aj.job, aj.disabled FROM agent a JOIN agent_jobs aj ON a.agent_id = aj.agent_id JOIN job_db.job j ON j.name = aj.job AND aj.removed = 0 AND a.agent_ip = ?",
        (agent_ip,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]

def get_agent_jobs_and_status(cursor, agent_ip):
    cursor.execute(
        "SELECT aj.job, aj.disabled FROM agent a JOIN agent_jobs aj ON a.agent_id = aj.agent_id JOIN job_db.job j ON j.name = aj.job AND aj.removed = 0 AND a.agent_ip = ?",
        (agent_ip,))
    rows = cursor.fetchall()
    return {row[0]: {"disabled": row[1]} for row in rows}


def get_agent_all_jobs(cursor, agent_ip):
    cursor.execute(
        "SELECT aj.job, aj.disabled FROM agent a JOIN agent_jobs aj ON a.agent_id = aj.agent_id JOIN job_db.job j ON j.name = aj.job AND a.agent_ip = ?",
        (agent_ip,))
    rows = cursor.fetchall()
    return {row[0]: {"disabled": row[1]} for row in rows}


def get_agent_removed_jobs(cursor, agent_ip):
    cursor.execute(
        "select aj.job FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.removed = 1 AND agent_ip = ?",
        (agent_ip,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agent_disabled_jobs(cursor, agent_ip):
    cursor.execute(
        "select aj.job FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.disabled = 1 AND agent_ip = ?",
        (agent_ip,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agents(cursor, labels_filter):
    if not labels_filter:
        labels_filter = ["agent"]
    labels_filter = [f"'{label}'" for label in labels_filter]
    labels_filter = ",".join(labels_filter)

    cursor.execute(
        f"SELECT DISTINCT agent_ip FROM agent a JOIN agent_labels ar ON a.agent_id = ar.agent_id WHERE a.detained = 0 AND ar.label IN ({labels_filter}) ORDER BY position;")
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent a JOIN agent_jobs aj ON a.agent_id = aj.agent_id INNER JOIN job_db.job j ON j.name = aj.job WHERE aj.job = ? ORDER BY a.agent_ip",
        (job,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agent_labels(cursor, agent_ip):
    if agent_ip:
        cursor.execute(
            "SELECT DISTINCT label FROM agent a JOIN agent_labels ar ON a.agent_id = ar.agent_id AND agent_ip = ?;",
            (agent_ip,))
    else:
        cursor.execute("SELECT DISTINCT label FROM agent a JOIN agent_labels ar ON a.agent_id = ar.agent_id;", )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agent_tags(cursor, agent_ip):
    cursor.execute("SELECT key, value FROM agent a JOIN agent_tags at ON a.agent_id = at.agent_id WHERE a.agent_ip = ?",
                   (agent_ip,))
    rows = cursor.fetchall()
    return {row[0]: row[1] for row in rows}


def get_agent_id(cursor, agent_ip):
    cursor.execute("SELECT agent_id FROM agent WHERE agent_ip = ?", (agent_ip,))
    row = cursor.fetchone()
    return row[0]


def get_agent_available_resources(cursor, agent_ip):
    cursor.execute("SELECT agent_memory_mb, agent_cpu FROM agent WHERE agent_ip = ?", (agent_ip,))
    row = cursor.fetchone()
    return row


def get_allocation_counts(cursor, job):
    cursor.execute("""
        SELECT
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.previous_md5_hash IS NULL AND aj.removed <> 1 AND aj.job = ?) AS new_allocations_count,
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.previous_md5_hash = aj.current_md5_hash AND aj.job = ?) AS unchanged_allocations_count,
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.previous_md5_hash IS NOT NULL AND aj.previous_md5_hash != aj.current_md5_hash AND aj.job = ?) AS changed_allocations_count,
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.removed = 1 AND aj.job = ?) AS removed_allocations_count,
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.job = ?) AS total_allocations_count
    """, (job, job, job, job, job))
    new_count, unchanged_count, changed_count, removed_count, total_count = cursor.fetchone()
    counts = {}
    counts["new"] = new_count
    counts["unchanged"] = unchanged_count
    counts["changed"] = changed_count
    counts["removed"] = removed_count
    counts["total"] = total_count
    return counts


def get_new_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.previous_md5_hash IS NULL AND aj.job = ?",
        (job,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_changed_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.previous_md5_hash IS NOT NULL AND aj.previous_md5_hash != aj.current_md5_hash AND aj.job = ?",
        (job,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_removed_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.removed = 1 AND aj.job = ?",
        (job,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_disabled_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.disabled = 1 AND aj.job = ?",
        (job,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]
