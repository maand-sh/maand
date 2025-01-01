import argparse
import os
from concurrent.futures import ThreadPoolExecutor, wait

import alloc_command_executor
import command_helper
import context_manager
import job_data
import job_health_check
import maand_data


def get_args_agents_jobs_health_check():
    parser = argparse.ArgumentParser()
    parser.add_argument('--agents', default="")
    parser.add_argument('--jobs', default="")
    parser.add_argument('--target', default="", required=True)
    parser.add_argument('--job_health_check', action='store_true')
    parser.add_argument('--alloc_health_check', action='store_true')
    parser.set_defaults(job_health_check=False)
    parser.set_defaults(alloc_health_check=False)

    args = parser.parse_args()

    args.agents = args.agents.split(',') if args.agents else []
    args.jobs = args.jobs.split(',') if args.jobs else []

    return args


def run_target(target, job, allocations):
    args = get_args_agents_jobs_health_check()
    with maand_data.get_db() as db:
        cursor = db.cursor()

        job_commands = job_data.get_job_commands(cursor, job, f"pre_{target}")
        for command in job_commands:
            alloc_command_executor.prepare_command(cursor, job, command)
            for agent_ip in allocations:
                alloc_command_executor.execute_alloc_command(cursor, job, command, agent_ip, {"TARGET": args.target})

        job_commands = job_data.get_job_commands(cursor, job, "job_control")
        if len(job_commands) == 0:
            for agent_ip in allocations:
                bucket = os.getenv("BUCKET")
                agent_env = context_manager.get_agent_minimal_env(agent_ip)
                command_helper.capture_command_remote(
                    f"python3 /opt/agent/{bucket}/bin/runner.py {bucket} {target} --jobs {job}", env=agent_env,
                    prefix=agent_ip)
                if args.alloc_health_check:
                    job_health_check.health_check(cursor, [job], wait=True)
        else:
            alloc_command_executor.prepare_command(cursor, job, "job_control")
            for command in job_commands:
                for agent_ip in allocations:
                    alloc_command_executor.execute_alloc_command(cursor, job, command, agent_ip,
                                                                 {"TARGET": args.target})
                    if args.alloc_health_check:
                        job_health_check.health_check(cursor, [job], wait=True)

        if args.job_health_check:
            job_health_check.health_check(cursor, [job], wait=True)

        job_commands = job_data.get_job_commands(cursor, job, f"post_{target}")
        for command in job_commands:
            alloc_command_executor.prepare_command(cursor, job, command)
            for agent_ip in allocations:
                alloc_command_executor.execute_alloc_command(cursor, job, command, agent_ip, {"TARGET": args.target})


def main():
    args = get_args_agents_jobs_health_check()

    with maand_data.get_db() as db:
        cursor = db.cursor()

        context_manager.export_env_bucket_update_seq(cursor)
        max_deployment_seq = job_data.get_max_deployment_seq(cursor)

        for seq in range(0, max_deployment_seq + 1):
            jobs = job_data.get_jobs(cursor, deployment_seq=seq)
            if args.jobs:
                jobs = list(set(jobs) & set(args.jobs))

            job_allocations = {}
            for job in jobs:
                allocations = maand_data.get_allocations(cursor, job)
                if args.agents:
                    allocations = list(set(allocations) & set(args.agents))

                if args.target != "stop":
                    allocations = [
                        agent_ip for agent_ip in allocations
                        if job in maand_data.get_agent_jobs(cursor, agent_ip).keys()
                           and maand_data.get_agent_jobs(cursor, agent_ip)[job].get("disabled") == 0
                    ]

                if allocations:
                    job_allocations[job] = allocations

            with ThreadPoolExecutor() as executor:
                futures = [
                    executor.submit(run_target, args.target, job, allocations)
                    for job, allocations in job_allocations.items()
                ]

                # Wait for all tasks to complete
                wait(futures)

                # Check for exceptions
                for future in futures:
                    future.result()  # Will raise an exception if one occurred


if __name__ == "__main__":
    main()
