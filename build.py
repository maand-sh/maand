import base64
import copy
import hashlib
import json
import os
import sys
import uuid

import jsonschema
from jsonschema import Draft202012Validator

import alloc_command_executor
import kv_manager
from core import maand_data, utils, workspace, const, context_manager, command_manager, cert_provider

logger = utils.get_logger()


def build_agent_tags(cursor, agent_id, agent):
    try:
        cursor.execute("DELETE FROM agent_tags WHERE agent_id = ?", (agent_id,))
        tags = agent.get("tags", {})
        for key, value in tags.items():
            key = key.lower()
            value = str(value)
            cursor.execute(
                "INSERT INTO agent_tags (agent_id, key, value) VALUES (?, ?, ?)",
                (
                    agent_id,
                    key,
                    value,
                ),
            )
    except Exception as e:
        logger.error(f"Error building agent tags for agent_id {agent_id}: {e}")
        raise


def build_agent_labels(cursor, agent_id, agent):
    try:
        cursor.execute("DELETE FROM agent_labels WHERE agent_id = ?", (agent_id,))
        labels = agent.get("labels", [])
        labels.append("agent")
        labels = list(set(labels))  # Remove duplicates
        for label in labels:
            cursor.execute(
                "INSERT INTO agent_labels (agent_id, label) VALUES (?, ?)",
                (
                    agent_id,
                    label,
                ),
            )
    except Exception as e:
        logger.error(f"Error building agent labels for agent_id {agent_id}: {e}")
        raise


def build_agent_resources(cursor, agent_ip):
    try:
        namespace = f"maand/agent/{agent_ip}"
        available_memory, available_cpu = maand_data.get_agent_available_resources(
            cursor, agent_ip
        )
        if available_memory != "0.0":
            kv_manager.put(cursor, namespace, "agent_memory", available_memory)
        if available_cpu != "0.0":
            kv_manager.put(cursor, namespace, "agent_cpu", available_cpu)
    except Exception as e:
        logger.error(f"Error building agent resources for {agent_ip}: {e}")
        raise


def build_agents(cursor):
    try:
        agents = workspace.get_agents()

        schema = {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "host": {"type": "string", "format": "ipv4"},
                    "labels": {"type": "array", "items": {"type": "string"}},
                    "cpu": {"type": "string"},
                    "memory": {"type": "string"},
                },
                "required": ["host"],
            },
        }

        # Validate agent data using JSON schema
        jsonschema.validate(
            instance=agents,
            schema=schema,
            format_checker=Draft202012Validator.FORMAT_CHECKER,
        )

        for index, agent in enumerate(agents):
            agent_ip = agent.get("host")
            agent_memory = float(utils.extract_size_in_mb(agent.get("memory", "0 MB")))
            agent_cpu = float(utils.extract_cpu_frequency_in_mhz(agent.get("cpu", "0 MHZ")))

            cursor.execute("SELECT agent_id FROM agent WHERE agent_ip = ?", (agent_ip,))
            row = cursor.fetchone()

            if row:
                agent_id = row[0]
            else:
                agent_id = str(uuid.uuid4())

            if row:
                cursor.execute(
                    "UPDATE agent SET agent_memory_mb = ?, agent_cpu = ?, position = ?, detained = 0 WHERE agent_id = ?",
                    (
                        agent_memory,
                        agent_cpu,
                        index,
                        agent_id,
                    ),
                )
            else:
                cursor.execute(
                    "INSERT INTO agent (agent_id, agent_ip, agent_memory_mb, agent_cpu, detained, position) VALUES (?, ?, ?, ?, 0, ?)",
                    (
                        agent_id,
                        agent_ip,
                        agent_memory,
                        agent_cpu,
                        index,
                    ),
                )

            build_agent_labels(cursor, agent_id, agent)
            build_agent_tags(cursor, agent_id, agent)
            build_agent_resources(cursor, agent_ip)

        # Handle missing agents (detain agents no longer in workspace)
        cursor.execute("SELECT agent_ip FROM agent")
        rows = cursor.fetchall()
        current_agents = [row[0] for row in rows]

        workspace_agents = [agent["host"] for agent in agents]
        missing_agents = list(set(current_agents) - set(workspace_agents))
        for agent_ip in missing_agents:
            cursor.execute("UPDATE agent SET detained = 1 WHERE agent_ip = ?", (agent_ip,))

        cursor.execute("SELECT agent_ip FROM agent WHERE detained = 1")
        rows = cursor.fetchall()
        detained_agents = {row[0] for row in rows}

        for agent_ip in detained_agents:
            for namespace in [
                f"maand/certs/agent/{agent_ip}",
                f"maand/agent/{agent_ip}",
                f"vars/agent/{agent_ip}",
            ]:
                keys = kv_manager.get_keys(cursor, namespace)
                for key in keys:
                    kv_manager.delete(cursor, namespace, key)

    except Exception as e:
        logger.error(f"Error building agents: {e}")
        raise


