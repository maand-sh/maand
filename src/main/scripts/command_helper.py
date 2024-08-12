import os
import subprocess
import uuid


def command_local(cmd, env=None):
    file_id = uuid.uuid4()
    with open(f"/tmp/{file_id}", "w") as f:
        f.write("#!/bin/bash\n")
        f.write(cmd)
    file_path = f"/tmp/{file_id}"
    env = env or os.environ.copy()
    try:
        return subprocess.run(["sh", file_path], env=env, check=True)
    except Exception as e:
        raise e


def command_remote(cmd, agent_ip=None):
    file_id = uuid.uuid4()
    with open(f"/tmp/{file_id}", "w") as f:
        f.write("#!/bin/bash\n")
        f.write(cmd)
    file_path = f"/tmp/{file_id}"
    env = os.environ.copy()
    env.setdefault("AGENT_IP", agent_ip)
    return command_local(
        f"ssh -o StrictHostKeyChecking=no -o LogLevel=error $SSH_USER@$AGENT_IP 'sh -s' < {file_path}", env=env)
