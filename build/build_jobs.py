import configparser
import hashlib
import json
import os
import uuid

import jsonschema
from jsonschema import Draft202012Validator

from core import const, job_data, maand_data, utils, workspace
import kv_manager

logger = utils.get_logger()


def delete_job(cursor, job):
    cursor.execute("DELETE FROM job_ports WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", (job,))
    cursor.execute("DELETE FROM job_labels WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
                   (job,))
    cursor.execute("DELETE FROM job_certs WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", (job,))
    cursor.execute("DELETE FROM job_commands WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
                   (job,))
    cursor.execute("DELETE FROM job_files WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", (job,))
    cursor.execute("DELETE FROM job WHERE name = ?", (job,))


def build_deployment_seq(cursor):
    sql = '''
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
        '''

    cursor.execute(sql)


def is_command_file_exists(job, command):
    return os.path.isfile(f"/bucket/workspace/jobs/{job}/_modules/{command}.py")


def get_job_cluster_level_value(job):
    jobs_conf_path = utils.get_maand_jobs_conf()
    path = f"/bucket/{jobs_conf_path}"

    config_parser = configparser.ConfigParser()
    config_parser.read(path)

    name = f"{job}.variables"
    job_kv = {}
    if config_parser.has_section(name):
        keys = config_parser.options(name)
        for key in keys:
            key = key.upper()
            value = config_parser.get(name, key)
            job_kv[key] = value

    if "MEMORY" in job_kv:
        job_kv["MEMORY"] = float(utils.extract_size_in_mb(job_kv.get("MEMORY")))
    if "CPU" in job_kv:
        job_kv["CPU"] = float(utils.extract_cpu_frequency_in_mhz(job_kv.get("CPU")))

    return job_kv


def build_jobs(cursor, job):
    values = {}
    delete_job(cursor, job)

    schema = {
        "type": "object",
        "properties": {
            "version": {"type": "string"},
            "labels": {
                "type": "array",
                "items": {"type": "string"}
            },
            "resources": {
                "type": "object",
                "properties": {
                    "memory": {
                        "type": "object",
                        "properties": {
                            "min": {"type": "string"},
                            "max": {"type": "string"}
                        },
                        "additionalProperties": False
                    },
                    "cpu": {
                        "type": "object",
                        "properties": {
                            "min": {"type": "string"},
                            "max": {"type": "string"}
                        },
                        "additionalProperties": False
                    },
                    "ports": {
                        "type": "object",
                        "patternProperties": {
                            "^port_": {"type": ["string", "object"]}
                        },
                        "additionalProperties": False
                    }
                },
                "additionalProperties": False
            },
            "certs": {
                "type": "array",
                "items": {
                    "type": "object",
                    "patternProperties": {
                        ".*": {"type": ["string", "object"]}
                    }
                }
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
                                    ]
                                }
                            },
                            "depend_on": {
                                "type": "object",
                                "properties": {
                                    "job": {"type": "string"},
                                    "command": {"type": "string"},
                                    "config": {"type": "object"}
                                },
                                "additionalProperties": False
                            }
                        },
                        "additionalProperties": False
                    }
                },
                "additionalProperties": False
            }
        },
        "additionalProperties": False
    }

    manifest = workspace.get_job_manifest(job)

    jsonschema.validate(instance=manifest, schema=schema, format_checker=Draft202012Validator.FORMAT_CHECKER, )

    files = workspace.get_job_files(job)

    labels = manifest.get("labels")
    certs = manifest.get("certs")
    version = manifest.get("version", "unknown")
    commands = manifest.get("commands")

    labels = list(set(labels))

    job_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, str(job)))
    min_memory_limit = float(
        utils.extract_size_in_mb(manifest.get("resources", {}).get("memory", {}).get("min", "0 MB")))
    max_memory_limit = float(
        utils.extract_size_in_mb(manifest.get("resources", {}).get("memory", {}).get("max", "0 MB")))
    min_cpu_limit = float(
        utils.extract_cpu_frequency_in_mhz(manifest.get("resources", {}).get("cpu", {}).get("min", "0 MHZ")))
    max_cpu_limit = float(
        utils.extract_cpu_frequency_in_mhz(manifest.get("resources", {}).get("cpu", {}).get("max", "0 MHZ")))
    ports = manifest.get("resources", {}).get("ports", {})
    certs_hash = hashlib.md5(json.dumps(certs).encode()).hexdigest()

    cursor.execute(
        "INSERT INTO job (job_id, name, version, min_memory_mb, max_memory_mb, min_cpu, max_cpu, certs_md5_hash, deployment_seq) "
        "VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)",
        (job_id, job, version, min_memory_limit, max_memory_limit, min_cpu_limit, max_cpu_limit, certs_hash))

    values["MIN_MEMORY_LIMIT"] = min_memory_limit
    values["MAX_MEMORY_LIMIT"] = max_memory_limit
    values["MIN_CPU_LIMIT"] = min_cpu_limit
    values["MAX_CPU_LIMIT"] = max_cpu_limit

    for label in labels:
        cursor.execute("INSERT INTO job_labels (job_id, label) VALUES (?, ?)", (job_id, label,))

    for cert in certs:
        for name, config in cert.items():
            pkcs8 = config.get("pkcs8", 0)
            subject = config.get("subject", "")
            cursor.execute("INSERT INTO job_certs (job_id, name, pkcs8, subject) VALUES (?, ?, ?, ?)",
                           (job_id, name, pkcs8, subject,))

    for command, command_obj in commands.items():
        executed_on = command_obj.get("executed_on", ["direct"])
        depend_on = command_obj.get("depend_on", {})
        if executed_on:
            depend_on_job = depend_on.get("job")
            jobs = job_data.get_jobs(cursor)
            if depend_on_job and depend_on_job not in jobs:
                logger.error(f"{depend_on_job} job not found: command: {command}, depend on job: {depend_on_job}")
            depend_on_command = depend_on.get("command")
            depend_on_config = json.dumps(depend_on.get("config", {}))

            if depend_on_job:
                if not is_command_file_exists(depend_on_job, depend_on_command):
                    raise Exception(f"job {job}, alloc command not found depend_on job {job} alloc_command {command}")

            for on in executed_on:
                cursor.execute(
                    "INSERT INTO job_commands (job_id, job_name, name, executed_on, depend_on_job, depend_on_command, depend_on_config) VALUES (?, ?, ?, ?, ?, ?, ?)",
                    (job_id, job, command, on, depend_on_job, depend_on_command, depend_on_config))
        else:
            logger.error(f"The commands must include an 'executed_on'. job: {job}, command: {command}")

        if not is_command_file_exists(job, command):
            raise Exception(f"alloc command not found job {job} alloc_command {command}")

    for file in files:
        isdir = os.path.isdir(f"{const.WORKSPACE_PATH}/jobs/{file}")
        content = ""
        if not isdir:
            with open(f"{const.WORKSPACE_PATH}/jobs/{file}", 'rb') as f:
                content = f.read()
        cursor.execute("INSERT INTO job_files (job_id, path, content, isdir) VALUES (?, ?, ?, ?)",
                       (job_id, file, content, isdir))

    for name, port in ports.items():
        name = name.upper()
        cursor.execute("INSERT INTO job_ports (job_id, name, port) VALUES (?, ?, ?)", (job_id, name, port,))
        values[name] = port

    return values


