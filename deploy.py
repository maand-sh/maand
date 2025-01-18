import argparse
import base64
import hashlib
import json
import os
from itertools import chain
from pathlib import Path

import jinja2

import job_control
import kv_manager
from core import command_manager, context_manager, maand_data, system_manager, const, utils

logger = utils.get_logger()


def get_args():
    parser = argparse.ArgumentParser(description="Deploy and manage jobs.")
    parser.add_argument(
        "--jobs", default="", help="Comma-separated list of jobs to process."
    )
    args = parser.parse_args()
    args.jobs = args.jobs.split(",") if args.jobs else []
    return args


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

    job_certs = maand_data.get_job_certs_config(cursor, job)

    if job_certs:
        command_manager.command_local(f"mkdir -p {job_cert_location}")
        command_manager.command_local(
            f"cp -f {const.SECRETS_PATH}/ca.crt {job_cert_location}/"
        )

    for cert in job_certs:
        name = cert.get("name")
        job_cert_path = f"{job_cert_location}/{name}"
        job_cert_kv_path = f"{job_cert_kv_location}/{name}"

        write_cert(cursor, f"{job_cert_path}.key", namespace, f"{job_cert_kv_path}.key")
        write_cert(cursor, f"{job_cert_path}.crt", namespace, f"{job_cert_kv_path}.crt")
        if cert.get("pkcs8", False):
            write_cert(
                cursor, f"{job_cert_path}.pem", namespace, f"{job_cert_kv_path}.pem"
            )


def process_templates(cursor, agent_ip, job):
    agent_dir = context_manager.get_agent_dir(agent_ip)

    values = {}
    for job_namespace in [
        "maand",
        f"vars/job/{job}",
        f"job/{job}",
        f"maand/job/{job}",
        f"maand/agent/{agent_ip}",
    ]:
        job_keys = kv_manager.get_keys(cursor, job_namespace)
        for key in job_keys:
            values[key] = kv_manager.get(cursor, job_namespace, key)

    logger.debug("Processing templates...")
    for ext in ["*.json", "*.service", "*.conf", "*.yml", "*.yaml", "*.env", "*.txt"]:
        for f in Path(f"{agent_dir}/jobs/{job}").rglob(ext):
            try:
                with open(f, "r") as file:
                    data = file.read()
                content = jinja2.Template(
                    data, undefined=jinja2.StrictUndefined
                ).render(values)
                if content != data:
                    with open(f, "w") as file:
                        file.write(content)
                logger.debug(f"Processed template: {f}")
            except Exception as e:
                logger.error(f"Error processing file {f}: {e}")
                raise e


def transpile(cursor, agent_ip, job):
    logger.debug("Transpiling templates...")
    process_templates(cursor, agent_ip, job)


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


def prepare_allocation(cursor, job, allocation_ip):
    agent_dir = context_manager.get_agent_dir(allocation_ip)
    agent_jobs = maand_data.get_agent_jobs(cursor, allocation_ip)
    if job in agent_jobs:
        command_manager.command_local(f"mkdir -p {agent_dir}/jobs/")
        maand_data.copy_job_files(cursor, job, agent_dir)

    transpile(cursor, allocation_ip, job)
    update_certificates(cursor, job, allocation_ip)


def handle_disabled_stopped_allocations(cursor, job):
    counts = maand_data.get_allocation_counts(cursor, job)
    removed_allocations = maand_data.get_removed_allocations(cursor, job)
    disabled_allocations = maand_data.get_disabled_allocations(cursor, job)
    allocations = []
    allocations.extend(removed_allocations)
    allocations.extend(disabled_allocations)

    if counts["removed"] == counts["total"]:  # job removed
        job_control.run_target(
            "stop",
            "deploy",
            job,
            allocations,
            alloc_health_check_flag=False,
            job_health_check_flag=False,
        )
        update_allocation_hash(cursor, job, allocations)
    elif counts["removed"] > 0:  # few allocations removed
        job_control.run_target(
            "stop",
            "deploy",
            job,
            allocations,
            alloc_health_check_flag=True,
            job_health_check_flag=False,
        )
        update_allocation_hash(cursor, job, allocations)


def update_allocation_hash(cursor, job, allocations):
    for agent_ip in allocations:
        agent_dir = context_manager.get_agent_dir(agent_ip)
        md5_hash = calculate_dir_md5(f"{agent_dir}/jobs/{job}")
        cursor.execute(
            "UPDATE agent_jobs SET current_md5_hash = ?, previous_md5_hash = current_md5_hash "
            "WHERE job = ? AND agent_id = (SELECT agent_id FROM agent WHERE agent_ip = ?)",
            (
                md5_hash,
                job,
                agent_ip,
            ),
        )


