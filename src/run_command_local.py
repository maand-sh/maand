import argparse
import os

import command_manager
import const
import context_manager
import maand_data
import system_manager


def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument('--agents', default="")
    parser.add_argument('--labels', default="")
    parser.add_argument('--concurrency', default="4", type=int)
    parser.add_argument('--cmd', default="")
    parser.add_argument('--no-check', action='store_true')
    parser.set_defaults(no_check=False)
    args = parser.parse_args()

    if args.agents:
        args.agents = args.agents.split(',')
    if args.labels:
        args.labels = args.labels.split(',')

    return args


def run_command(agent_ip):
    with maand_data.get_db() as db:
        cursor = db.cursor()
        env = context_manager.get_agent_env(cursor, agent_ip)
        command_manager.capture_command_local(f"sh {const.WORKSPACE_PATH}/command.sh", env=env, prefix=agent_ip)


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
        if not args.no_check:
            system_manager.run(cursor, context_manager.validate_cluster_update_seq)
        system_manager.run(cursor, run_command, concurrency=args.concurrency, labels_filter=args.labels,
                           agents_filter=args.agents)