def build_maand_jobs_conf(job):
    values = {}
    jobs_conf_path = utils.get_maand_jobs_conf()
    path = f"/bucket/{jobs_conf_path}"
    if not os.path.exists(path):
        return

    config_parser = configparser.ConfigParser()
    config_parser.read(path)

    kv = get_job_cluster_level_value(job)
    for key, value in kv.items():
        values[key] = value
    return values


def manage_kv(cursor, namespace, values):
    for key, value in values.items():
        kv_manager.put(cursor, namespace, key, str(value))

    keys = values.keys()
    keys = [key.upper() for key in keys]
    all_keys = kv_manager.get_keys(cursor, namespace)
    missing_keys = list(set(all_keys) ^ set(keys))
    for key in missing_keys:
        kv_manager.delete(cursor, namespace, key)


def cleanup_polluted_memory_cpu_settings(job_variables, values):
    if not job_variables.get("MEMORY"):
        job_variables["MEMORY"] = values.get("MAX_MEMORY_LIMIT")
    if not job_variables.get("CPU"):
        job_variables["CPU"] = values.get("MAX_CPU_LIMIT")

    found_memory_settings = False
    for k in ["MIN_MEMORY_LIMIT", "MAX_MEMORY_LIMIT", "MEMORY"]:
        if k in values and values[k] != 0.0:
            found_memory_settings = True
        if k in job_variables and job_variables[k] != 0.0:
            found_memory_settings = True

    found_cpu_settings = False
    for k in ["MIN_CPU_LIMIT", "MAX_CPU_LIMIT", "CPU"]:
        if k in values and values[k] != 0.0:
            found_cpu_settings = True
        if k in job_variables and job_variables[k] != 0.0:
            found_cpu_settings = True

    if not found_memory_settings:
        for k in ["MIN_MEMORY_LIMIT", "MAX_MEMORY_LIMIT", "MEMORY"]:
            if k in values:
                del values[k]
            if k in job_variables:
                del job_variables[k]

    if not found_cpu_settings:
        for k in ["MIN_CPU_LIMIT", "MAX_CPU_LIMIT", "CPU"]:
            if k in values:
                del values[k]
            if k in job_variables:
                del job_variables[k]


def build(cursor):
    jobs = workspace.get_jobs()
    for job in jobs:
        values = build_jobs(cursor, job) or {}
        job_variables = build_maand_jobs_conf(job) or {}
        build_deployment_seq(cursor)

        cleanup_polluted_memory_cpu_settings(job_variables, values)

        manage_kv(cursor, f"vars/job/{job}", values)
        manage_kv(cursor, f"{job}.variables", job_variables)

    cursor.execute("SELECT name FROM job")
    all_jobs = [row[0] for row in cursor.fetchall()]
    missing_jobs = list(set(jobs) ^ set(all_jobs))
    for job in missing_jobs:
        delete_job(cursor, job)

    agents = maand_data.get_agents(cursor, labels_filter=None)
    for agent_ip in agents:
        agent_removed_jobs = maand_data.get_agent_removed_jobs(cursor, agent_ip)
        for job in agent_removed_jobs:
            for namespace in [f"job/{job}", f"vars/job/{job}", f"{job}.variables"]:
                deleted_keys = kv_manager.get_keys(cursor, namespace)
                for key in deleted_keys:
                    kv_manager.delete(cursor, namespace, key)
