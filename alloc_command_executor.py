import json
import os
import shutil
import subprocess
import sys

from core import context_manager, job_data, maand_data
from core import utils

logger = utils.get_logger()


def execute_alloc_command(cursor, job, command, agent_ip, env):
    allocation_env = context_manager.get_agent_env(cursor, agent_ip)
    allocation_env["JOB"] = job
    for k, v in env.items():
        allocation_env[k] = v

    values = context_manager.get_job_env(cursor, job)
    for k, v in values.items():
        allocation_env[k] = v

    for key, value in os.environ.items():
        if key.startswith("MAAND_"):
            allocation_env[key] = value

    try:
        args = ["python3", f"/modules/{job}/_modules/{command}.py"]
        args.extend(sys.argv[1:])
        process = subprocess.Popen(
            args,
            cwd=f"/modules/{job}/_modules",
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=allocation_env,
            text=True
        )

        for line in process.stdout:
            line = line.strip()
            print(line, flush=True)

        for line in process.stderr:
            line = line.strip()
            print(line, flush=True)

        process.wait()
        return process.returncode == 0
    except Exception as e:
        raise Exception(f"error job: {job}, allocation: {agent_ip}, command: {command}, error: {e}")


def prepare_command(cursor, job, command):
    context_manager.export_env_bucket_update_seq(cursor)
    job_data.copy_job_modules(cursor, job)

    shutil.copy("/maand/stdlib.py", f"/modules/{job}/_modules/stdlib.py")
    cursor.execute(
        "SELECT job_name, name, depend_on_config FROM job_db.job_commands WHERE depend_on_job = ? AND depend_on_command = ?",
        (job, command))
    rows = cursor.fetchall()
    demands = []
    for depend_on_job, depend_on_command, depend_on_config in rows:
        demands.append({"job": depend_on_job, "command": depend_on_command, "config": json.loads(depend_on_config)})
    with open(f"/modules/{job}/_modules/demands.json", "w") as f:
        f.write(json.dumps(demands))


def main():
    job = os.environ.get("JOB")
    command = os.environ.get("COMMAND")
    event = os.environ.get("EVENT", "direct")

    with maand_data.get_db() as db:
        cursor = db.cursor()

        commands = job_data.get_job_commands(cursor, job, event)
        if command not in commands:
            raise Exception(f"job: {job}, command: {command}, event {event} not found")

        prepare_command(cursor, job, command)

        result = True
        allocations = maand_data.get_allocations(cursor, job)
        for agent_ip in allocations:
            result = result and execute_alloc_command(cursor, job, command, agent_ip, {})

        if not result:
            sys.exit(1)


if __name__ == '__main__':
    main()
