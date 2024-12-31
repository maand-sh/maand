import base64
import hashlib
import os
from copy import deepcopy
from pathlib import Path
from string import Template

import command_helper
import const
import context_manager
import kv_manager
import maand_data
import job_data
import utils

logger = utils.get_logger()


def write_cert(cursor, location, namespace, kv_path):
    content = kv_manager.get(cursor, namespace, kv_path)
    if content:
        content = base64.b64decode(content)
        with open(location, "wb") as f:
            f.write(content)


def update_certificates(cursor, job, agent_ip):
    agent_dir = context_manager.get_agent_dir(agent_ip)

    job_cert_location = f"{agent_dir}/jobs/{job}/certs"
    job_cert_kv_location = f"{job}/certs"
    namespace = f"certs/job/{agent_ip}"

    job_certs = job_data.get_job_certs_config(cursor, job)

    if job_certs:
        command_helper.command_local(f"mkdir -p {job_cert_location}")
        command_helper.command_local(
            f"cp -f {const.SECRETS_PATH}/ca.crt {job_cert_location}/"
        )

    for cert in job_certs:
        name = cert.get("name")
        job_cert_path = f"{job_cert_location}/{name}"
        job_cert_kv_path = f"{job_cert_kv_location}/{name}"

        write_cert(cursor, f"{job_cert_path}.key", namespace, f"{job_cert_kv_path}.key")
        write_cert(cursor, f"{job_cert_path}.crt", namespace, f"{job_cert_kv_path}.crt")
        if cert.get("pkcs8", False):
            write_cert(cursor, f"{job_cert_path}.pem", namespace, f"{job_cert_kv_path}.pem")


def process_templates(cursor, values, job):
    values = deepcopy(values)
    for k, v in values.items():
        values[k] = v.replace("$$", "$")
    agent_ip = values["AGENT_IP"]
    agent_dir = context_manager.get_agent_dir(agent_ip)
    logger.debug("Processing templates...")
    for ext in ["*.json", "*.service", "*.conf", "*.yml", "*.yaml", "*.env", "*.txt"]:
        values = deepcopy(values)

        job_namespace = f"vars/job/{job}"
        job_keys = kv_manager.get_keys(cursor, job_namespace)
        for key in job_keys:
            values[key] = kv_manager.get(cursor, job_namespace, key)

        for f in Path(f"{agent_dir}/jobs/{job}").rglob(ext):
            try:
                with open(f, "r") as file:
                    data = file.read()
                template = Template(data)
                content = template.substitute(values)
                if content != data:
                    with open(f, "w") as file:
                        file.write(content)
                logger.debug(f"Processed template: {f}")
            except Exception as e:
                logger.error(f"Error processing file {f}: {e}")
                raise e


def transpile(cursor, agent_ip, job):
    logger.debug("Transpiling templates...")
    values = context_manager.get_agent_env(cursor, agent_ip)
    process_templates(cursor, values, job)


def calculate_file_md5(file_path):
    """Calculate the MD5 checksum of a file."""
    hash_md5 = hashlib.md5()
    try:
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(4096), b""):
                hash_md5.update(chunk)
    except Exception as e:
        print(f"Error reading file {file_path}: {e}")
    return hash_md5.hexdigest()


def calculate_dir_md5(folder_path):
    """Calculate a combined MD5 checksum for all files in a folder."""
    if not os.path.exists(folder_path):
        return None
    hash_md5 = hashlib.md5()
    for root, _, files in os.walk(folder_path):
        for file in sorted(files):  # Sort to ensure consistent order
            file_path = os.path.join(root, file)
            if os.path.isfile(file_path):
                file_md5 = calculate_file_md5(file_path)
                hash_md5.update(file_md5.encode())  # Update with file checksum
    return hash_md5.hexdigest()


def update_allocation(job, allocation_ip):
    with maand_data.get_db() as db:
        cursor = db.cursor()

        agent_dir = context_manager.get_agent_dir(allocation_ip)
        agent_jobs = maand_data.get_agent_jobs(cursor, allocation_ip)
        if job in agent_jobs:
            command_helper.command_local(f"mkdir -p {agent_dir}/jobs/")
            maand_data.copy_job(cursor, job, agent_dir)

        transpile(cursor, allocation_ip, job)
        update_certificates(cursor, job, allocation_ip)

        md5_hash = calculate_dir_md5(f"{agent_dir}/jobs/{job}")
        cursor.execute(
            "UPDATE agent_jobs SET current_md5_hash = ?, previous_md5_hash = current_md5_hash WHERE job = ? AND agent_id = (SELECT agent_id FROM agent WHERE agent_ip = ?)",
            (md5_hash, job, allocation_ip,))