def build_job_deployment_seq(cursor):
    sql = """
            WITH RECURSIVE job_command_seq AS (
                SELECT jc.job_name, 0 AS level FROM job_commands jc WHERE jc.depend_on_job IS NULL

                UNION ALL

                SELECT jc.job_name, jcs.level + 1 AS level
                FROM
                    job_commands jc INNER JOIN job_command_seq jcs ON jc.depend_on_job = jcs.job_name
            )
            UPDATE job SET deployment_seq = t.deployment_seq FROM (
            SELECT
                DISTINCT job_name, deployment_seq
            FROM
                (SELECT job_name, (SELECT MAX(level) FROM job_command_seq jcs WHERE jcs.job_name = t.job_name) as deployment_seq FROM job_command_seq t) t1
            ORDER BY deployment_seq) t WHERE job.name = t.job_name;
        """

    cursor.execute(sql)


def is_command_file_exists(job, command):
    return os.path.isfile(f"/bucket/workspace/jobs/{job}/_modules/{command}.py")


def get_job_variables(job):
    config_parser = utils.get_maand_jobs_conf()

    name = f"{job}.variables"
    job_kv = {}
    if config_parser.has_section(name):
        keys = config_parser.options(name)
        for key in keys:
            key = key.lower()
            value = config_parser.get(name, key)
            job_kv[key] = value

    if "memory" in job_kv:
        job_kv["memory"] = float(utils.extract_size_in_mb(job_kv.get("memory")))
    if "cpu" in job_kv:
        job_kv["cpu"] = float(utils.extract_cpu_frequency_in_mhz(job_kv.get("cpu")))

    return job_kv


def build_bucket_jobs_conf(job):
    values = {}
    kv = get_job_variables(job)
    for key, value in kv.items():
        values[key] = value
    return values


def delete_job(cursor, job):
    cursor.execute(
        "DELETE FROM job_ports WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
        (job,),
    )
    cursor.execute(
        "DELETE FROM job_labels WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
        (job,),
    )
    cursor.execute(
        "DELETE FROM job_certs WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
        (job,),
    )
    cursor.execute(
        "DELETE FROM job_commands WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
        (job,),
    )
    cursor.execute(
        "DELETE FROM job_files WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
        (job,),
    )
    cursor.execute("DELETE FROM job WHERE name = ?", (job,))


