import os
import sys

from dotenv import dotenv_values

import command_helper
import utils

logger = utils.get_logger()


def get_agent_id():
    try:
        with open("/opt/agent/agent_id.txt", "r") as f:
            return f.read().strip()
    except FileNotFoundError:
        logger.error("agent_id.txt not found.")
        sys.exit(1)


def get_cluster_id():
    try:
        with open("/opt/agent/cluster_id.txt", "r") as f:
            return f.read().strip()
    except FileNotFoundError:
        logger.error("agent_id.txt not found.")
        sys.exit(1)


def _load_values():
    values = dotenv_values("/workspace/variables.env")

    agent_id = get_agent_id()
    agent_ip = os.getenv("AGENT_IP")

    values["AGENT_ID"] = agent_id
    values["AGENT_IP"] = agent_ip

    return values, agent_ip


def _add_roles_to_values(values):
    agent_ip = values["AGENT_IP"]
    available_roles = set()
    agents = utils.get_agent_and_roles()

    for ip, roles in agents.items():
        available_roles.update(roles)

    available_roles = set(available_roles)

    for role in available_roles:
        key_nodes = f"{role}_NODES".upper()
        role_hosts = utils.get_agents_by_role(role)
        values[key_nodes] = ",".join(role_hosts)

        if agent_ip in role_hosts:
            role_hosts.remove(agent_ip)

        key_others = f"{role}_OTHERS".upper()
        values[key_others] = ",".join(role_hosts)

        for idx, host in enumerate(utils.get_agents_by_role(role)):
            key = f"{role}_{idx}".upper()
            values[key] = host

            if host == agent_ip:
                key = f"{role}_ALLOCATION_INDEX".upper()
                values[key] = idx

    host_roles = utils.get_agent_and_roles()
    values["ROLES"] = ",".join(host_roles.get(agent_ip))

    return values


def _add_tags_to_values(values, node_ip):
    agents = utils.get_agent_and_tags()
    tags = agents.get(node_ip, {})
    for k, v in tags.items():
        key = f"{k}".upper()
        values[key] = v
    return values


def get_values():
    values, agent_ip = _load_values()
    values = _add_roles_to_values(values)
    values = _add_tags_to_values(values, agent_ip)
    return values


def validate_cluster_id():
    cluster_id = os.getenv("CLUSTER_ID")

    if not cluster_id:
        logger.error("Required environment variable: CLUSTER_ID is not set.")
        sys.exit(1)

    command_helper.command_local("bash /scripts/rsync_remote_local.sh")
    command_helper.command_local("mkdir -p /opt/agent")

    if os.path.isfile("/opt/agent/cluster_id.txt"):
        with open("/opt/agent/cluster_id.txt", "r") as f:
            if f.read().strip() != cluster_id:
                raise Exception("Failed on cluster id validation: mismatch")
