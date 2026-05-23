# Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
# Use of this source code is governed by a MIT style
# license that can be found in the LICENSE file.

"""Client for the maand command runtime API (KV store and command demands)."""

import os

import requests

_RUNTIME_API_PORT = 8080
_ROUTE_STORE_KEYS = "/kv"
_ROUTE_STORE_KEYS_LIST = "/kv/keys"
_ROUTE_STORE_SECRET = "/kv/secret"
_ROUTE_DEMANDS = "/demands"
_ROUTE_SEMAPHORE_ACQUIRE = "/semaphore/acquire"
_ROUTE_SEMAPHORE_RELEASE = "/semaphore/release"
_ROUTE_SEMAPHORE_STATUS = "/semaphore/status"


def allocation_id():
    return os.environ.get("ALLOCATION_ID")


def allocation_ip():
    return os.environ.get("ALLOCATION_IP")


def is_allocation_disabled():
    return os.environ.get("DISABLED") == "1"


def command_event():
    return os.environ.get("EVENT")


def command_name():
    return os.environ.get("COMMAND")


def job_name():
    return os.environ.get("JOB")


def _runtime_api_base_url():
    host = os.environ.get("JOB_COMMAND_API_HOST", "0.0.0.0")
    return f"http://{host}:{_RUNTIME_API_PORT}"


def _runtime_request_headers():
    return {
        "X-ALLOCATION-ID": allocation_id(),
        "COMMAND": command_name(),
        "EVENT": command_event(),
    }


def get_store_value(namespace, key):
    """GET /kv — read a key from an allowed namespace."""
    return requests.get(
        f"{_runtime_api_base_url()}{_ROUTE_STORE_KEYS}",
        json={"namespace": namespace, "key": key},
        headers=_runtime_request_headers(),
    )


def put_job_variable(key, value):
    """PUT /kv — write a key under vars/job/<current job>."""
    return requests.put(
        f"{_runtime_api_base_url()}{_ROUTE_STORE_KEYS}",
        json={
            "namespace": f"vars/job/{job_name()}",
            "key": key,
            "value": value,
        },
        headers=_runtime_request_headers(),
    )


def list_job_keys(namespace=None):
    """GET /kv/keys — list keys under vars/job/<job> and secrets/job/<job>."""
    body = {}
    if namespace is not None:
        body["namespace"] = namespace
    return requests.get(
        f"{_runtime_api_base_url()}{_ROUTE_STORE_KEYS_LIST}",
        json=body,
        headers=_runtime_request_headers(),
    )


def delete_job_variable(key):
    """DELETE /kv — remove a key under vars/job/<current job>."""
    return requests.delete(
        f"{_runtime_api_base_url()}{_ROUTE_STORE_KEYS}",
        json={
            "namespace": f"vars/job/{job_name()}",
            "key": key,
        },
        headers=_runtime_request_headers(),
    )


def delete_job_secret(key):
    """DELETE /kv/secret — remove an encrypted key under secrets/job/<current job>."""
    return requests.delete(
        f"{_runtime_api_base_url()}{_ROUTE_STORE_SECRET}",
        json={
            "namespace": f"secrets/job/{job_name()}",
            "key": key,
        },
        headers=_runtime_request_headers(),
    )


def put_job_secret(key, value):
    """PUT /kv/secret — write an encrypted key under secrets/job/<current job>."""
    return requests.put(
        f"{_runtime_api_base_url()}{_ROUTE_STORE_SECRET}",
        json={
            "namespace": f"secrets/job/{job_name()}",
            "key": key,
            "value": value,
        },
        headers=_runtime_request_headers(),
    )


def list_command_demands():
    """GET /demands — list job commands that depend on this command."""
    return requests.get(
        f"{_runtime_api_base_url()}{_ROUTE_DEMANDS}",
        headers=_runtime_request_headers(),
    )


def acquire_semaphore(name, capacity=1, timeout_seconds=600):
    """POST /semaphore/acquire — block until this allocation holds a slot.

    Use capacity=1 for a leader/mutex (e.g. deploy one allocation before the rest).
    Scoped per job and command event (pre_deploy, post_deploy, etc.).
    """
    return requests.post(
        f"{_runtime_api_base_url()}{_ROUTE_SEMAPHORE_ACQUIRE}",
        json={
            "name": name,
            "capacity": capacity,
            "timeout_seconds": timeout_seconds,
        },
        headers=_runtime_request_headers(),
        timeout=timeout_seconds + 30,
    )


def release_semaphore(name):
    """POST /semaphore/release — release a slot held by this allocation."""
    return requests.post(
        f"{_runtime_api_base_url()}{_ROUTE_SEMAPHORE_RELEASE}",
        json={"name": name},
        headers=_runtime_request_headers(),
    )


def semaphore_status(name):
    """GET /semaphore/status — inspect holders and waiters for a named semaphore."""
    return requests.get(
        f"{_runtime_api_base_url()}{_ROUTE_SEMAPHORE_STATUS}",
        params={"name": name},
        headers=_runtime_request_headers(),
    )


# Backward-compatible aliases for older job command scripts.
def get_allocation_id():
    return allocation_id()


def get_allocation_ip():
    return allocation_ip()


def get_event():
    return command_event()


def get_command():
    return command_name()


def get_job():
    return job_name()


def kv_get(namespace, key):
    return get_store_value(namespace, key)


def kv_put(key, value):
    return put_job_variable(key, value)


def kv_put_secret(key, value):
    return put_job_secret(key, value)


def get_demands():
    return list_command_demands()
