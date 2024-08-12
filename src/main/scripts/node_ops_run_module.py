import os

import command_helper
import context_manager
import utils


def run_module(module=None):
    context_manager.validate_cluster_id()

    agent_ip = os.getenv("AGENT_IP")
    assigned_jobs = utils.get_assigned_jobs(agent_ip)

    values = context_manager.get_values()
    if module is None:
        module = os.getenv("MODULE")
    values["MODULE"] = module

    with open("/opt/agent/context.env", "w") as f:
        for key, value in values.items():
            f.write("export {}={}\n".format(key, value))

    for job in assigned_jobs:
        if os.path.exists(f"/workspace/jobs/{job}/modules/run.sh"):
            command_helper.command_local(f"""
                mkdir -p /modules/{job} && rsync -r /workspace/jobs/{job}/modules/ /modules/{job}/                
                cd /modules/{job} && source /opt/agent/context.env && bash /modules/{job}/run.sh
            """)


if __name__ == "__main__":
    run_module()