def build_job(cursor, job):
    values = {}
    delete_job(cursor, job)

    schema = {
        "type": "object",
        "properties": {
            "version": {"type": "string"},
            "labels": {"type": "array", "items": {"type": "string"}},
            "resources": {
                "type": "object",
                "properties": {
                    "memory": {
                        "type": "object",
                        "properties": {
                            "min": {"type": "string"},
                            "max": {"type": "string"},
                        },
                        "additionalProperties": False,
                    },
                    "cpu": {
                        "type": "object",
                        "properties": {
                            "min": {"type": "string"},
                            "max": {"type": "string"},
                        },
                        "additionalProperties": False,
                    },
                    "ports": {
                        "type": "object",
                        "patternProperties": {"^port_": {"type": ["string", "object"]}},
                        "additionalProperties": False,
                    },
                },
                "additionalProperties": False,
            },
            "certs": {
                "type": "array",
                "items": {
                    "type": "object",
                    "patternProperties": {".*": {"type": ["string", "object"]}},
                },
            },
            "commands": {
                "type": "object",
                "patternProperties": {
                    "^command_": {
                        "type": "object",
                        "properties": {
                            "executed_on": {
                                "type": "array",
                                "items": {
                                    "type": "string",
                                    "allOf": [
                                        {
                                            "pattern": "^(direct|health_check|post_build|pre_deploy|post_deploy|job_control)$"
                                        }
                                    ],
                                },
                            },
                            "depend_on": {
                                "type": "object",
                                "properties": {
                                    "job": {"type": "string"},
                                    "command": {"type": "string"},
                                    "config": {"type": "object"},
                                },
                                "additionalProperties": False,
                            },
                        },
                        "additionalProperties": False,
                    }
                },
                "additionalProperties": False,
            },
        },
        "additionalProperties": False,
    }

    manifest = workspace.get_job_manifest(job)

    jsonschema.validate(
        instance=manifest,
        schema=schema,
        format_checker=Draft202012Validator.FORMAT_CHECKER,
    )

    files = workspace.get_job_files(job)

    labels = manifest.get("labels")
    certs = manifest.get("certs")
    version = manifest.get("version", "unknown")
    commands = manifest.get("commands")

    labels = list(set(labels))

    job_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, str(job)))
    min_memory_limit = float(
        utils.extract_size_in_mb(
            manifest.get("resources", {}).get("memory", {}).get("min", "0 MB")
        )
    )
    max_memory_limit = float(
        utils.extract_size_in_mb(
            manifest.get("resources", {}).get("memory", {}).get("max", "0 MB")
        )
    )
    min_cpu_limit = float(
        utils.extract_cpu_frequency_in_mhz(
            manifest.get("resources", {}).get("cpu", {}).get("min", "0 MHZ")
        )
    )
    max_cpu_limit = float(
        utils.extract_cpu_frequency_in_mhz(
            manifest.get("resources", {}).get("cpu", {}).get("max", "0 MHZ")
        )
    )
    ports = manifest.get("resources", {}).get("ports", {})
    certs_hash = hashlib.md5(json.dumps(certs).encode()).hexdigest()

    cursor.execute(
        "INSERT INTO job (job_id, name, version, min_memory_mb, max_memory_mb, min_cpu, max_cpu, certs_md5_hash, deployment_seq) "
        "VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)",
        (
            job_id,
            job,
            version,
            min_memory_limit,
            max_memory_limit,
            min_cpu_limit,
            max_cpu_limit,
            certs_hash,
        ),
    )

    values["min_memory_limit"] = min_memory_limit
    values["max_memory_limit"] = max_memory_limit
    values["min_cpu_limit"] = min_cpu_limit
    values["max_cpu_limit"] = max_cpu_limit

    for label in labels:
        cursor.execute(
            "INSERT INTO job_labels (job_id, label) VALUES (?, ?)",
            (
                job_id,
                label,
            ),
        )

    for cert in certs:
        for name, config in cert.items():
            pkcs8 = config.get("pkcs8", 0)
            subject = config.get("subject", "")
            cursor.execute(
                "INSERT INTO job_certs (job_id, name, pkcs8, subject) VALUES (?, ?, ?, ?)",
                (
                    job_id,
                    name,
                    pkcs8,
                    subject,
                ),
            )

    for command, command_obj in commands.items():
        executed_on = command_obj.get("executed_on", ["direct"])
        depend_on = command_obj.get("depend_on", {})
        if executed_on:
            depend_on_job = depend_on.get("job")
            depend_on_command = depend_on.get("command")
            depend_on_config = json.dumps(depend_on.get("config", {}))

            if depend_on_job:
                if not is_command_file_exists(depend_on_job, depend_on_command):
                    raise Exception(
                        f"job {job}, alloc command not found depend_on job {job} alloc_command {command}"
                    )

            for on in executed_on:
                cursor.execute(
                    "INSERT INTO job_commands (job_id, job_name, name, executed_on, depend_on_job, depend_on_command, depend_on_config) VALUES (?, ?, ?, ?, ?, ?, ?)",
                    (
                        job_id,
                        job,
                        command,
                        on,
                        depend_on_job,
                        depend_on_command,
                        depend_on_config,
                    ),
                )
        else:
            logger.error(
                f"The commands must include an 'executed_on'. job: {job}, command: {command}"
            )

        if not is_command_file_exists(job, command):
            raise Exception(
                f"alloc command not found job {job} alloc_command {command}"
            )

    for file in files:
        isdir = os.path.isdir(f"{const.WORKSPACE_PATH}/jobs/{file}")
        content = ""
        if not isdir:
            with open(f"{const.WORKSPACE_PATH}/jobs/{file}", "rb") as f:
                content = f.read()
        cursor.execute(
            "INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)",
            (job_id, file, content, isdir),
        )

    for name, port in ports.items():
        name = name.lower()
        cursor.execute(
            "INSERT INTO job_ports (job_id, name, port) VALUES (?, ?, ?)",
            (
                job_id,
                name,
                port,
            ),
        )
        values[name] = port

    return values


