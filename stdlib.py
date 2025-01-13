import json
import os
import sys

sys.path.append("/maand")

import kv_manager as internal_kv_manager
from core import maand_data as __maand_data, command_manager as __command_manager
from core import context_manager


def get_demands():
    job = os.environ.get("JOB")
    with open(f"/modules/{job}/_modules/demands.json") as f:
        return json.load(f)


def get_db():
    return __maand_data.get_db()


def kv_get(cursor, namespace, key):
    return internal_kv_manager.get(cursor, namespace, key)


def kv_get_metadata(cursor, namespace, key):
    return internal_kv_manager.get_metadata(cursor, namespace, key)


def kv_put(cursor, namespace, key, value):
    job = os.environ.get("JOB")
    assert key.lower() == key
    assert namespace.lower() == namespace
    assert namespace == f"job/{job}"
    return internal_kv_manager.put(cursor, namespace, key, value)


def kv_delete(cursor, namespace, key):
    return internal_kv_manager.delete(cursor, namespace, key)


def execute_shell_command(command, agent_ip=None):
    host = agent_ip or os.environ.get("AGENT_IP")
    agent_env = context_manager.get_agent_minimal_env(host)
    return __command_manager.command_remote(command, env=agent_env)
