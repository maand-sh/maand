import argparse
import base64
import json
from itertools import chain

import command_helper
import const
import context_manager
import job_control
import job_data
import kv_manager
import maand_data
import system_manager
import update_job


def get_args():
    parser = argparse.ArgumentParser(description="Deploy and manage jobs.")
    parser.add_argument('--jobs', default="", help="Comma-separated list of jobs to process.")
    args = parser.parse_args()
    args.jobs = args.jobs.split(',') if args.jobs else []
    return args


def write_cert(cursor, location, namespace, kv_path):
    content = kv_manager.get(cursor, namespace, kv_path)
    if content:
        content = base64.b64decode(content)
        with open(location, "wb") as f:
            f.write(content)


def handle_disabled_stopped_allocations(cursor, job):
    counts = maand_data.get_allocation_counts(cursor, job)
    removed_allocations = maand_data.get_removed_allocations(cursor, job)
    disabled_allocations = maand_data.get_disabled_allocations(cursor, job)
    allocations = []
    allocations.extend(removed_allocations)
    allocations.extend(disabled_allocations)

    if counts['removed'] == counts['total']:  # job removed
        job_control.run_target("stop", "deploy", job, allocations, alloc_health_check_flag=False,
                               job_health_check_flag=False)
    elif counts['removed'] > 0:  # few allocations removed
        job_control.run_target("stop", "deploy", job, allocations, alloc_health_check_flag=True,
                               job_health_check_flag=False)


def handle_new_updated_allocations(cursor, job):
    counts = maand_data.get_allocation_counts(cursor, job)

    allocations = maand_data.get_allocations(cursor, job)
    new_allocations = maand_data.get_new_allocations(cursor, job)
    changed_allocations = maand_data.get_changed_allocations(cursor, job)

    if counts['new'] > 0:  # allocations added
        job_control.run_target("start", "deploy", job, new_allocations, alloc_health_check_flag=False,
                               job_health_check_flag=True)
    elif counts['changed'] < counts["total"]:  # few allocations modified
        job_control.run_target("restart", "deploy", job, changed_allocations, alloc_health_check_flag=True,
                               job_health_check_flag=False)
    else:
        job_control.run_target("restart", "deploy", job, allocations, alloc_health_check_flag=True,
                               job_health_check_flag=False)


def handle_agent_files(cursor, agent_ip):
    agent_dir = context_manager.get_agent_dir(agent_ip)
    bucket_id = maand_data.get_bucket_id(cursor)

    command_helper.command_local(f"""
        mkdir -p {agent_dir}/certs
        rsync {const.SECRETS_PATH}/ca.crt {agent_dir}/certs/
    """)

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

    command_helper.command_local(f"mkdir -p {agent_dir}/bin")
    command_helper.command_local(f"rsync -r /maand/agent/bin/ {agent_dir}/bin/")


def update(cursor, jobs):
    agents = maand_data.get_agents(cursor, labels_filter=None)

    job_hash = {}
    for agent_ip in agents:
        handle_agent_files(cursor, agent_ip)

        agent_removed_jobs = maand_data.get_agent_removed_jobs(cursor, agent_ip)
        agent_disabled_jobs = maand_data.get_agent_disabled_jobs(cursor, agent_ip)
        stopped_jobs = set(jobs) & set(chain(agent_removed_jobs, agent_disabled_jobs))
        for job in stopped_jobs:
            handle_disabled_stopped_allocations(cursor, job)

        for job in jobs:
            md5_hash = update_job.prepare_allocation(cursor, job, agent_ip)  # copy files to agent dir
            job_hash[job] = md5_hash
        context_manager.rsync_upload_agent_files(agent_ip, jobs, agent_removed_jobs)  # update changes

    return job_hash


def main():
    args = get_args()

    with maand_data.get_db() as db:
        cursor = db.cursor()

        # Update agents and system environment
        system_manager.run(cursor, command_helper.scan_agent)
        context_manager.export_env_bucket_update_seq(cursor)

        update_seq = maand_data.get_update_seq(cursor)
        next_update_seq = int(update_seq) + 1
        maand_data.update_update_seq(cursor, next_update_seq)

        db.commit()

        max_deployment_seq = job_data.get_max_deployment_seq(cursor)
        for seq in range(max_deployment_seq + 1):
            jobs = job_data.get_jobs(cursor, deployment_seq=seq)
            if args.jobs:
                jobs = list(set(jobs) & set(args.jobs))
            job_hash = update(cursor, jobs)
            db.commit()

            for job in jobs:
                handle_new_updated_allocations(cursor, job)

                agents = maand_data.get_allocations(cursor, job)
                md5_hash = job_hash.get(job, None)
                for agent_ip in agents:
                    cursor.execute(
                        "UPDATE agent_jobs SET current_md5_hash = ?, previous_md5_hash = current_md5_hash WHERE job = ? AND agent_id = (SELECT agent_id FROM agent WHERE agent_ip = ?)",
                        (md5_hash, job, agent_ip,))


if __name__ == "__main__":
    main()
