import os
import subprocess
import sys
import uuid

file_id = uuid.uuid4()
with open(f"/tmp/{file_id}", "w") as f:
    f.write("#!/bin/bash\n")
    f.write("set -ueo pipefail\n")
    f.write(
        "ssh -o StrictHostKeyChecking=no -o LogLevel=error $SSH_USER@$AGENT_IP 'bash -xs' < /workspace/command.sh")

file_path = f"/tmp/{file_id}"
r = subprocess.run(["sh", file_path], env=os.environ.copy())

if r.returncode != 0:
    sys.exit(r.returncode)