def handle_new_updated_allocations(job):
    with maand_data.get_db() as db:
        cursor = db.cursor()
        allocations = maand_data.get_allocations(cursor, job)
        update_allocation_hash(cursor, job, allocations)
        counts = maand_data.get_allocation_counts(cursor, job)
        new_allocations = maand_data.get_new_allocations(cursor, job)
        changed_allocations = maand_data.get_changed_allocations(cursor, job)
        db.rollback()

    if counts["new"] > 0:  # allocations added
        job_control.run_target(
            "start",
            "deploy",
            job,
            new_allocations,
            alloc_health_check_flag=False,
            job_health_check_flag=True,
        )
    elif counts["changed"] < counts["total"]:  # few allocations modified
        job_control.run_target(
            "restart",
            "deploy",
            job,
            changed_allocations,
            alloc_health_check_flag=True,
            job_health_check_flag=False,
        )
    else:
        job_control.run_target(
            "restart",
            "deploy",
            job,
            allocations,
            alloc_health_check_flag=True,
            job_health_check_flag=False,
        )

    with maand_data.get_db() as db:
        cursor = db.cursor()
        allocations = maand_data.get_allocations(cursor, job)
        update_allocation_hash(cursor, job, allocations)
        db.commit()


def handle_agent_files(cursor, agent_ip):
    agent_dir = context_manager.get_agent_dir(agent_ip)
    bucket_id = maand_data.get_bucket_id(cursor)

    command_manager.command_local(f"mkdir -p {agent_dir}")

    agent_id = maand_data.get_agent_id(cursor, agent_ip)
    with open(f"{agent_dir}/agent.txt", "w") as f:
        f.write(agent_id)

    with open(f"{agent_dir}/bucket.txt", "w") as f:
        f.write(bucket_id)

    update_seq = maand_data.get_update_seq(cursor)
    with open(f"{agent_dir}/update_seq.txt", "w") as f:
        f.write(str(update_seq))

    agent_labels = maand_data.get_agent_labels(cursor, agent_ip)
    with open(f"{agent_dir}/labels.txt", "w") as f:
        f.writelines("\n".join(agent_labels))

    agent_jobs = maand_data.get_agent_jobs_and_status(cursor, agent_ip)
    with open(f"{agent_dir}/jobs.json", "w") as f:
        f.writelines(json.dumps(agent_jobs))

    command_manager.command_local(f"mkdir -p {agent_dir}/bin")
    command_manager.command_local(f"rsync -r /maand/agent/bin/ {agent_dir}/bin/")


def update(jobs):
    with maand_data.get_db() as db:
        cursor = db.cursor()

        agents = maand_data.get_agents(cursor, labels_filter=None)
        for agent_ip in agents:
            handle_agent_files(cursor, agent_ip)

            agent_removed_jobs = maand_data.get_agent_removed_jobs(cursor, agent_ip)
            agent_disabled_jobs = maand_data.get_agent_disabled_jobs(cursor, agent_ip)
            stopped_jobs = set(jobs) & set(
                chain(agent_removed_jobs, agent_disabled_jobs)
            )

            for job in stopped_jobs:
                handle_disabled_stopped_allocations(cursor, job)

            db.commit()

            for job in jobs:
                prepare_allocation(
                    cursor, job, agent_ip
                )  # copy files to agent dir

            context_manager.rsync_upload_agent_files(
                agent_ip, jobs, agent_removed_jobs
            )  # update changes


def main():
    args = get_args()

    with maand_data.get_db() as db:
        cursor = db.cursor()

        # Update agents and system environment
        system_manager.run(cursor, command_manager.scan_agent)
        context_manager.export_env_bucket_update_seq(cursor)

        update_seq = maand_data.get_update_seq(cursor)
        next_update_seq = int(update_seq) + 1
        maand_data.update_update_seq(cursor, next_update_seq)

        db.commit()

        max_deployment_seq = maand_data.get_max_deployment_seq(cursor)
        for seq in range(max_deployment_seq + 1):
            jobs = maand_data.get_jobs(cursor, deployment_seq=seq)
            if args.jobs:
                jobs = list(set(jobs) & set(args.jobs))

            update(jobs)

            for job in jobs:
                handle_new_updated_allocations(job)


if __name__ == "__main__":
    main()
