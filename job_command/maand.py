import os
import requests

def get_allocation_id():
    return os.environ.get("ALLOCATION_ID")

def get_allocation_ip():
    return os.environ.get("ALLOCATION_IP")

def get_allocation_index():
    return os.environ.get("ALLOCATION_INDEX")

def get_allocation_disabled():
    return os.environ.get("ALLOCATION_DISABLED")

def get_event():
    return os.environ.get("EVENT")

def get_command():
    return os.environ.get("command")

def get_job():
    return os.environ.get("JOB")

def kv_get(namespace, key):
    return requests.get(f"http://localhost:8080/kv", json={"namespace":namespace, "key": key}, headers={"X-ALLOCATION-ID": get_allocation_id()})

def kv_put(key, value):
    return requests.put(f"http://localhost:8080/kv", json={"namespace": f"vars/job/{get_job()}", "key": key, "value": value}, headers={"X-ALLOCATION-ID": get_allocation_id()})

def demands():
    return requests.get(f"http://localhost:8080/demands", headers={"X-ALLOCATION-ID": get_allocation_id()})