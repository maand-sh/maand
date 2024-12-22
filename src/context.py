import sys
sys.path.append("/scripts")

import json
import os
import maand
import kv_manager

def get_db():
    return maand.get_db()

def get_demands():
    job = os.environ.get("JOB")
    with open(f"/modules/{job}/_modules/demands.json") as f:
        return json.load(f)

def get_kv_manager():
    return kv_manager
