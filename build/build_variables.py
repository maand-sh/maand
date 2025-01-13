import copy
import uuid

from dotenv import dotenv_values

import kv_manager
from core import const, maand_data


def build_env(cursor, path):
    namespace = "maand"
    key_values = dotenv_values(path)

    for key, value in key_values.items():
        key = key.lower()
        kv_manager.put(cursor, namespace, key, value)

    available_keys = [k.lower() for k in key_values.keys()]
    all_keys = kv_manager.get_keys(cursor, namespace)
    missing_keys = list(set(all_keys) ^ set(available_keys))
    for key in missing_keys:
        kv_manager.delete(cursor, namespace, key)


def build_agent_variables(cursor):
    agents = maand_data.get_agents(cursor, labels_filter=None)

    for agent_ip in agents:
        labels = maand_data.get_agent_labels(cursor, agent_ip=None)
        agent_labels = maand_data.get_agent_labels(cursor, agent_ip=agent_ip)

        values = {}
        for label in labels:
            key_nodes = f"{label}_nodes".lower()

            agents = maand_data.get_agents(cursor, [label])
            values[key_nodes] = ",".join(agents)

            key = f"{label}_length".lower()
            values[key] = str(len(agents))

            for idx, host in enumerate(agents):
                key = f"{label}_{idx}".lower()
                values[key] = host

            if label not in agent_labels:
                continue

            other_agents = copy.deepcopy(agents)
            if agent_ip in other_agents:
                other_agents.remove(agent_ip)

            key_peers = f"{label}_peers".lower()
            if other_agents:
                values[key_peers] = ",".join(other_agents)

            for idx, host in enumerate(agents):
                if host == agent_ip:
                    key = f"{label}_allocation_index".lower()
                    values[key] = str(idx)

            key = f"{label}_label_id".lower()
            values[key] = str(uuid.uuid5(uuid.NAMESPACE_DNS, str(label)))

        values["labels"] = ",".join(sorted(agent_labels))

        agent_tags = maand_data.get_agent_tags(cursor, agent_ip)
        for key, value in agent_tags.items():
            values[key] = value

        agent_memory, agent_cpu = maand_data.get_agent_available_resources(cursor, agent_ip)
        if agent_memory != "0.0":
            values["agent_memory"] = agent_memory
        if agent_cpu != "0.0":
            values["agent_cpu"] = agent_cpu

        namespace = f"maand/agent/{agent_ip}"
        for key, value in values.items():
            kv_manager.put(cursor, namespace, key, str(value))

        all_keys = kv_manager.get_keys(cursor, namespace)
        missing_keys = list(set(all_keys) ^ set(values.keys()))
        for key in missing_keys:
            kv_manager.delete(cursor, namespace, key)


def build(cursor):
    build_env(cursor, f"{const.WORKSPACE_PATH}/maand.vars")
    build_agent_variables(cursor)
