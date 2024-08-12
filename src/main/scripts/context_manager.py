import os
import sys
import difflib

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


def load_secrets(values):
    secrets = dotenv_values("/workspace/secrets.env")
    for key, value in secrets.items():
        values[key] = value
    return values


def _add_roles_to_values(values, agent_ip):
    available_roles = set()
    agent_roles = utils.get_agent_and_roles()

    for ip, roles in agent_roles.items():
        available_roles.update(roles)

    available_roles = set(available_roles)

    for role in available_roles:
        key_nodes = f"{role}_NODES".upper()
        agent_roles = utils.get_agents([role])
        hosts_ip = list(agent_roles.keys())
        values[key_nodes] = ",".join(hosts_ip)

        other_agents = list(agent_roles.keys())
        if agent_ip in other_agents:
            other_agents.remove(agent_ip)
        key_others = f"{role}_OTHERS".upper()
        values[key_others] = ",".join(other_agents)

        for idx, host in enumerate(list(agent_roles.keys())):
            key = f"{role}_{idx}".upper()
            values[key] = host

            if host == agent_ip:
                key = f"{role}_ALLOCATION_INDEX".upper()
                values[key] = idx

        key = f"{role}_LENGTH".upper()
        values[key] = len(agent_roles.keys())

    agent_roles = utils.get_agent_and_roles()
    values["ROLES"] = ",".join(sorted(agent_roles.get(agent_ip)))

    return values


def _add_tags_to_values(values, node_ip):
    agents = utils.get_agent_and_tags()
    tags = agents.get(node_ip, {})
    for k, v in tags.items():
        key = f"{k}".upper()
        values[key] = v
    return values


def get_values():
    agent_id = get_agent_id()
    agent_ip = os.getenv("AGENT_IP")

    values = dotenv_values("/workspace/variables.env")

    values["CLUSTER_ID"] = get_cluster_id()
    values["AGENT_ID"] = agent_id
    values["AGENT_IP"] = agent_ip

    values = _add_roles_to_values(values, agent_ip)
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
        with open("/opt/agent/cluster_id.txt", "r", encoding='utf-8') as f:
            data = f.read().strip().casefold()
            if data != cluster_id.strip():
                raise Exception("Failed on cluster id validation: mismatch")
