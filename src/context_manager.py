import os
import subprocess

import command_helper
import kv_manager
import utils
import maand


def get_agent_dir(agent_ip):
    return f"/opt/agents/{agent_ip}"


def get_agent_minimal_env(agent_ip):
    config = utils.get_maand_conf()
    return {
        "AGENT_IP": agent_ip,
        "AGENT_DIR": get_agent_dir(agent_ip),
        "SSH_USER": config.get("default", "ssh_user"),
        "SSH_KEY": config.get("default", "ssh_key"),
        "USE_SUDO": config.get("default", "use_sudo"),
        "BUCKET": os.environ.get("BUCKET")
    }


def get_agent_env(cursor, agent_ip):
    env = get_agent_minimal_env(agent_ip)
    for ns in ["variables.env", "secrets.env", "ports.env", f"vars/{agent_ip}"]:
        keys = kv_manager.get_keys(cursor, ns)
        for key in keys:
            env[key] = kv_manager.get(cursor, ns, key)
    return env


def rsync_upload_agent_files(agent_ip, jobs, agent_removed_jobs):
    agent_env = get_agent_minimal_env(agent_ip)
    lines = []

    for job in jobs:
        lines.append(f"+ jobs/{job}\n")
    for job in agent_removed_jobs:
        lines.append(f"+ jobs/{job}\n")

    lines.append("- jobs/*\n")

    with open(f"/tmp/{agent_ip}_rsync_rules.txt", "w") as f:
        f.writelines(lines)

    bucket = agent_env.get("BUCKET", "")
    command_helper.command_remote(f"mkdir -p /opt/agent/{bucket}", env=agent_env)
    command_helper.command_local("bash /maand/rsync_upload.sh", env=agent_env)


def validate_agent_bucket(agent_ip, fail_if_no_bucket_id=True):
    logger = utils.get_logger(ns=agent_ip)
    try:
        agent_env = get_agent_minimal_env(agent_ip)
        bucket = os.environ.get("BUCKET")
        res = command_helper.command_remote(f"ls /opt/agent/{bucket}", agent_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if fail_if_no_bucket_id and res.returncode != 0:
            raise Exception(f"agent {agent_ip} : bucket not found.")
    except Exception as e:
        logger.error(e)
        utils.stop_the_world()


def validate_update_seq(agent_ip):
    logger = utils.get_logger(ns=agent_ip)
    try:
        agent_env = get_agent_minimal_env(agent_ip)
        update_seq = os.environ.get("UPDATE_SEQ")
        bucket_id = os.environ.get("BUCKET")
        res = command_helper.command_remote(f"cat /opt/agent/{bucket_id}/update_seq.txt", agent_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if res.returncode == 1:
            raise Exception(f"{agent_ip} : {res.stderr}")
        agent_update_seq = res.stdout.decode("utf-8")
        if res.returncode == 0 and agent_update_seq != update_seq:
            raise AssertionError(f"Failed on update_seq validation: mismatch, agent {agent_ip}.")
    except Exception as e:
        logger.error(e)
        utils.stop_the_world()


def validate_cluster_update_seq(agent_ip):
    validate_agent_bucket(agent_ip)
    validate_update_seq(agent_ip)


def export_env_bucket_update_seq(cursor):
    bucket = maand.get_bucket_id(cursor)
    os.environ.setdefault("BUCKET", bucket)
    update_seq = maand.get_update_seq(cursor)
    os.environ.setdefault("UPDATE_SEQ", str(update_seq))
