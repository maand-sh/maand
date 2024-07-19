import argparse
import os
import uuid
import multiprocessing

import command_helper
import utils

parser = argparse.ArgumentParser(description="Just an example", formatter_class=argparse.ArgumentDefaultsHelpFormatter)
parser.add_argument("-r", "--roles", help="filter hosts by roles", required=False, default="")
parser.add_argument("-c", "--concurrency", help="max concurrency", required=False, default=0, type=int)
parser.add_argument("-i", "--ignore_error", help="ignore_error", required=False, default=0, type=int)
parser.add_argument("-o", "--operation", help="action", required=True)


def run(work_item):
    host, command_group, operation, ignore_error = work_item
    image = os.getenv("IMAGE_NAME")
    workspace = os.getenv("WORKSPACE")

    name = f"""{command_group}-{host.replace(".", "-")}"""
    r = command_helper.command_local(cmd=f"""
        docker run --privileged -e HOST={host} -e NODE_OPS="1" -e WORKSPACE="{workspace}" -v {workspace}:/workspace -v /var/run/docker.sock:/var/run/docker.sock --name "{name}" {image} {operation}
    """, return_error=True)
    if r.returncode != 0:
        raise Exception(r)
    else:
        output = command_helper.command_local(cmd=f"docker logs {name}", return_error=True).stdout.decode('utf-8')
        if output:
            print(output)
        command_helper.command_local(cmd=f"docker rm {name}")


def main():
    args = parser.parse_args()
    command_group = uuid.uuid4()
    config = vars(args)

    ignore_error = config["ignore_error"]
    config["roles"] = [x.strip() for x in config["roles"].split(",") if x.strip()]

    nodes = utils.get_host_roles(config["roles"])
    hosts = list(nodes.keys())

    if len(hosts) == 0:
        return

    max_concurrency = config["concurrency"] or len(hosts) or 1
    work_items = list(zip(hosts, (len(hosts) * [str(command_group)]), (len(hosts) * [config["operation"]]), (len(hosts) * [ignore_error])))

    with multiprocessing.Pool(processes=max_concurrency) as pool:
        pool.map(run, work_items)

main()
