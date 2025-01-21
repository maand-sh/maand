import os
import sqlite3
import uuid

import kv_manager
from core import const


def get_db(fail_if_not_found=True):
    if fail_if_not_found and not os.path.exists(const.MAAND_DB_PATH):
        raise Exception("maand is not initialized")
    return sqlite3.connect(const.MAAND_DB_PATH)


def setup_maand_database(cursor):
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS bucket (bucket_id TEXT, update_seq INT, ca_md5_hash TEXT)"
    )
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS agent (agent_id TEXT, agent_ip TEXT, agent_memory_mb TEXT, agent_cpu TEXT, detained INT, position INT)"
    )
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS agent_labels (agent_id TEXT, label TEXT)"
    )
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS agent_tags (agent_id TEXT, key TEXT, value TEXT)"
    )
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS agent_jobs (agent_id TEXT, job TEXT, disabled INT, removed INT, current_md5_hash TEXT, previous_md5_hash TEXT)"
    )

    cursor.execute(
        "CREATE TABLE IF NOT EXISTS job (job_id TEXT PRIMARY KEY, name TEXT, version TEXT, min_memory_mb TEXT, max_memory_mb TEXT, min_cpu TEXT, max_cpu TEXT, certs_md5_hash TEXT, deployment_seq INT)"
    )
    cursor.execute("CREATE TABLE IF NOT EXISTS job_labels (job_id TEXT, label TEXT)")
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS job_ports (job_id TEXT, name TEXT, port INT)"
    )
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS job_certs (job_id TEXT, name TEXT, pkcs8 INT, subject TEXT)"
    )
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS job_files (job_id TEXT, path TEXT, content BLOB, isdir BOOL)"
    )
    cursor.execute(
        "CREATE TABLE IF NOT EXISTS job_commands (job_id TEXT, job_name TEXT, name TEXT, executed_on TEXT, depend_on_job TEXT, depend_on_command TEXT, depend_on_config TEXT)"
    )

    cursor.execute(
        "CREATE TABLE IF NOT EXISTS key_value (key TEXT, value TEXT, namespace TEXT, version INT, ttl TEXT, created_date TEXT, deleted INT)"
    )

    cursor.execute("SELECT bucket_id FROM bucket")
    if cursor.fetchone() is None:
        cursor.execute(
            "INSERT INTO bucket (bucket_id, update_seq) VALUES (?, ?)",
            (str(uuid.uuid4()), 0),
        )
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
        "SELECT aj.job, aj.disabled FROM agent a JOIN agent_jobs aj ON a.agent_id = aj.agent_id JOIN job j ON j.name = aj.job AND aj.removed = 0 AND a.agent_ip = ?",
        (agent_ip,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agent_jobs_and_status(cursor, agent_ip):
    cursor.execute(
        "SELECT aj.job, aj.disabled FROM agent a JOIN agent_jobs aj ON a.agent_id = aj.agent_id JOIN job j ON j.name = aj.job AND aj.removed = 0 AND a.agent_ip = ?",
        (agent_ip,),
    )
    rows = cursor.fetchall()
    return {row[0]: {"disabled": row[1]} for row in rows}


def get_agent_removed_jobs(cursor, agent_ip):
    cursor.execute(
        "select aj.job FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.removed = 1 AND agent_ip = ?",
        (agent_ip,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agent_disabled_jobs(cursor, agent_ip):
    cursor.execute(
        "select aj.job FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.disabled = 1 AND agent_ip = ?",
        (agent_ip,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agents(cursor, labels_filter):
    if not labels_filter:
        labels_filter = ["agent"]
    labels_filter = [f"'{label}'" for label in labels_filter]
    labels_filter = ",".join(labels_filter)

    cursor.execute(
        f"SELECT DISTINCT agent_ip FROM agent a JOIN agent_labels ar ON a.agent_id = ar.agent_id WHERE a.detained = 0 AND ar.label IN ({labels_filter}) ORDER BY position;"
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent a JOIN agent_jobs aj ON a.agent_id = aj.agent_id INNER JOIN job j ON j.name = aj.job WHERE aj.job = ? ORDER BY a.agent_ip",
        (job,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agent_labels(cursor, agent_ip):
    if agent_ip:
        cursor.execute(
            "SELECT DISTINCT label FROM agent a JOIN agent_labels ar ON a.agent_id = ar.agent_id AND agent_ip = ?;",
            (agent_ip,),
        )
    else:
        cursor.execute(
            "SELECT DISTINCT label FROM agent a JOIN agent_labels ar ON a.agent_id = ar.agent_id;",
        )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_agent_tags(cursor, agent_ip):
    cursor.execute(
        "SELECT key, value FROM agent a JOIN agent_tags at ON a.agent_id = at.agent_id WHERE a.agent_ip = ?",
        (agent_ip,),
    )
    rows = cursor.fetchall()
    return {row[0]: row[1] for row in rows}


def get_agent_id(cursor, agent_ip):
    cursor.execute("SELECT agent_id FROM agent WHERE agent_ip = ?", (agent_ip,))
    row = cursor.fetchone()
    return row[0]


def get_agent_available_resources(cursor, agent_ip):
    cursor.execute(
        "SELECT agent_memory_mb, agent_cpu FROM agent WHERE agent_ip = ?", (agent_ip,)
    )
    row = cursor.fetchone()
    return row


def get_allocation_counts(cursor, job):
    cursor.execute(
        """
        SELECT
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.previous_md5_hash IS NULL AND aj.removed <> 1 AND aj.job = ?) AS new_allocations_count,
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.previous_md5_hash = aj.current_md5_hash AND aj.job = ?) AS unchanged_allocations_count,
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.previous_md5_hash IS NOT NULL AND aj.previous_md5_hash != aj.current_md5_hash AND aj.job = ?) AS changed_allocations_count,
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.removed = 1 AND aj.job = ?) AS removed_allocations_count,
            (SELECT COUNT(*) FROM agent_jobs aj WHERE aj.job = ?) AS total_allocations_count
    """,
        (job, job, job, job, job),
    )
    new_count, unchanged_count, changed_count, removed_count, total_count = (
        cursor.fetchone()
    )
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
        (job,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_changed_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.previous_md5_hash IS NOT NULL AND aj.previous_md5_hash != aj.current_md5_hash AND aj.job = ?",
        (job,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_removed_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.removed = 1 AND aj.job = ?",
        (job,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_disabled_allocations(cursor, job):
    cursor.execute(
        "SELECT a.agent_ip FROM agent_jobs aj JOIN agent a ON a.agent_id = aj.agent_id WHERE aj.disabled = 1 AND aj.job = ?",
        (job,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_jobs(cursor, deployment_seq=-1):
    if deployment_seq == -1:
        cursor.execute("SELECT name FROM job")
    else:
        cursor.execute(
            "SELECT name FROM job WHERE deployment_seq = ?", (deployment_seq,)
        )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def get_max_deployment_seq(cursor):
    cursor.execute(
        "SELECT ifnull(max(deployment_seq), 0) AS max_deployment_seq FROM job"
    )
    row = cursor.fetchone()
    return row[0]


def get_job_certs_config(cursor, job):
    cursor.execute(
        "SELECT jc.name, jc.pkcs8, jc.subject FROM job_certs jc JOIN job j ON j.job_id = jc.job_id WHERE j.name = ?",
        (job,),
    )
    rows = cursor.fetchall()
    return [{"name": row[0], "pkcs8": row[1], "subject": row[2]} for row in rows]


def get_job_md5_hash(cursor, job):
    cursor.execute("SELECT certs_md5_hash FROM job WHERE name = ?", (job,))
    row = cursor.fetchone()
    return row[0]


def get_job_commands(cursor, job, event):
    cursor.execute(
        "SELECT DISTINCT name as command_name FROM job_commands WHERE job_name = ? AND executed_on = ?",
        (
            job,
            event,
        ),
    )
    rows = cursor.fetchall()
    commands = []
    for (command_name,) in rows:
        commands.append(command_name)
    return commands


def get_job_resource_limits(cursor, job):
    cursor.execute(
        "SELECT min_memory_mb, max_memory_mb, min_cpu, max_cpu FROM job WHERE name = ?",
        (job,),
    )
    min_memory_mb, max_memory_mb, min_cpu, max_cpu = cursor.fetchone()
    min_memory_mb, max_memory_mb, min_cpu, max_cpu = (
        float(min_memory_mb),
        float(max_memory_mb),
        float(min_cpu),
        float(max_cpu),
    )
    return (
        min_memory_mb,
        max_memory_mb,
        min_cpu,
        max_cpu,
    )


def copy_job_files(cursor, name, allocation_ip, agent_dir):
    cursor.execute(
        "SELECT path, content, isdir FROM job_files WHERE job_id = (SELECT job_id FROM job WHERE name = ?) AND path NOT LIKE ? ORDER BY isdir DESC",
        (name, f"{name}/_modules%"),
    )
    rows = cursor.fetchall()

    for path, content, isdir in rows:
        if isdir:
            os.makedirs(f"{agent_dir}/jobs/{path}", exist_ok=True)
            continue
        with open(f"{agent_dir}/jobs/{path}", "wb") as f:
            f.write(content)

    namespace = f"maand/agent/{allocation_ip}/job/{name}"
    certs = kv_manager.get_keys(cursor, namespace)
    if certs:
        os.makedirs(f"{agent_dir}/jobs/{name}/certs", exist_ok=True)
    for cert in certs:
        with open(f"{agent_dir}/jobs/{name}/{cert}", "wb") as f:
            content = kv_manager.get(cursor, namespace, cert)
            f.write(str.encode(content))



def setup_job_modules(cursor, job):
    cursor.execute(
        "SELECT path, content, isdir FROM job_files WHERE job_id = (SELECT job_id FROM job WHERE name = ?) AND path like ? ORDER BY isdir DESC",
        (job, f"{job}/_modules%"),
    )
    rows = cursor.fetchall()

    os.makedirs(f"/modules/{job}/_modules", exist_ok=True)

    for path, content, isdir in rows:
        if isdir:
            os.makedirs(f"/modules/{path}", exist_ok=True)
            continue
        with open(f"/modules/{path}", "wb") as f:
            f.write(content)
