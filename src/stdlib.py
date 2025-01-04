import sys

sys.path.append("/maand")

import json
import os
import maand_data
import kv_manager
import command_manager

def get_db():
    return maand_data.get_db()


def get_demands():
    job = os.environ.get("JOB")
    with open(f"/modules/{job}/_modules/demands.json") as f:
        return json.load(f)


def get_kv_manager():
    return kv_manager

def get_command_manager():
    return command_manager