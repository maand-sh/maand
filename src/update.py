import argparse
import base64
import json
from copy import deepcopy
from pathlib import Path
from string import Template

import command_helper
import const
import context_manager
import kv_manager
import maand_data
import system_manager
import utils

logger = utils.get_logger()


def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument('--jobs', default="", required=False)
    parser.add_argument('--concurrency', default="1", type=int)
    args = parser.parse_args()

    if args.jobs:
        args.jobs = args.jobs.split(',')

    return args


def write_cert(cursor, location, namespace, kv_path):
    content = kv_manager.get(cursor, namespace, kv_path)
    if content:
        content = base64.b64decode(content)
        with open(location, "wb") as f:
            f.write(content)


def update_certificates(cursor, jobs, agent_ip):
    agent_dir = context_manager.get_agent_dir(agent_ip)

    name = "agent"
    agent_cert_location = f"{agent_dir}/certs"
    agent_cert_path = f"{agent_cert_location}/{name}"
    agent_cert_kv_path = f"certs/{name}"

    write_cert(
        cursor, f"{agent_cert_path}.key", f"certs/{agent_ip}", f"{agent_cert_kv_path}.key"
    )
    write_cert(
        cursor, f"{agent_cert_path}.crt", f"certs/{agent_ip}", f"{agent_cert_kv_path}.crt"
    )
    write_cert(
        cursor, f"{agent_cert_path}.pem", f"certs/{agent_ip}", f"{agent_cert_kv_path}.pem"
    )

    for job in jobs:
        job_cert_location = f"{agent_dir}/jobs/{job}/certs"
        job_cert_kv_location = f"{job}/certs"
        namespace = f"certs/job/{agent_ip}"

        job_certs = maand_data.get_job_certs_config(cursor, job)

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


def process_templates(cursor, values, jobs):
    values = deepcopy(values)
    for k, v in values.items():
        values[k] = v.replace("$$", "$")
    agent_ip = values["AGENT_IP"]
    agent_dir = context_manager.get_agent_dir(agent_ip)
    logger.debug("Processing templates...")
    for ext in ["*.json", "*.service", "*.conf", "*.yml", "*.yaml", "*.env", "*.txt"]:
        for job in jobs:
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


def transpile(cursor, agent_ip, jobs):
    logger.debug("Transpiling templates...")
    values = context_manager.get_agent_env(cursor, agent_ip)
    process_templates(cursor, values, jobs)


def sync(agent_ip):
    with maand_data.get_db() as db:
        cursor = db.cursor()

        args = get_args()
        logger.debug("Starting sync process...")
        agent_dir = context_manager.get_agent_dir(agent_ip)
        bucket_id = maand_data.get_bucket_id(cursor)

        command_helper.command_local(f"""
            mkdir -p {agent_dir}/certs
            rsync {const.SECRETS_PATH}/ca.crt {agent_dir}/certs/
        """)

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

        command_helper.command_local(f"mkdir -p {agent_dir}/bin")
        command_helper.command_local(f"rsync -r /maand/agent/bin/ {agent_dir}/bin/")

        agent_jobs = maand_data.get_agent_jobs(cursor, agent_ip)
        with open(f"{agent_dir}/jobs.json", "w") as f:
            f.writelines(json.dumps(agent_jobs))

        if len(agent_jobs) > 0:
            command_helper.command_local(f"mkdir -p {agent_dir}/jobs/")

            for job in agent_jobs:
                maand_data.copy_job(cursor, job, agent_dir)

        transpile(cursor, agent_ip, agent_jobs.keys())
        update_certificates(cursor, agent_jobs, agent_ip)

        command_helper.command_local(f"chown -R 1061:1062 {agent_dir}")

        removed_jobs = maand_data.get_agent_removed_jobs(cursor, agent_ip)
        disabled_jobs = maand_data.get_agent_disabled_jobs(cursor, agent_ip)
        jobs = list(agent_jobs.keys())
        if args.jobs:
            jobs = list(set(agent_jobs.keys()) & set(args.jobs))
            removed_jobs = list(set(jobs) & set(removed_jobs))
            disabled_jobs = list(set(jobs) & set(disabled_jobs))

        agent_env = context_manager.get_agent_minimal_env(agent_ip)

        if removed_jobs:
            command_helper.capture_command_remote(
                f"test -f /opt/agent/{bucket_id}/bin/runner.py && python3 /opt/agent/{bucket_id}/bin/runner.py {bucket_id} stop --jobs {','.join(removed_jobs)}",
                env=agent_env, prefix=agent_ip)

        if disabled_jobs:
            command_helper.capture_command_remote(
                f"test -f /opt/agent/{bucket_id}/bin/runner.py && python3 /opt/agent/{bucket_id}/bin/runner.py {bucket_id} stop --jobs {','.join(disabled_jobs)}",
                env=agent_env, prefix=agent_ip)

        context_manager.rsync_upload_agent_files(agent_ip, jobs, removed_jobs)

        logger.debug("Sync process completed.")


def validate_agent_namespace(agent_ip):
    context_manager.validate_agent_bucket(agent_ip, fail_if_no_bucket_id=False)


def update():
    args = get_args()
    with maand_data.get_db() as db:
        cursor = db.cursor()

        context_manager.export_env_bucket_update_seq(cursor)

        system_manager.run(cursor, command_helper.scan_agent)
        system_manager.run(cursor, validate_agent_namespace)

        update_seq = maand_data.get_update_seq(cursor)
        next_update_seq = int(update_seq) + 1
        maand_data.update_update_seq(cursor, next_update_seq)

        db.commit()

        system_manager.run(cursor, sync, concurrency=args.concurrency)


if __name__ == "__main__":
    update()
