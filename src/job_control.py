import argparse
import os
from concurrent.futures import ThreadPoolExecutor, wait

import alloc_command_executor
import command_helper
import context_manager
import job_data
import job_health_check
import maand_data


def get_args():
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


def run_target(target, action, job, allocations, alloc_health_check_flag=False, job_health_check_flag=False):
    with maand_data.get_db() as db:
        cursor = db.cursor()

        available_allocations = maand_data.get_allocations(cursor, job)
        # Run pre-target commands
        pre_commands = job_data.get_job_commands(cursor, job, f"pre_{action}")
        execute_commands(cursor, pre_commands, job, available_allocations, target)

        # Run main job control or default action
        job_control_commands = job_data.get_job_commands(cursor, job, "job_control")
        if job_control_commands:
            execute_commands(cursor, job_control_commands, job, allocations, target, alloc_health_check_flag)
        else:
            execute_default_action(job, allocations, target, alloc_health_check_flag)

        # Perform job-level health checks if needed
        if job_health_check_flag:
            job_health_check.health_check(cursor, [job], wait=True)

        # Run post-target commands
        post_commands = job_data.get_job_commands(cursor, job, f"post_{action}")
        execute_commands(cursor, post_commands, job, available_allocations, target)


def execute_commands(cursor, commands, job, allocations, target, alloc_health_check=False):
    for command in commands:
        alloc_command_executor.prepare_command(cursor, job, command)
        for agent_ip in allocations:
            alloc_command_executor.execute_alloc_command(cursor, job, command, agent_ip, {"TARGET": target})
            if alloc_health_check:
                job_health_check.health_check(cursor, [job], wait=True)


def execute_default_action(job, allocations, target, alloc_health_check):
    bucket = os.getenv("BUCKET")
    for agent_ip in allocations:
        agent_env = context_manager.get_agent_minimal_env(agent_ip)
        command_helper.capture_command_remote(
            f"python3 /opt/agent/{bucket}/bin/runner.py {bucket} {target} --jobs {job}",
            env=agent_env, prefix=agent_ip
        )
        if alloc_health_check:
            with maand_data.get_db() as db:
                cursor = db.cursor()
                job_health_check.health_check(cursor, [job], wait=True)


def main():
    args = get_args()

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
                        if job in maand_data.get_agent_jobs(cursor, agent_ip)
                           and maand_data.get_agent_jobs_and_status(cursor, agent_ip)[job].get("disabled") == 0
                    ]

                if allocations:
                    job_allocations[job] = allocations

            target = args.target.lower()
            with ThreadPoolExecutor() as executor:
                futures = [
                    executor.submit(run_target, target, target, job, allocations, args.alloc_health_check,
                                    args.job_health_check)
                    for job, allocations in job_allocations.items()
                ]

                # Wait for all tasks to complete
                wait(futures)

                # Check for exceptions
                for future in futures:
                    future.result()  # Will raise an exception if one occurred


if __name__ == "__main__":
    main()
