import os
import uuid
from pathlib import Path
from string import Template

import certs
import command_helper
import context_manager
import utils

logger = utils.get_logger()


def process_templates(values):
    logger.info("Processing templates...")
    for ext in ["*.json", "*.service", "*.conf", "*.yml", "*.env", "*.token"]:
        for f in Path('/opt/agent/').rglob(ext):
            try:
                with open(f, 'r') as file:
                    data = file.read()
                template = Template(data)
                content = template.substitute(values)
                if content != data:
                    with open(f, 'w') as file:
                        file.write(content)
                logger.info(f"Processed template: {f}")
            except Exception as e:
                logger.error(f"Error processing file {f}: {e}")
                raise e


def transpile():
    logger.info("Transpiling templates...")
    values = context_manager.get_values()
    process_templates(values)


def sync():
    logger.info("Starting sync process...")
    context_manager.validate_cluster_id()

    cluster_id = os.getenv("CLUSTER_ID")
    agent_ip = os.getenv("AGENT_IP")

    if not cluster_id:
        logger.error("CLUSTER_ID environment variables are not set.")
        return

    command_helper.command_remote("mkdir -p /opt/agent")
    command_helper.command_local("bash /scripts/rsync_remote_local.sh")

    if not os.path.isfile("/opt/agent/cluster_id.txt"):
        with open("/opt/agent/cluster_id.txt", "w") as f:
            f.write(cluster_id)

    if not os.path.isfile("/opt/agent/agent_id.txt"):
        with open("/opt/agent/agent_id.txt", "w") as f:
            f.write(uuid.uuid4().__str__())

    if not os.path.isfile("/workspace/ca.key"):
        raise Exception("CA key file not found.")
    #     certs.generate_ca_private()
    #     certs.generate_ca_public(cluster_id, 365)

    command_helper.command_local("""
        mkdir -p /opt/agent/certs
        rsync /workspace/ca.crt /opt/agent/certs/
    """)

    agents = utils.get_agent_and_roles()
    agent_roles = agents.get(agent_ip, [])
    with open("/opt/agent/roles.txt", "w") as f:
        f.writelines("\n".join(agent_roles))

    assigned_jobs = []
    role_jobs = utils.get_role_and_jobs()

    for role in agent_roles:
        if role not in role_jobs:
            continue
        jobs = role_jobs.get(role)
        assigned_jobs.extend(jobs)

    assigned_jobs = list(set(assigned_jobs))

    command_helper.command_local("""
        rsync -r /agent/bin /opt/agent/
        mkdir -p /opt/agent/jobs/
    """)

    if assigned_jobs:
        for job in assigned_jobs:
            command_helper.command_local(
                f"rsync -r --exclude 'modules' /workspace/jobs/{job} /opt/agent/jobs/")

    transpile()

    values = context_manager.get_values()
    with open("/opt/agent/values.env", "w") as f:
        for key, value in values.items():
            f.write("export {}={}\n".format(key, value))

    update_certificates(assigned_jobs, cluster_id)

    command_helper.command_local("rm -f /workspace/ca.srl")
    command_helper.command_local("bash /scripts/rsync_local_remote.sh")
    logger.info("Sync process completed.")


def update_certificates(jobs, cluster_id):
    logger.info("Updating certificates...")
    agent_ip = os.getenv("AGENT_IP")

    path = "/opt/agent/certs"
    if (not os.path.isfile(f"{path}/agent.key") or
            (os.path.isfile(f"{path}/agent.crt") and certs.is_certificate_expiring_soon(f"{path}/agent.crt"))):
        name = "agent"
        certs.generate_site_private(name, path)
        certs.generate_private_pem_pkcs_8(name, path)
        certs.generate_site_csr(name, cluster_id, path)
        subject_alt_name = f"DNS.1:localhost,IP.1:127.0.0.1,IP.2:{agent_ip}"
        certs.generate_site_public(name, subject_alt_name, 60, path)
        command_helper.command_local(f"rm -f {path}/{name}.csr")

    for job in jobs:
        metadata = utils.get_job_metadata(job, base_path=f"/opt/agent/jobs")
        certificates = metadata.get("certs", [])
        for cert in certificates:

            path = f"/opt/agent/jobs/{job}/certs"
            command_helper.command_local(f"mkdir -p {path}")

            for name, cert_config in cert.items():

                if (not os.path.isfile(f"{path}/{name}.key") or
                        (os.path.isfile(f"{path}/{name}.key") and certs.is_certificate_expiring_soon(
                            f"{path}/{name}.crt"))):

                    ttl = cert_config.get("ttl", 60)
                    certs.generate_site_private(name, path)

                    if cert_config.get("pkcs8", False):
                        certs.generate_private_pem_pkcs_8(name, path)

                    common = cert_config.get("subject", cluster_id)
                    certs.generate_site_csr(name, common, path)

                    subject_alt_name = cert_config.get("subject_alt_name",
                                                       f"DNS.1:localhost,IP.1:127.0.0.1,IP.2:{agent_ip}")
                    certs.generate_site_public(name, subject_alt_name, ttl, path)
                    command_helper.command_local(f"rm -f {path}/{name}.csr")
                    logger.info(f"Updated certificate: {name}")


if __name__ == "__main__":
    sync()
