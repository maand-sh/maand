import os
import sys
from pathlib import Path
from string import Template

import command_helper
import utils
import variables


def get_host_id():
    with open("/opt/agent/node.txt", "r") as f:
        return f.read().strip()


def transpile():
    host_id = get_host_id()
    node_ip = os.getenv("HOST")

    interface_name = variables.get_network_interface_name()
    cluster_id = variables.get_cluster_id()

    values = {
        "NODE_NAME": host_id,
        "INTERFACE_NAME": interface_name,
        "CLUSTER_ID": cluster_id,
        "NODE_IP": node_ip,
    }

    available_roles = set()
    nodes = utils.get_host_roles()
    for ip in nodes:
        roles = nodes.get(ip, [])
        for role in roles:
            available_roles.add(role)

    for role in available_roles:
        key = f"{role}_NODES".upper()
        values[key] = ",".join(utils.get_host_list(role))

        for idx, host in enumerate(utils.get_host_list(role)):
            key = f"{role}_{idx}".upper()
            values[key] = host

    values["ROLES"] = ",".join(available_roles)

    nodes = utils.get_host_tags()
    tags = nodes.get(node_ip)
    for k, v in tags.items():
        key = f"{k}".upper()
        values[key] = v

    for ext in ["*.json", "*.service", "*.conf", "*.yml", "*.env", "*.token"]:
        for f in Path('/opt/agent/').rglob(ext):
            with open(f, 'r') as file:
                data = file.read()

            template = Template(data)
            content = template.substitute(values)

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

    nodes = utils.get_host_roles()
    roles = nodes.get(host, [])
    with open("/opt/agent/roles.txt", "w") as f:
        f.writelines("\n".join(roles))

    assigned_jobs = []
    jobs = utils.get_job_roles()

    for job, job_roles in jobs.items():
        if set(roles) & set(job_roles):
            assigned_jobs.append(job)

    assigned_jobs = ",".join(assigned_jobs)

    command_helper.command_local(f"rsync -r /agent/bin /opt/agent/")
    if assigned_jobs:
        command_helper.command_local(f"rsync -r /workspace/jobs/{assigned_jobs} /opt/agent/jobs/")

    transpile()

    command_helper.command_local("""
        bash /scripts/rsync_local_remote.sh
    """)


if __name__ == "__main__":
    sync()