def manage_job_variables(cursor, namespace, values):
    for key, value in values.items():
        kv_manager.put(cursor, namespace, key, str(value))

    keys = values.keys()
    keys = [key.lower() for key in keys]
    all_keys = kv_manager.get_keys(cursor, namespace)
    missing_keys = list(set(all_keys) ^ set(keys))
    for key in missing_keys:
        kv_manager.delete(cursor, namespace, key)


def remove_unused_resource_settings(job_variables, values):
    if not job_variables.get("memory"):
        job_variables["memory"] = values.get("max_memory_limit")
    if not job_variables.get("cpu"):
        job_variables["cpu"] = values.get("max_cpu_limit")

    found_memory_settings = False
    for k in ["min_memory_limit", "max_memory_limit", "memory"]:
        if k in values and values[k] != 0.0:
            found_memory_settings = True
        if k in job_variables and job_variables[k] != 0.0:
            found_memory_settings = True

    found_cpu_settings = False
    for k in ["min_cpu_limit", "max_cpu_limit", "cpu"]:
        if k in values and values[k] != 0.0:
            found_cpu_settings = True
        if k in job_variables and job_variables[k] != 0.0:
            found_cpu_settings = True

    if not found_memory_settings:
        for k in ["min_memory_limit", "max_memory_limit", "memory"]:
            if k in values:
                del values[k]
            if k in job_variables:
                del job_variables[k]

    if not found_cpu_settings:
        for k in ["min_cpu_limit", "max_cpu_limit", "cpu"]:
            if k in values:
                del values[k]
            if k in job_variables:
                del job_variables[k]


def build_jobs(cursor):
    jobs = workspace.get_jobs()
    for job in jobs:
        assert job == job.lower()
        values = build_job(cursor, job) or {}
        job_variables = build_bucket_jobs_conf(job) or {}
        build_job_deployment_seq(cursor)

        remove_unused_resource_settings(job_variables, values)

        manage_job_variables(cursor, f"maand/job/{job}", values)
        manage_job_variables(cursor, f"vars/job/{job}", job_variables)

    cursor.execute("SELECT name FROM job")
    all_jobs = [row[0] for row in cursor.fetchall()]
    missing_jobs = list(set(jobs) ^ set(all_jobs))
    for job in missing_jobs:
        delete_job(cursor, job)

    agents = maand_data.get_agents(cursor, labels_filter=None)
    for agent_ip in agents:
        agent_removed_jobs = maand_data.get_agent_removed_jobs(cursor, agent_ip)
        for job in agent_removed_jobs:
            for namespace in [f"maand/job/{job}", f"vars/job/{job}", f"job/{job}"]:
                deleted_keys = kv_manager.get_keys(cursor, namespace)
                for key in deleted_keys:
                    kv_manager.delete(cursor, namespace, key)


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
                maand_data.get_job_resource_limits(cursor, job)
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


