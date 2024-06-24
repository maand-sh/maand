import json
import os
import sys
from pathlib import Path

import command_helper
import utils
import variables


def get_host_id():
    with open("/opt/agent/node.txt", "r") as f:
        return f.read().strip()


def transpile():
    host_id = get_host_id()

    interface_name = variables.get_network_interface_name()
    cluster_id = variables.get_cluster_id()

    values = {
        "$NODE_NAME": host_id,
        "$INTERFACE_NAME": interface_name,
        "$CLUSTER_ID": cluster_id
    }

    for ext in ["*.json", "*.service", "*.conf", "*.yml", "*.env", "*.token"]:
        for f in Path('/opt/agent/').rglob(ext):
            with open(f, 'r') as file:
                data = file.read()

            content = data

            for k, v in values.items():
                content = content.replace(k, v)

            if content != data:
                with open(f, 'w') as file:
                    file.write(content)


def sync():
    cluster_id = variables.get_cluster_id()
    host = os.getenv("HOST")

    command_helper.command_local("""
        bash /scripts/rsync_remote_local.sh
    """)

    if not os.path.isfile("/opt/agent/node.txt"):
        print("/opt/agent/node.txt is not found")
        sys.exit(1)

    with open(f"/opt/agent/cluster.txt", "w") as f:
        f.write(f"{cluster_id}")

    nodes = utils.get_host_and_roles()
    roles = nodes.get(host, [])
    with open("/opt/agent/roles.txt", "w") as f:
        f.writelines("\n".join(roles))

    command_helper.command_local("""
        touch /opt/agent/profile
        rsync -r /agent/bin /opt/agent/        
    """)

    transpile()

    command_helper.command_local("""
        bash /scripts/rsync_local_remote.sh
    """)


if __name__ == "__main__":
    sync()
