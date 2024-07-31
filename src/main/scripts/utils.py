import functools
import glob
import json
import logging
import os


def get_agents(role_filter=None):
    with open("/workspace/agents.json", "r") as f:
        data = json.loads(f.read())

    agents = {item.get("host"): item for item in data}

    if role_filter:
        agents = {agent_ip: agent for agent_ip, agent in agents.items() if set(role_filter) & set(agent.get("roles", []))}

    return agents


def get_agent_and_roles(role_filter=None):
    agents = get_agents(role_filter)
    for agent_ip, agent in agents.items():
        agents[agent_ip] = agent.get("roles", [])
    return agents


def get_agent_and_tags(role_filter=None):
    agents = get_agents(role_filter)
    for host, agent in agents.items():
        agents[host] = agent.get("tags", {})
    return agents


def get_agents_by_role(role):
    hosts = get_agent_and_roles()
    role_hosts = [ip for ip, roles in hosts.items() if role in roles]
    return role_hosts


def get_job_metadata(job_folder_name, base_path="/workspace/jobs/"):
    for metadata_path in glob.glob(f"{base_path}/{job_folder_name}/manifest.json"):
        if os.path.isfile(metadata_path):
            with open(metadata_path, "r") as f:
                metadata = json.load(f)
                return metadata
    return {}


@functools.cache
def get_role_and_jobs():
    roles = {}
    for metadata_path in glob.glob("/workspace/jobs/*/manifest.json"):
        if not os.path.isfile(metadata_path):
            continue

        with open(metadata_path, "r") as f:
            metadata = json.load(f)
            if "roles" not in metadata:
                continue

        for role in metadata["roles"]:
            if role not in roles:
                roles[role] = []
            job_folder_name = os.path.basename(os.path.dirname(metadata_path))
            roles[role].append(job_folder_name)

    return roles


@functools.cache
def get_logger():
    logging.basicConfig(level=logging.INFO)
    return logging.getLogger(__name__)
