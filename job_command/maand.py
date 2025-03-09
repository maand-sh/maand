# Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
# Use of this source code is governed by a MIT style
# license that can be found in the LICENSE file.

import os
import requests

def get_allocation_id():
    return os.environ.get("ALLOCATION_ID")

def get_allocation_ip():
    return os.environ.get("ALLOCATION_IP")

def get_allocation_disabled():
    return os.environ.get("DISABLED")

def get_event():
    return os.environ.get("EVENT")

def get_command():
    return os.environ.get("command")

def get_job():
    return os.environ.get("JOB")

def kv_get(namespace, key):
    return requests.get(f"http://host.docker.internal:8080/kv", json={"namespace":namespace, "key": key},
                        headers={"X-ALLOCATION-ID": get_allocation_id(), "COMMAND": get_command(), "EVENT": get_event()})

def kv_put(key, value):
    return requests.put(f"http://host.docker.internal:8080/kv", json={"namespace": f"vars/job/{get_job()}", "key": key, "value": value},
                        headers={"X-ALLOCATION-ID": get_allocation_id(), "COMMAND": get_command(), "EVENT": get_event()})

def demands():
    return requests.get(f"http://host.docker.internal:8080/demands",
                        headers={"X-ALLOCATION-ID": get_allocation_id(), "COMMAND": get_command(), "EVENT": get_event()})