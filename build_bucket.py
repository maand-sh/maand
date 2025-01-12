import re
import sys
from copy import deepcopy

from build import build_jobs, build_agents, build_allocations, build_variables, build_certs
from core import job_data, maand_data, utils, workspace
import alloc_command_executor
import kv_manager

logger = utils.get_logger()


def post_build_hook(cursor):
    target = "post_build"
    jobs = job_data.get_jobs(cursor)
    for job in jobs:
        job_data.copy_job_modules(cursor, job)
        allocations = maand_data.get_allocations(cursor, job)
        job_commands = job_data.get_job_commands(cursor, job, target)
        for command in job_commands:
            alloc_command_executor.prepare_command(cursor, job, command)
            for agent_ip in allocations:
                r = alloc_command_executor.execute_alloc_command(cursor, job, command, agent_ip, {"TARGET": target})
                if not r:
                    raise Exception(
                        f"error job: {job}, allocation: {agent_ip}, command: {command}, error: failed with error code")


def get_reversed_keys():
    reversed_keys = ["JOB", "EVENT", "LABELS", "AGENT_IP", "MAAND_.*", "CPU", "MEMORY", "AGENT_CPU", "AGENT_MEMORY",
                     "MAX_CPU_LIMIT", "MIN_CPU_LIMIT", "MAX_MEMORY_LIMIT", "MIN_MEMORY_LIMIT", "COMMAND"]
    labels = workspace.get_labels()
    labels.append("agent")
    for label in labels:
        key = label.upper()
        reversed_keys.append(f"{key}_NODES")
        reversed_keys.append(f"{key}_PEERS")
        reversed_keys.append(f"{key}_LENGTH")
        reversed_keys.append(f"{key}_LABEL_ID")
        reversed_keys.append(f"{key}_ALLOCATION_INDEX")
        reversed_keys.append(f"{key}\\_\\d+")
    return reversed_keys


def validate_kv(cursor):
    variables_env_keys = kv_manager.get_keys(cursor, "variables.env")
    reversed_keys = get_reversed_keys()
    for reversed_key in reversed_keys:
        for key in variables_env_keys:
            if key == reversed_key:
                raise Exception(f"'{key}' found in variables.env, key '{key}' is not allowed")
            if re.search(reversed_key, key):
                raise Exception(f"'{key}' found in variables.env, pattern '{reversed_key}' is not allowed")

    jobs_reversed_keys = deepcopy(reversed_keys)
    jobs_reversed_keys.remove("CPU")
    jobs_reversed_keys.remove("MEMORY")
    for job in workspace.get_jobs():
        jobs_variables = kv_manager.get_keys(cursor, f"{job}.variables")
        for reversed_key in jobs_reversed_keys:
            for key in jobs_variables:
                if key == reversed_key:
                    raise Exception(f"'{key}' found in {job}.variables, key '{key}' is not allowed")
                if re.search(reversed_key, key):
                    raise Exception(f"'{key}' found in {job}.variables, pattern '{reversed_key}' is not allowed")


def build():
    with maand_data.get_db() as db:
        cursor = db.cursor()
        try:
            build_agents.build(cursor)
            build_jobs.build(cursor)
            build_allocations.build(cursor)
            build_variables.build(cursor)
            build_certs.build(cursor)
            validate_kv(cursor)
            db.commit()
            post_build_hook(cursor)
        except Exception as e:
            logger.error(e)
            db.rollback()
            sys.exit(1)


if __name__ == "__main__":
    build()
