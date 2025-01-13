import jsonschema
from jsonschema import Draft202012Validator

import kv_manager
from core import job_data, maand_data, utils, workspace

logger = utils.get_logger()


def build_allocated_jobs(cursor):
    disabled = workspace.get_disabled_jobs()

    schema = {
        "type": "object",
        "properties": {
            "jobs": {
                "type": "object",
                "patternProperties": {
                    ".*": {
                        "type": "object",
                        "properties": {
                            "agents": {
                                "type": "array",
                                "items": {"type": "string", "format": "ipv4"},
                            }
                        },
                    }
                },
            },
            "agents": {"type": "array", "items": {"type": "string", "format": "ipv4"}},
        },
    }

    jsonschema.validate(
        instance=disabled,
        schema=schema,
        format_checker=Draft202012Validator.FORMAT_CHECKER,
    )

    disabled_jobs = disabled.get("jobs", {})
    disabled_agents = disabled.get("agents", [])
    cursor.execute("SELECT agent_id, agent_ip FROM agent")
    agents = cursor.fetchall()

    for agent_id, agent_ip in agents:
        cursor.execute(
            """
            SELECT DISTINCT j.name
            FROM job j
            JOIN job_labels jl ON jl.job_id = j.job_id
            JOIN agent_labels al ON al.label = jl.label
            WHERE (
                SELECT COUNT(DISTINCT jl_sub.label)
                FROM job_labels jl_sub
                WHERE jl_sub.job_id = j.job_id
            ) = (
                SELECT COUNT(DISTINCT al_sub.label)
                FROM agent_labels al_sub
                JOIN agent a ON al_sub.agent_id = a.agent_id
                WHERE al_sub.label IN (
                    SELECT jl_sub.label
                    FROM job_labels jl_sub
                    WHERE jl_sub.job_id = j.job_id
                ) AND a.agent_ip = ?
            );""",
            (agent_ip,),
        )

        assigned_jobs = [row[0] for row in cursor.fetchall()]

        for job in assigned_jobs:
            disabled = agent_ip in disabled_agents
            if not disabled:
                job_disabled_agents = disabled_jobs.get(job, {}).get("agents", [])
                disabled = agent_ip in job_disabled_agents
                if len(job_disabled_agents) == 0:
                    disabled = job in disabled_jobs

            cursor.execute(
                "SELECT * FROM agent_jobs WHERE job = ? AND agent_id = ?",
                (
                    job,
                    agent_id,
                ),
            )
            row = cursor.fetchone()
            if row:
                cursor.execute(
                    "UPDATE agent_jobs SET disabled = ?, removed = 0 WHERE job = ? AND agent_id = ?",
                    (
                        disabled,
                        job,
                        agent_id,
                    ),
                )
            else:
                cursor.execute(
                    "INSERT INTO agent_jobs (job, agent_id, disabled, removed) VALUES (?, ?, ?, 0)",
                    (job, agent_id, disabled),
                )

        cursor.execute("SELECT job FROM agent_jobs WHERE agent_id = ?", (agent_id,))
        all_assigned_jobs = [row[0] for row in cursor.fetchall()]
        removed_jobs = list(set(all_assigned_jobs) ^ set(assigned_jobs))
        for job in removed_jobs:
            cursor.execute(
                "UPDATE agent_jobs SET removed = 1 WHERE job = ? AND agent_id = ?",
                (
                    job,
                    agent_id,
                ),
            )

        cursor.execute(
            "UPDATE agent_jobs SET disabled = 1 WHERE agent_id IN (SELECT agent_id FROM agent WHERE agent_ip = ? AND detained = 1)",
            (agent_ip,),
        )


def validate_resource_limit(cursor):
    cursor.execute(
        "SELECT agent_ip, CAST(agent_memory_mb AS FLOAT) AS agent_memory_mb, CAST(agent_cpu AS FLOAT) AS agent_cpu FROM agent"
    )
    agents = cursor.fetchall()

    for agent_ip, agent_memory_mb, agent_cpu in agents:
        jobs = maand_data.get_agent_jobs(cursor, agent_ip)

        total_allocated_memory = 0
        total_allocated_cpu = 0
        for job in jobs:
            min_memory_mb, max_memory_mb, min_cpu_mhz, max_cpu_mhz = (
                job_data.get_job_resource_limits(cursor, job)
            )

            namespace = f"vars/job/{job}"
            job_cpu = float(kv_manager.get(cursor, namespace, "cpu") or "0")
            job_memory = float(kv_manager.get(cursor, namespace, "memory") or "0")

            if min_memory_mb > max_memory_mb:
                raise Exception(
                    f"Memory allocation for job {job} is invalid. "
                    f"Minimum allowed: {min_memory_mb} MB, Maximum allowed: {max_memory_mb} MB."
                )

            if min_cpu_mhz > max_cpu_mhz:
                raise Exception(
                    f"CPU allocation for job {job} is invalid. "
                    f"Minimum allowed: {min_cpu_mhz} MHZ, Maximum allowed: {max_cpu_mhz} MHZ."
                )

            if min_memory_mb > 0 and min_memory_mb > job_memory:
                raise Exception(
                    f"Memory allocation for job {job} is invalid. "
                    f"Minimum allowed: {min_memory_mb} MB, Allocated: {job_memory} MB."
                )

            if max_memory_mb > 0 and max_memory_mb < job_memory:
                raise Exception(
                    f"Memory allocation for job {job} is invalid. "
                    f"Maximum allowed: {max_memory_mb} MB, Allocated: {job_memory} MB."
                )

            if min_cpu_mhz > 0 and min_cpu_mhz > job_cpu:
                raise Exception(
                    f"CPU allocation for job {job} is invalid. "
                    f"Minimum allowed: {min_cpu_mhz} MHZ, Allocated: {job_cpu} MHZ."
                )

            if max_cpu_mhz > 0 and max_cpu_mhz < job_cpu:
                raise Exception(
                    f"CPU allocation for job {job} is invalid. "
                    f"Maximum allowed: {max_cpu_mhz} MHZ, Allocated: {job_cpu} MHZ."
                )

            total_allocated_memory += job_memory
            total_allocated_cpu += job_cpu

            if total_allocated_memory > agent_memory_mb:
                raise Exception(
                    f"Agent {agent_ip} has insufficient memory for job {job}. "
                    f"Available: {agent_memory_mb} MB, Required: {total_allocated_memory} MB."
                )
            if total_allocated_cpu > agent_cpu:
                raise Exception(
                    f"Agent {agent_ip} has insufficient CPU for job {job}. "
                    f"Available: {agent_cpu} MHZ, Required: {total_allocated_cpu} MHZ."
                )

        cursor.execute(
            "SELECT GROUP_CONCAT(job) AS jobs, port FROM (SELECT (SELECT name AS job FROM job WHERE job_id = jp.job_id) AS job, name, port FROM job_ports jp WHERE port IN (SELECT port FROM job_ports GROUP BY port HAVING COUNT(port) > 1)) GROUP BY port;"
        )
        rows = cursor.fetchall()
        msg = []
        for (
            jobs,
            port,
        ) in rows:
            msg.append(f"jobs: {jobs}, on port: {port}")
        if msg:
            msg = ",".join(msg)
            raise Exception(f"port collision detected: {msg}")


def build(cursor):
    build_allocated_jobs(cursor)
    validate_resource_limit(cursor)
