import argparse
import os
import time

from core import command_manager, context_manager, const, maand_data, job_health_check, utils, system_manager


def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument('--agents', default="")
    parser.add_argument('--labels', default="")
    parser.add_argument('--cmd', default="")
    parser.add_argument('--concurrency', default="4", type=int)
    parser.add_argument('--disable-cluster-check', action='store_true')
    parser.add_argument('--health_check', action='store_true')
    parser.add_argument('--local', action='store_true')

    parser.set_defaults(disable_cluster_check=False)
    parser.set_defaults(health_check=False)
    parser.set_defaults(local=False)
    args = parser.parse_args()

    if args.agents:
        args.agents = args.agents.split(',')
    if args.labels:
        args.labels = args.labels.split(',')

    return args


def run_command(agent_ip):
    args = get_args()
    env = context_manager.get_agent_minimal_env(agent_ip)
    with maand_data.get_db() as db:
        cursor = db.cursor()
        jobs = maand_data.get_agent_jobs(cursor, agent_ip)

        if args.health_check and not job_health_check.health_check(cursor, jobs, False, times=20, interval=5):
            utils.stop_the_world()

        if args.local:
            command_manager.capture_command_local(f"sh {const.WORKSPACE_PATH}/command.sh", env=env, prefix=agent_ip)
        else:
            command_manager.capture_command_file_remote(f"{const.WORKSPACE_PATH}/command.sh", env, prefix=agent_ip)

        time.sleep(5)
        if args.health_check and not job_health_check.health_check(cursor, jobs, True, times=20, interval=5):
            utils.stop_the_world()


if __name__ == "__main__":
    args = get_args()
    if args.cmd:
        with open(f"{const.WORKSPACE_PATH}/command.sh", "w") as f:
            f.write(args.cmd)

    if not os.path.exists(f"{const.WORKSPACE_PATH}/command.sh"):
        raise Exception("No command file found")

    with maand_data.get_db() as db:
        cursor = db.cursor()

        context_manager.export_env_bucket_update_seq(cursor)
        system_manager.run(cursor, command_manager.scan_agent)

        if not args.disable_cluster_check:
            system_manager.run(cursor, context_manager.validate_cluster_update_seq)

        system_manager.run(
            cursor,
            run_command,
            concurrency=args.concurrency,
            labels_filter=args.labels,
            agents_filter=args.agents,
        )
