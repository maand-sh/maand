import uuid
import utils
import workspace
import kv_manager
import maand
import jsonschema
from jsonschema import Draft202012Validator

logger = utils.get_logger()


def build_agent_tags(cursor, agent_id, agent_ip, agent):
    cursor.execute("DELETE FROM agent_tags WHERE agent_id = ?", (agent_id,))
    tags = agent.get("tags", {})
    for key, value in tags.items():
        key = key.upper()
        value = str(value)
        cursor.execute("INSERT INTO agent_tags (agent_id, key, value) VALUES (?, ?, ?)", (agent_id, key, value,))

    namespace = f"vars/{agent_ip}"
    available_memory, available_cpu = maand.get_agent_available_resources(cursor, agent_ip)
    if available_memory != "0.0":
        kv_manager.put(cursor, namespace, "AGENT_MEMORY", available_memory)
    if available_cpu != "0.0":
        kv_manager.put(cursor, namespace, "AGENT_CPU", available_cpu)


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

    jsonschema.validate(instance=agents, schema=schema, format_checker=Draft202012Validator.FORMAT_CHECKER,)

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
            cursor.execute("UPDATE agent SET agent_memory_mb = ?, agent_cpu = ?, position = ?, detained = 0 WHERE agent_id = ?", (agent_memory, agent_cpu, index, agent_id, ))
        else:
            cursor.execute("INSERT INTO agent (agent_id, agent_ip, agent_memory_mb, agent_cpu, detained, position) VALUES (?, ?, ?, ?, 0, ?)", (agent_id, agent_ip, agent_memory, agent_cpu, index,))

        cursor.execute("DELETE FROM agent_labels WHERE agent_id = ?", (agent_id,))
        labels = agent.get("labels", [])
        labels.append("agent")
        labels = list(set(labels))
        for label in labels:
            cursor.execute("INSERT INTO agent_labels (agent_id, label) VALUES (?, ?)", (agent_id, label,))

        build_agent_tags(cursor, agent_id, agent_ip, agent)

    cursor.execute("SELECT agent_ip FROM agent")
    rows = cursor.fetchall()
    host_ips = [agent["host"] for agent in agents]
    agent_ips = [row[0] for row in rows]

    missing_agents = list(set(agent_ips) - set(host_ips))
    for agent_ip in missing_agents:
        cursor.execute("UPDATE agent SET detained = 1 WHERE agent_ip = ?", (agent_ip,))

    cursor.execute("SELECT agent_ip FROM agent WHERE detained = 1")
    rows = cursor.fetchall()
    agents_ip = {row[0] for row in rows}

    for agent_ip in agents_ip:
        for namespace in [ f"certs/{agent_ip}", f"vars/{agent_ip}"]:
            keys = kv_manager.get_keys(cursor, namespace)
            for key in keys:
                kv_manager.delete(cursor, namespace, key)


def build(cursor):
    build_agents(cursor)
