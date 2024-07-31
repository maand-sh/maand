import os

import command_helper
import context_manager
import utils


def run_module(module=None):
    context_manager.validate_cluster_id()

    agent_ip = os.getenv("AGENT_IP")

    agents = utils.get_agent_and_roles()
    roles = agents.get(agent_ip, [])

    values = context_manager.get_values()
    if module is None:
        module = os.getenv("MODULE")
    values["MODULE"] = module

    with open("/opt/agent/values.env", "w") as f:
        for key, value in values.items():
            f.write("export {}={}\n".format(key, value))

    for role in roles:
        if os.path.exists(f"/workspace/jobs/{role}/modules/run.sh"):
            command_helper.command_local_stdout(f"""
                mkdir -p /modules/{role} && rsync -r /workspace/jobs/{role}/modules/ /modules/{role}/                
                cd /modules/{role} && source /opt/agent/values.env && bash /modules/{role}/run.sh
            """)


if __name__ == "__main__":
    run_module()