def build_allocations(cursor):
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

    validate_resource_limit(cursor)


def build_bucket_conf(cursor):
    namespace = "maand"

    config_parser = utils.get_bucket_conf()
    key_values = {}
    if config_parser.has_section(namespace):
        keys = config_parser.options(namespace)
        for key in keys:
            key = key.lower()
            value = config_parser.get(namespace, key)
            key_values[key] = value

    for key, value in key_values.items():
        key = key.lower()
        kv_manager.put(cursor, namespace, key, value)

    available_keys = [k.lower() for k in key_values.keys()]
    all_keys = kv_manager.get_keys(cursor, namespace)
    missing_keys = list(set(all_keys) ^ set(available_keys))
    for key in missing_keys:
        kv_manager.delete(cursor, namespace, key)


def build_agent_variables(cursor):
    agents = maand_data.get_agents(cursor, labels_filter=None)

    for agent_ip in agents:
        labels = maand_data.get_agent_labels(cursor, agent_ip=None)
        agent_labels = maand_data.get_agent_labels(cursor, agent_ip=agent_ip)

        values = {}
        for label in labels:
            key_nodes = f"{label}_nodes".lower()

            agents = maand_data.get_agents(cursor, [label])
            values[key_nodes] = ",".join(agents)

            key = f"{label}_length".lower()
            values[key] = str(len(agents))

            for idx, host in enumerate(agents):
                key = f"{label}_{idx}".lower()
                values[key] = host

            if label not in agent_labels:
                continue

            other_agents = copy.deepcopy(agents)
            if agent_ip in other_agents:
                other_agents.remove(agent_ip)

            key_peers = f"{label}_peers".lower()
            if other_agents:
                values[key_peers] = ",".join(other_agents)

            for idx, host in enumerate(agents):
                if host == agent_ip:
                    key = f"{label}_allocation_index".lower()
                    values[key] = str(idx)

            key = f"{label}_label_id".lower()
            values[key] = str(uuid.uuid5(uuid.NAMESPACE_DNS, str(label)))

        values["labels"] = ",".join(sorted(agent_labels))

        agent_tags = maand_data.get_agent_tags(cursor, agent_ip)
        for key, value in agent_tags.items():
            values[key] = value

        agent_memory, agent_cpu = maand_data.get_agent_available_resources(
            cursor, agent_ip
        )
        if agent_memory != "0.0":
            values["agent_memory"] = agent_memory
        if agent_cpu != "0.0":
            values["agent_cpu"] = agent_cpu

        namespace = f"maand/agent/{agent_ip}"
        for key, value in values.items():
            kv_manager.put(cursor, namespace, key, str(value))

        all_keys = kv_manager.get_keys(cursor, namespace)
        missing_keys = list(set(all_keys) ^ set(values.keys()))
        for key in missing_keys:
            kv_manager.delete(cursor, namespace, key)


def build_variables(cursor):
    build_bucket_conf(cursor)
    build_agent_variables(cursor)


def get_cert_if_available(cursor, file_path, namespace, key):
    content = kv_manager.get(cursor, namespace, key)
    if content:
        content = base64.b64decode(content)
        with open(file_path, "wb") as f:
            f.write(content)


