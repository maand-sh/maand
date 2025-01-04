import copy
import os.path
import uuid

from dotenv import dotenv_values

import const
import kv_manager
import maand_data


def build_env(cursor, path):
    namespace = os.path.basename(path)
    key_values = dotenv_values(path)

    for key, value in key_values.items():
        key = key.upper()
        kv_manager.put(cursor, namespace, key, value)

    all_keys = kv_manager.get_keys(cursor, namespace)
    missing_keys = list(set(all_keys) ^ set(key_values.keys()))
    for key in missing_keys:
        kv_manager.delete(cursor, namespace, key)


def build_agent_variables(cursor):
    agents = maand_data.get_agents(cursor, labels_filter=None)

    for agent_ip in agents:
        labels = maand_data.get_agent_labels(cursor, agent_ip=None)
        agent_labels = maand_data.get_agent_labels(cursor, agent_ip=agent_ip)

        values = {}
        for label in labels:
            key_nodes = f"{label}_nodes".upper()

            agents = maand_data.get_agents(cursor, [label])
            values[key_nodes] = ",".join(agents)

            key = f"{label}_length".upper()
            values[key] = str(len(agents))

            for idx, host in enumerate(agents):
                key = f"{label}_{idx}".upper()
                values[key] = host

            if label not in agent_labels:
                continue

            other_agents = copy.deepcopy(agents)
            if agent_ip in other_agents:
                other_agents.remove(agent_ip)

            key_peers = f"{label}_peers".upper()
            if other_agents:
                values[key_peers] = ",".join(other_agents)

            for idx, host in enumerate(agents):
                if host == agent_ip:
                    key = f"{label}_allocation_index".upper()
                    values[key] = str(idx)

            key = f"{label}_label_id".upper()
            values[key] = str(uuid.uuid5(uuid.NAMESPACE_DNS, str(label)))

        values["LABELS"] = ",".join(sorted(agent_labels))

        agent_tags = maand_data.get_agent_tags(cursor, agent_ip)
        for key, value in agent_tags.items():
            values[key] = value

        agent_memory, agent_cpu = maand_data.get_agent_available_resources(cursor, agent_ip)
        if agent_memory != "0.0":
            values["AGENT_MEMORY"] = agent_memory
        if agent_cpu != "0.0":
            values["AGENT_CPU"] = agent_cpu

        namespace = f"vars/agent/{agent_ip}"
        for key, value in values.items():
            kv_manager.put(cursor, namespace, key, str(value))

        all_keys = kv_manager.get_keys(cursor, namespace)
        missing_keys = list(set(all_keys) ^ set(values.keys()))
        for key in missing_keys:
            kv_manager.delete(cursor, namespace, key)


def build(cursor):
    build_env(cursor, f"{const.WORKSPACE_PATH}/variables.env")
    build_agent_variables(cursor)
