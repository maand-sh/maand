# Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
# Use of this source code is governed by a MIT style
# license that can be found in the LICENSE file.

"""Client for the maand command runtime API (KV store, demands, worker SSH)."""

import os
import subprocess
from pathlib import Path

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


def put_deploy_order(order):
    """PUT deploy_order for the current job (pre_deploy or cli only).

    order: comma-separated worker IPs, or a list/tuple of IPs.
    """
    if isinstance(order, (list, tuple)):
        order = ",".join(str(ip).strip() for ip in order if str(ip).strip())
    return requests.put(
        f"{_runtime_api_base_url()}{_ROUTE_STORE_KEYS}",
        json={
            "namespace": f"maand/job/{job_name()}",
            "key": "deploy_order",
            "value": order,
        },
        headers=_runtime_request_headers(),
    )


def get_deploy_order():
    """GET deploy_order for the current job."""
    return get_store_value(f"maand/job/{job_name()}", "deploy_order")


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


def get_kv_value(namespace, key):
    """Read a KV value as a plaintext string."""
    response = get_store_value(namespace, key)
    response.raise_for_status()
    return response.json()["value"]


def find_bucket_root():
    """Locate the bucket root (directory containing maand.conf)."""
    for start in (Path.cwd().resolve(), Path(__file__).resolve().parent):
        for directory in [start, *start.parents]:
            if (directory / "maand.conf").is_file():
                return directory
    raise FileNotFoundError(f"maand.conf not found (cwd={Path.cwd()})")


def load_ssh():
    """Return (ssh_user, key_path, use_sudo) from maand.conf and secrets/."""
    bucket = find_bucket_root()
    conf = {}
    conf_path = bucket / "maand.conf"
    if conf_path.is_file():
        for line in conf_path.read_text().splitlines():
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, value = line.split("=", 1)
            conf[key.strip()] = value.strip().strip("'\"")
    user = conf.get("ssh_user", "root")
    key_name = conf.get("ssh_key", "worker.key")
    use_sudo = conf.get("use_sudo", "false").lower() in ("true", "1", "yes")
    key = bucket / "secrets" / key_name
    if not key.is_file():
        fallback = bucket / key_name
        if fallback.is_file():
            key = fallback
        else:
            raise FileNotFoundError(
                f"SSH key not found: {key} (also checked {fallback})"
            )
    return user, key, use_sudo


def run_ssh(
    worker_ip,
    remote_cmd,
    *,
    timeout=300,
    check=True,
    connect_timeout=15,
):
    """Run a remote command over SSH using maand.conf worker credentials."""
    user, key_path, use_sudo = load_ssh()
    prefix = "sudo -E " if use_sudo else ""
    wrapped = f"{prefix}timeout {timeout} {remote_cmd}"
    return subprocess.run(
        [
            "ssh",
            "-i",
            str(key_path),
            "-o",
            "BatchMode=yes",
            "-o",
            f"ConnectTimeout={connect_timeout}",
            "-o",
            "StrictHostKeyChecking=accept-new",
            f"{user}@{worker_ip}",
            wrapped,
        ],
        check=check,
        text=True,
        capture_output=True,
        timeout=timeout + connect_timeout + 30,
    )


def run_runner_target(
    target,
    *,
    worker_ip=None,
    job=None,
    timeout=300,
):
    """Run runner.py <target> --jobs <job> on a worker (same as deploy)."""
    worker_ip = worker_ip or allocation_ip()
    job = job or job_name()
    bucket_id = get_kv_value("maand", "bucket_id")
    remote = (
        f"python3 /opt/worker/{bucket_id}/bin/runner.py "
        f"{bucket_id} {target} --jobs {job}"
    )
    return run_ssh(worker_ip, remote, timeout=timeout)


def run_make_target(
    target,
    *,
    worker_ip=None,
    job=None,
    timeout=300,
):
    """Run make <target> in the job directory on a worker."""
    worker_ip = worker_ip or allocation_ip()
    job = job or job_name()
    bucket_id = get_kv_value("maand", "bucket_id")
    remote = f"make -C /opt/worker/{bucket_id}/jobs/{job} {target}"
    return run_ssh(worker_ip, remote, timeout=timeout)
