import os
import uuid

import command_helper

command_helper.command_remote("""
    mkdir -p /opt/agent
""")

command_helper.command_local("""
    mkdir -p /opt/agent        
    bash /scripts/rsync_remote_local.sh    
""")

file_path = "/opt/agent/node.txt"

if not os.path.exists(file_path):
    with open(file_path, "w") as f:
        f.write(str(uuid.uuid4()))

    command_helper.command_local("""        
        bash /scripts/rsync_local_remote.sh              
    """)

