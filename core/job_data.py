import os


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


def copy_job(cursor, name, agent_dir):
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


def copy_job_modules(cursor, job):
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
