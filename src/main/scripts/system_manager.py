import argparse
import multiprocessing
import os
import uuid

import command_helper
import utils

parser = argparse.ArgumentParser(description="Just an example", formatter_class=argparse.ArgumentDefaultsHelpFormatter)
parser.add_argument("-r", "--roles", help="filter hosts by roles", required=False, default="")
parser.add_argument("-c", "--concurrency", help="max concurrency", required=False, default=0, type=int)
parser.add_argument("-i", "--ignore_error", help="ignore_error", required=False, default=0, type=int)
parser.add_argument("-o", "--operation", help="action", required=True)


def run(work_item):
    agent_ip, command_group, operation, ignore_error = work_item
    image = os.getenv("IMAGE_NAME")
    workspace = os.getenv("WORKSPACE")

    run_id = f"""{command_group}-{agent_ip.replace(".", "-")}"""

    with open(f"/tmp/{run_id}.env", "w") as f:
        f.write(f"RUN_ID={run_id}\n")
        f.write(f"AGENT_IP={agent_ip}\n")
        f.write(f"WORKSPACE={workspace}\n")
        f.write(f"NODE_OPS=1\n")
        f.write(f"OPERATION={os.getenv("OPERATION")}\n")
        if os.getenv("MODULE"):
            f.write(f"MODULE={os.getenv("MODULE")}\n")
    try:
        command_helper.command_local(cmd=f"""
            docker run --env-file /tmp/{run_id}.env -v {workspace}:/workspace -v /var/run/docker.sock:/var/run/docker.sock --name "{run_id}" {image} {operation}
        """)
    except Exception as e:
        if not ignore_error:
            raise e

    command_helper.command_local(cmd=f"docker rm {run_id} > /dev/null 2>&1")


def main():
    args = parser.parse_args()
    command_group = uuid.uuid4()
    config = vars(args)

    ignore_error = config["ignore_error"]
    config["roles"] = [x.strip() for x in config["roles"].split(",") if x.strip()]

    agents = utils.get_agent_and_roles(config["roles"])
    agents_ip = list(agents.keys())

    if len(agents_ip) == 0:
        return

    max_concurrency = config["concurrency"] or len(agents_ip) or 1
    work_items = list(zip(agents_ip, (len(agents_ip) * [str(command_group)]), (len(agents_ip) * [config["operation"]]),
                          (len(agents_ip) * [ignore_error])))

    with multiprocessing.Pool(processes=max_concurrency) as pool:
        pool.map(run, work_items)


if __name__ == "__main__":
    main()
