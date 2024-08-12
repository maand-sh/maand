import os
import subprocess
import sys
import uuid

import context_manager

context_manager.validate_cluster_id()

values = context_manager.get_values()
with open("/opt/agent/context.env", "w") as f:
    keys = sorted(values.keys())
    for key in keys:
        value = values.get(key)
        f.write("export {}={}\n".format(key, value))

file_id = uuid.uuid4()
with open(f"/tmp/{file_id}", "w") as f:
    f.write("#!/bin/bash\n")
    f.write("set -ueo pipefail\n")
    f.write("source /opt/agent/context.env\n")
    f.write("bash /workspace/command.sh\n")

file_path = f"/tmp/{file_id}"
r = subprocess.run(["sh", file_path], env=os.environ.copy())

if r.returncode != 0:
    sys.exit(r.returncode)
