import uuid

import jsonschema
from jsonschema import Draft202012Validator

from core import maand_data, utils, workspace
import kv_manager

logger = utils.get_logger()


def build_agent_tags(cursor, agent_id, agent):
    cursor.execute("DELETE FROM agent_tags WHERE agent_id = ?", (agent_id,))
    tags = agent.get("tags", {})
    for key, value in tags.items():
        key = key.lower()
        value = str(value)
        cursor.execute("INSERT INTO agent_tags (agent_id, key, value) VALUES (?, ?, ?)", (agent_id, key, value,))


def build_agent_labels(cursor, agent_id, agent):
    cursor.execute("DELETE FROM agent_labels WHERE agent_id = ?", (agent_id,))
    labels = agent.get("labels", [])
    labels.append("agent")
    labels = list(set(labels))
    for label in labels:
        cursor.execute("INSERT INTO agent_labels (agent_id, label) VALUES (?, ?)", (agent_id, label,))


def build_agent_resources(cursor, agent_ip):
    namespace = f"maand/agent/{agent_ip}"
    available_memory, available_cpu = maand_data.get_agent_available_resources(cursor, agent_ip)
    if available_memory != "0.0":
        kv_manager.put(cursor, namespace, "agent_memory", available_memory)
    if available_cpu != "0.0":
        kv_manager.put(cursor, namespace, "agent_cpu", available_cpu)


def build_agents(cursor):
    agents = workspace.get_agents()

    schema = {
        "type": "array",
        "items": {
            "type": "object",
            "properties": {
                "host": {"type": "string", "format": "ipv4"},
                "labels": {
                    "type": "array",
                    "items": {"type": "string"}
                },
                "cpu": {"type": "string"},
                "memory": {"type": "string"}
            },
            "required": ["host"]
        }
    }

    jsonschema.validate(instance=agents, schema=schema, format_checker=Draft202012Validator.FORMAT_CHECKER, )

    for index, agent in enumerate(agents):
        agent_ip = agent.get("host")
        agent_memory = float(utils.extract_size_in_mb(agent.get("memory", "0 MB")))
        agent_cpu = float(utils.extract_cpu_frequency_in_mhz(agent.get("cpu", "0 MHZ")))

        cursor.execute("SELECT agent_id FROM agent WHERE agent_ip = ?", (agent_ip,))
        row = cursor.fetchone()

        if row:
            agent_id = row[0]
        else:
            agent_id = str(uuid.uuid4())

        if row:
            cursor.execute(
                "UPDATE agent SET agent_memory_mb = ?, agent_cpu = ?, position = ?, detained = 0 WHERE agent_id = ?",
                (agent_memory, agent_cpu, index, agent_id,))
        else:
            cursor.execute(
                "INSERT INTO agent (agent_id, agent_ip, agent_memory_mb, agent_cpu, detained, position) VALUES (?, ?, ?, ?, 0, ?)",
                (agent_id, agent_ip, agent_memory, agent_cpu, index,))

        build_agent_labels(cursor, agent_id, agent)
        build_agent_tags(cursor, agent_id, agent)
        build_agent_resources(cursor, agent_ip)

    cursor.execute("SELECT agent_ip FROM agent")
    rows = cursor.fetchall()
    current_agents = [row[0] for row in rows]

    workspace_agents = [agent["host"] for agent in agents]

    missing_agents = list(set(current_agents) - set(workspace_agents))
    for agent_ip in missing_agents:
        cursor.execute("UPDATE agent SET detained = 1 WHERE agent_ip = ?", (agent_ip,))

    cursor.execute("SELECT agent_ip FROM agent WHERE detained = 1")
    rows = cursor.fetchall()
    detained_agents = {row[0] for row in rows}

    for agent_ip in detained_agents:
        for namespace in [f"maand/certs/agent/{agent_ip}", f"maand/agent/{agent_ip}", f"vars/agent/{agent_ip}"]:
            keys = kv_manager.get_keys(cursor, namespace)
            for key in keys:
                kv_manager.delete(cursor, namespace, key)


def build(cursor):
    build_agents(cursor)