def put_cert(cursor, file_path, namespace, key):
    with open(file_path, "rb") as f:
        content = base64.b64encode(f.read()).decode("utf-8")
        kv_manager.put(cursor, namespace, key, content)


def get_ca_file_hash():
    with open("/bucket/secrets/ca.crt", "r") as f:
        data = f.read()
        return hashlib.md5(data.encode("utf-8")).hexdigest()


def build_ca_hash(cursor):
    current_md5_hash = maand_data.get_ca_md5_hash(cursor)
    new_md5_hash = get_ca_file_hash()
    if current_md5_hash != new_md5_hash:
        cursor.execute("UPDATE bucket SET ca_md5_hash = ?", (new_md5_hash,))
        return True
    return False


def build_agent_certs(cursor):
    bucket_id = maand_data.get_bucket_id(cursor)
    agents = maand_data.get_agents(cursor, labels_filter=None)

    current_md5_hash = maand_data.get_ca_md5_hash(cursor)
    new_md5_hash = get_ca_file_hash()
    update_certs = current_md5_hash != new_md5_hash

    for agent_ip in agents:
        agent_dir = context_manager.get_agent_dir(agent_ip)
        agent_cert_location = f"{agent_dir}/certs"
        command_manager.command_local(f"mkdir -p {agent_cert_location}")

        namespace = f"maand/certs/agent/{agent_ip}"
        agent_cert_path = f"{agent_cert_location}/agent"

        if not update_certs:
            get_cert_if_available(
                cursor, f"{agent_cert_path}.key", namespace, "agent.key"
            )
            get_cert_if_available(
                cursor, f"{agent_cert_path}.crt", namespace, "agent.crt"
            )
            get_cert_if_available(
                cursor, f"{agent_cert_path}.pem", namespace, "agent.pem"
            )

        found = (
                os.path.isfile(f"{agent_cert_path}.key")
                and os.path.isfile(f"{agent_cert_path}.crt")
                and os.path.isfile(f"{agent_cert_path}.pem")
        )

        if not found or cert_provider.is_certificate_expiring_soon(
                f"{agent_cert_path}.crt"
        ):
            cert_provider.generate_site_private("agent", agent_cert_location)
            cert_provider.generate_private_pem_pkcs_8("agent", agent_cert_location)
            cert_provider.generate_site_csr(
                "agent", f"/CN={bucket_id}", agent_cert_location
            )
            subject_alt_name = f"DNS.1:localhost,IP.1:127.0.0.1,IP.2:{agent_ip}"
            cert_provider.generate_site_public(
                "agent", subject_alt_name, 60, agent_cert_location
            )

            put_cert(cursor, f"{agent_cert_path}.key", namespace, "agent.key")
            put_cert(cursor, f"{agent_cert_path}.crt", namespace, "agent.crt")
            put_cert(cursor, f"{agent_cert_path}.pem", namespace, "agent.pem")


