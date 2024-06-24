import os
import subprocess
import sys
import uuid


def command_local(cmd, env=None, return_error=False):
    file_id = uuid.uuid4()
    with open(f"/tmp/{file_id}", "w") as f:
        f.write("#!/bin/bash\n")
        f.write(cmd)
    file_path = f"/tmp/{file_id}"
    env = env or os.environ.copy()
    r = None
    try:
        r = subprocess.run(["sh", file_path], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    except Exception as e:
        with open(file_path, "r") as f:
            print(f.read())
    finally:
        if not return_error and r.returncode != 0:
            print(r.stderr.decode('utf-8'))
            sys.exit(1)
        return r


def command_file_local(file_path, env=None, return_error=False):
    env = env or os.environ.copy()
    r = None
    try:
        r = subprocess.run(["sh", file_path], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    except Exception as e:
        with open(file_path, "r") as f:
            print(f.read())
    finally:
        if not return_error and r.returncode != 0:
            print(r.stderr.decode('utf-8'))
            sys.exit(1)
        return r


def command_remote(cmd, host=None):
    file_id = uuid.uuid4()
    with open(f"/tmp/{file_id}", "w") as f:
        f.write("#!/bin/bash\n")
        f.write(cmd)
    file_path = f"/tmp/{file_id}"
    env = os.environ.copy()
    if host:
        env.setdefault("HOST", host)
    try:
        return command_local(f"ssh -o StrictHostKeyChecking=no -o LogLevel=error $SSH_USER@$HOST 'sh -s' < {file_path}",
                             env=env)
    except Exception as e:
        print(env)
        raise e


def command_file_remote(file_path, host=None):
    env = os.environ.copy()
    if host:
        env.setdefault("HOST", host)
    return command_local(f"ssh -o StrictHostKeyChecking=no -o LogLevel=error $SSH_USER@$HOST 'sh -s' < {file_path}",
                         env=env)