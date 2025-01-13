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


def build():
    with maand_data.get_db() as db:
        cursor = db.cursor()
        try:
            build_agents.build(cursor)
            build_jobs.build(cursor)
            build_allocations.build(cursor)
            build_variables.build(cursor)
            build_certs.build(cursor)
            db.commit()
            post_build_hook(cursor)
        except Exception as e:
            logger.error(e)
            db.rollback()
            sys.exit(1)


if __name__ == "__main__":
    build()
