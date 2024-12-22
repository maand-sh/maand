import argparse

import command_helper
import context_manager
import maand
import system_manager

def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument('--agents', default="")
    parser.add_argument('--labels', default="")
    parser.set_defaults(no_check=False)
    args = parser.parse_args()

    if args.agents:
        args.agents = args.agents.split(',')
    if args.labels:
        args.labels = args.labels.split(',')

    return args

def run_command(agent_ip):
    agent_env = context_manager.get_agent_minimal_env(agent_ip)
    command_helper.capture_command_remote("uptime", env=agent_env, prefix=agent_ip)

if __name__ == "__main__":
    args = get_args()

    with maand.get_db() as db:
        cursor = db.cursor()

        context_manager.export_env_bucket_update_seq(cursor)
        system_manager.run(cursor, command_helper.scan_agent, labels_filter=args.labels, agents_filter=args.agents)
        system_manager.run(cursor, run_command, labels_filter=args.labels, agents_filter=args.agents)