def build_job_certs(cursor):
    bucket_id = maand_data.get_bucket_id(cursor)
    agents = maand_data.get_agents(cursor, labels_filter=None)
    jobs = maand_data.get_jobs(cursor)
    config_parser = utils.get_maand_conf()

    current_md5_hash = maand_data.get_ca_md5_hash(cursor)
    new_md5_hash = get_ca_file_hash()
    update_certs = current_md5_hash != new_md5_hash

    for agent_ip in agents:
        for job in jobs:
            agent_dir = context_manager.get_agent_dir(agent_ip)
            job_cert_location = f"{agent_dir}/jobs/{job}/certs"
            job_cert_kv_location = f"{job}/certs"
            namespace = f"maand/certs/job/{agent_ip}"

            job_certs = maand_data.get_job_certs_config(cursor, job)

            if not update_certs and job_certs:
                current_hash = kv_manager.get(
                    cursor, namespace, f"{job_cert_kv_location}/md5.hash"
                )
                new_hash = maand_data.get_job_md5_hash(cursor, job)
                if current_hash != new_hash:
                    kv_manager.put(
                        cursor, namespace, f"{job_cert_kv_location}/md5.hash", new_hash
                    )
                    update_certs = True

            for cert in job_certs:
                command_manager.command_local(f"mkdir -p {job_cert_location}")
                name = cert.get("name")
                job_cert_path = f"{job_cert_location}/{name}"

                if not update_certs:
                    get_cert_if_available(
                        cursor,
                        f"{job_cert_path}.key",
                        namespace,
                        f"{job_cert_kv_location}/{name}.key",
                    )
                    get_cert_if_available(
                        cursor,
                        f"{job_cert_path}.crt",
                        namespace,
                        f"{job_cert_kv_location}/{name}.crt",
                    )
                    get_cert_if_available(
                        cursor,
                        f"{job_cert_path}.pem",
                        namespace,
                        f"{job_cert_kv_location}/{name}.pem",
                    )

                found = os.path.isfile(f"{job_cert_path}.key") and os.path.isfile(
                    f"{job_cert_path}.crt"
                )
                if cert.get("pkcs8", False):
                    found = found and os.path.isfile(f"{job_cert_path}.pem")

                if not found or cert_provider.is_certificate_expiring_soon(
                        f"{job_cert_path}.crt"
                ):
                    ttl = config_parser.get("default", "certs_ttl") or 60
                    cert_provider.generate_site_private(name, job_cert_location)
                    if cert.get("pkcs8", 0) == 1:
                        cert_provider.generate_private_pem_pkcs_8(
                            name, job_cert_location
                        )

                    subj = cert.get("subject", f"/CN={bucket_id}")
                    cert_provider.generate_site_csr(name, subj, job_cert_location)
                    subject_alt_name = cert.get(
                        "subject_alt_name",
                        f"DNS.1:localhost,IP.1:127.0.0.1,IP.2:{agent_ip}",
                    )
                    cert_provider.generate_site_public(
                        name, subject_alt_name, ttl, job_cert_location
                    )
                    command_manager.command_local(f"rm -f {job_cert_path}.csr")

                    put_cert(
                        cursor,
                        f"{job_cert_path}.key",
                        namespace,
                        f"{job_cert_kv_location}/{name}.key",
                    )
                    put_cert(
                        cursor,
                        f"{job_cert_path}.crt",
                        namespace,
                        f"{job_cert_kv_location}/{name}.crt",
                    )

                    if cert.get("pkcs8", False):
                        put_cert(
                            cursor,
                            f"{job_cert_path}.pem",
                            namespace,
                            f"{job_cert_kv_location}/{name}.pem",
                        )


def build_certs(cursor):
    build_agent_certs(cursor)
    build_job_certs(cursor)
    build_ca_hash(cursor)
    command_manager.command_local(f"rm -f {const.SECRETS_PATH}/ca.srl")


def post_build_hook(cursor):
    target = "post_build"
    jobs = maand_data.get_jobs(cursor)
    for job in jobs:
        maand_data.setup_job_modules(cursor, job)
        allocations = maand_data.get_allocations(cursor, job)
        job_commands = maand_data.get_job_commands(cursor, job, target)
        for command in job_commands:
            alloc_command_executor.prepare_command(cursor, job, command)
            for agent_ip in allocations:
                r = alloc_command_executor.execute_alloc_command(
                    job, command, agent_ip, {"TARGET": target}
                )
                if not r:
                    raise Exception(
                        f"error job: {job}, allocation: {agent_ip}, command: {command}, error: failed with error code"
                    )


def build():
    with maand_data.get_db() as db:
        cursor = db.cursor()
        try:
            build_agents(cursor)
            build_jobs(cursor)
            build_allocations(cursor)
            build_variables(cursor)
            build_certs(cursor)
            db.commit()
            post_build_hook(cursor)
        except Exception as e:
            logger.error(e)
            db.rollback()
            sys.exit(1)


if __name__ == "__main__":
    build()
