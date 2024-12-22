import os


def setup_job_database(cursor):
    cursor.execute("CREATE TABLE IF NOT EXISTS job_db.job (job_id TEXT PRIMARY KEY, name TEXT, version TEXT, min_memory_mb TEXT, max_memory_mb TEXT, min_cpu TEXT, max_cpu TEXT, certs_md5_hash TEXT, deployment_seq INT)")
    cursor.execute("CREATE TABLE IF NOT EXISTS job_db.job_labels (job_id TEXT, label TEXT)")
    cursor.execute("CREATE TABLE IF NOT EXISTS job_db.job_ports (job_id TEXT, name TEXT, port INT)")
    cursor.execute("CREATE TABLE IF NOT EXISTS job_db.job_certs (job_id TEXT, name TEXT, pkcs8 INT, subject TEXT)")
    cursor.execute("CREATE TABLE IF NOT EXISTS job_db.job_files (job_id TEXT, path TEXT, content BLOB, isdir BOOL)")
    cursor.execute("CREATE TABLE IF NOT EXISTS job_db.job_commands (job_id TEXT, job_name TEXT, name TEXT, executed_on TEXT, depend_on_job TEXT, depend_on_command TEXT, depend_on_config TEXT)")


def get_jobs(cursor, deployment_seq = -1):
    if deployment_seq == -1:
        cursor.execute("SELECT name FROM job_db.job")
    else:
        cursor.execute("SELECT name FROM job_db.job WHERE deployment_seq = ?", (deployment_seq,))
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def has_custom_job_control(cursor, job):
    cursor.execute("SELECT custom_job_control FROM job_db.job where name = ?", (job,))
    row = cursor.fetchone()
    return row[0] == 1


def get_max_deployment_seq(cursor):
    cursor.execute("SELECT max(deployment_seq) FROM job_db.job")
    row = cursor.fetchone()
    return row[0]


def get_job_certs_config(cursor, job):
    cursor.execute("SELECT jc.name, jc.pkcs8, jc.subject FROM job_db.job_certs jc JOIN job j ON j.job_id = jc.job_id WHERE j.name = ?", (job,))
    rows = cursor.fetchall()
    return [{"name": row[0], "pkcs8": row[1], "subject": row[2]}  for row in rows]


def get_job_md5_hash(cursor, job):
    cursor.execute("SELECT certs_md5_hash FROM job_db.job WHERE name = ?", (job,))
    row = cursor.fetchone()
    return row[0]


def get_job_commands(cursor, job, event):
    cursor.execute("SELECT DISTINCT name as command_name FROM job_db.job_commands WHERE job_name = ? AND executed_on = ?", (job, event, ))
    rows = cursor.fetchall()
    commands = []
    for command_name, in rows:
        commands.append(command_name)
    return commands


def get_job_resource_limits(cursor, job):
    cursor.execute("SELECT min_memory_mb, max_memory_mb, min_cpu, max_cpu FROM job_db.job WHERE name = ?", (job,))
    min_memory_mb, max_memory_mb, min_cpu, max_cpu = cursor.fetchone()
    min_memory_mb, max_memory_mb, min_cpu, max_cpu = (float(min_memory_mb), float(max_memory_mb), float(min_cpu), float(max_cpu),)
    return (min_memory_mb, max_memory_mb, min_cpu, max_cpu, )


def copy_job(cursor, name, agent_dir):
    cursor.execute("SELECT path, content, isdir FROM job_db.job_files WHERE job_id = (SELECT job_id FROM job_db.job WHERE name = ?) AND path NOT LIKE ? ORDER BY isdir DESC", (name, f"{name}/_modules%"))
    rows = cursor.fetchall()

    for path, content, isdir in rows:
        if isdir:
            os.makedirs(f"{agent_dir}/jobs/{path}", exist_ok=True)
            continue
        with open(f"{agent_dir}/jobs/{path}", "wb") as f:
            f.write(content)


def copy_job_modules(cursor, job):
    cursor.execute("SELECT path, content, isdir FROM job_db.job_files WHERE job_id = (SELECT job_id FROM job_db.job WHERE name = ?) AND path like ? ORDER BY isdir DESC", (job, f"{job}/_modules%"))
    rows = cursor.fetchall()

    for path, content, isdir in rows:
        if isdir:
            os.makedirs(f"/modules/{path}", exist_ok=True)
            continue
        with open(f"/modules/{path}", "wb") as f:
            f.write(content)
