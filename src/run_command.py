import argparse
import os
import time

import job_health_check
import command_helper
import const
import context_manager
import maand
import system_manager
import utils

def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument('--agents', default="")
    parser.add_argument('--labels', default="")
    parser.add_argument('--concurrency', default="4", type=int)
    parser.add_argument('--no-check', action='store_true')
    parser.add_argument('--health_check', action='store_true')
    parser.set_defaults(no_check=False)
    parser.set_defaults(health_check=False)
    args = parser.parse_args()

    if args.agents:
        args.agents = args.agents.split(',')
    if args.labels:
        args.labels = args.labels.split(',')

    return args


def run_command(agent_ip):
    args = get_args()
    env = context_manager.get_agent_minimal_env(agent_ip)
    with maand.get_db() as db:
        cursor = db.cursor()
        jobs = maand.get_agent_jobs(cursor, agent_ip)

        if args.health_check and not job_health_check.health_check(cursor, jobs, False, times=20, interval=5):
            utils.stop_the_world()
        command_helper.capture_command_file_remote(f"{const.WORKSPACE_PATH}/command.sh", env, prefix=agent_ip)
        time.sleep(5)
        if args.health_check and not job_health_check.health_check(cursor, jobs, True, times=20, interval=5):
            utils.stop_the_world()


if __name__ == "__main__":
    if not os.path.exists(f"{const.WORKSPACE_PATH}/command.sh"):
        raise Exception("No command file found")

    args = get_args()

    with maand.get_db() as db:
        cursor = db.cursor()

        context_manager.export_env_bucket_update_seq(cursor)
        system_manager.run(cursor, command_helper.scan_agent)

        if not args.no_check:
            system_manager.run(cursor, context_manager.validate_cluster_update_seq)

        system_manager.run(
            cursor,
            run_command,
            concurrency=args.concurrency,
            labels_filter=args.labels,
            agents_filter=args.agents,
        )
