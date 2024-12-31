import subprocess
import uuid

import const
import utils


def capture_command_local(cmd, env, prefix):
    file_id = uuid.uuid4()

    with open(f"/tmp/{file_id}", "w") as f:
        f.write("#!/bin/bash\n")
        f.write(cmd)
    file_path = f"/tmp/{file_id}"

    logger = utils.get_logger(prefix)

    process = subprocess.Popen(
        ["sh", file_path],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )

    for line in process.stdout:
        line = line.strip()
        logger.info(line)
        for handler in logger.handlers:
            handler.flush()

    for line in process.stderr:
        line = line.strip()
        logger.info(line)
        for handler in logger.handlers:
            handler.flush()

    process.wait()


def command_local(cmd, env=None, stdout=None, stderr=None):
    file_id = uuid.uuid4()
    with open(f"/tmp/{file_id}", "w") as f:
        f.write("#!/bin/bash\n")
        f.write(cmd)
    file_path = f"/tmp/{file_id}"
    return subprocess.run(["sh", file_path], env=env, stdout=stdout, stderr=stderr)


def capture_command_remote(cmd, env, prefix):
    use_sudo = env.get("USE_SUDO", "0") == "1"
    file_id = uuid.uuid4()
    with open(f"/tmp/{file_id}", "w") as f:
        f.write("#!/bin/bash\n")
        f.write(cmd)
    file_path = f"/tmp/{file_id}"
    sh = "sh" if not use_sudo else "sudo sh"
    return capture_command_local(
        f"ssh -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i {const.BUCKET_PATH}/$SSH_KEY $SSH_USER@$AGENT_IP 'timeout 300 {sh}' < {file_path}",
        env=env, prefix=prefix, )


def command_remote(cmd, env=None, stdout=None, stderr=None):
    use_sudo = env.get("USE_SUDO", "0") == "1"
    file_id = uuid.uuid4()
    with open(f"/tmp/{file_id}", "w") as f:
        f.write("#!/bin/bash\n")
        f.write(cmd)
    file_path = f"/tmp/{file_id}"
    sh = "sh" if not use_sudo else "sudo sh"
    return command_local(
        f"ssh -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i {const.BUCKET_PATH}/$SSH_KEY $SSH_USER@$AGENT_IP 'timeout 300 {sh}' < {file_path}",
        env=env, stdout=stdout, stderr=stderr)


def command_file_remote(file_path, env=None, stdout=None, stderr=None):
    use_sudo = env.get("USE_SUDO", "0") == "1"
    sh = "sh" if not use_sudo else "sudo sh"
    return command_local(
        f"ssh -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i {const.BUCKET_PATH}/$SSH_KEY $SSH_USER@$AGENT_IP 'timeout 300 {sh}' < {file_path}",
        env=env, stdout=stdout, stderr=stderr)


def capture_command_file_remote(file_path, env, prefix):
    use_sudo = env.get("USE_SUDO", "0") == "1"
    sh = "sh" if not use_sudo else "sudo sh"
    return capture_command_local(
        f"ssh -q -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i {const.BUCKET_PATH}/$SSH_KEY $SSH_USER@$AGENT_IP 'timeout 300 {sh}' < {file_path}",
        env, prefix)


def scan_agent(agent_ip):
    agent_file = agent_ip.replace(".", "_")
    command_local(f"ssh-keyscan -H {agent_ip} > /tmp/{agent_file}.agent; cat /tmp/*.agent > ~/.ssh/known_hosts",
                  stderr=subprocess.DEVNULL)
