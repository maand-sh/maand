import functools
import glob
import json
import logging
import os


def get_agents(role_filter=None):
    with open("/workspace/agents.json", "r") as f:
        data = json.loads(f.read())

    agents = {item.get("host"): item for item in data}

    for agent_ip in agents:
        agent = agents[agent_ip]
        if not agent.get("roles"):
            agent["roles"] = ["agent"]
        else:
            agent["roles"].append("agent")

        if not agent.get("tags"):
            agent["tags"] = {}

    if role_filter:
        agents = {agent_ip: agent for agent_ip, agent in agents.items() if
                  set(role_filter) & set(agent.get("roles", []))}

    return agents


def get_agent_and_roles(role_filter=None):
    agents = get_agents(role_filter)
    for agent_ip, agent in agents.items():
        agents[agent_ip] = agent.get("roles")
    return agents


def get_agent_and_tags(role_filter=None):
    agents = get_agents(role_filter)
    for host, agent in agents.items():
        agents[host] = agent.get("tags")
    return agents


def get_job_metadata(job_folder_name, base_path="/workspace/jobs/"):
    metadata_path = os.path.join(base_path, job_folder_name, "manifest.json")
    if os.path.exists(metadata_path):
        with open(metadata_path, "r") as f:
            metadata = json.load(f)
            if "roles" not in metadata:
                metadata["roles"] = []
            return metadata
    return {"roles": []}


def get_role_and_jobs():
    roles = {}
    for metadata_path in glob.glob("/workspace/jobs/*/manifest.json"):

        job_folder_name = os.path.basename(os.path.dirname(metadata_path))
        metadata = get_job_metadata(job_folder_name)

        for role in metadata["roles"]:
            if role not in roles:
                roles[role] = []
            roles[role].append(job_folder_name)

    return roles


def get_assigned_jobs(agent_ip):
    agents = get_agents()
    roles = agents.get(agent_ip).get("roles")
    role_jobs = get_role_and_jobs()
    assigned_jobs = []
    for role in roles:
        assigned_jobs.extend(role_jobs.get(role, []))
    return list(set(assigned_jobs))


def get_assigned_roles(agent_ip):
    agents = get_agents()
    roles = agents.get(agent_ip).get("roles")
    return list(set(roles))


@functools.cache
def get_logger():
    root_logger = logging.getLogger(os.getenv("AGENT_IP"))
    console_handler = logging.StreamHandler()
    root_logger.addHandler(console_handler)
    log_level = os.getenv("LOG_LEVEL", "INFO").upper()
    root_logger.setLevel(log_level)
    return root_logger
