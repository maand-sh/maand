import hashlib
import json
import os
import uuid
from pathlib import Path
from string import Template

import cert_provider
import command_helper
import context_manager
import utils

logger = utils.get_logger()


def process_templates(values):
    logger.debug("Processing templates...")
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
                logger.debug(f"Processed template: {f}")
            except Exception as e:
                logger.error(f"Error processing file {f}: {e}")
                raise e


def transpile():
    logger.debug("Transpiling templates...")
    values = context_manager.get_values()
    context_manager.load_secrets(values)
    process_templates(values)


def sync():
    logger.debug("Starting sync process...")
    context_manager.validate_cluster_id()
    cluster_id = os.getenv("CLUSTER_ID")
    agent_ip = os.getenv("AGENT_IP")

    command_helper.command_remote("mkdir -p /opt/agent")
    command_helper.command_local("bash /scripts/rsync_remote_local.sh")

    if not os.path.isfile("/opt/agent/cluster_id.txt"):
        with open("/opt/agent/cluster_id.txt", "w") as f:
            f.write(cluster_id)

    if not os.path.isfile("/opt/agent/agent_id.txt"):
        with open("/opt/agent/agent_id.txt", "w") as f:
            f.write(uuid.uuid4().__str__())

    command_helper.command_local("""
        mkdir -p /opt/agent/certs
        rsync /workspace/ca.crt /opt/agent/certs/
    """)

    assigned_jobs = utils.get_assigned_jobs(agent_ip)
    assigned_roles = utils.get_assigned_roles(agent_ip)

    with open("/opt/agent/roles.txt", "w") as f:
        f.writelines("\n".join(assigned_roles))

    command_helper.command_local("""
        rsync -r /agent/bin /opt/agent/
        mkdir -p /opt/agent/jobs/
    """)

    for job in assigned_jobs:
        command_helper.command_local(f"rsync -r --exclude 'modules' /workspace/jobs/{job} /opt/agent/jobs/")

    transpile()

    values = context_manager.get_values()
    with open("/opt/agent/context.env", "w") as f:
        keys = sorted(values.keys())
        for key in keys:
            value = values.get(key)
            f.write("{}={}\n".format(key, value))

    update_certificates(assigned_jobs, cluster_id)

    command_helper.command_local("rm -f /workspace/ca.srl")
    command_helper.command_local("bash /scripts/rsync_local_remote.sh")
    logger.debug("Sync process completed.")


def update_certificates(jobs, cluster_id):
    agent_ip = os.getenv("AGENT_IP")

    path = "/opt/agent/certs"
    if (not os.path.isfile(f"{path}/agent.key") or
            (os.path.isfile(f"{path}/agent.crt") and cert_provider.is_certificate_expiring_soon(f"{path}/agent.crt"))):
        logger.debug(f"Updating certificates agent.key and agent.crt")
        name = "agent"
        cert_provider.generate_site_private(name, path)
        cert_provider.generate_private_pem_pkcs_8(name, path)
        cert_provider.generate_site_csr(name, f"/CN={cluster_id}", path)
        subject_alt_name = f"DNS.1:localhost,IP.1:127.0.0.1,IP.2:{agent_ip}"
        cert_provider.generate_site_public(name, subject_alt_name, 60, path)
        command_helper.command_local(f"rm -f {path}/{name}.csr")

    for job in jobs:
        metadata = utils.get_job_metadata(job, base_path=f"/opt/agent/jobs")
        certificates = metadata.get("certs", [])

        path = f"/opt/agent/jobs/{job}/certs"
        command_helper.command_local(f"mkdir -p {path}")

        update_certs = False
        hash_file = f"/opt/agent/jobs/{job}/certs/md5.hash"

        certs_str = json.dumps(certificates)
        new_hash = hashlib.md5(certs_str.encode()).hexdigest()

        if os.path.exists(hash_file):
            with open(hash_file, "r") as f:
                current_hash = f.read()

            if new_hash != current_hash:
                update_certs = True
        else:
            update_certs = True

        with open(hash_file, "w") as f:
            f.write(new_hash)

        for cert in certificates:
            for name, cert_config in cert.items():
                if update_certs or (not os.path.isfile(f"{path}/{name}.key") or
                        (os.path.isfile(f"{path}/{name}.key") and cert_provider.is_certificate_expiring_soon(
                            f"{path}/{name}.crt"))):
                    logger.debug(f"Updating certificates {name}.key and {name}.crt")
                    ttl = cert_config.get("ttl", 60)
                    cert_provider.generate_site_private(name, path)

                    if cert_config.get("pkcs8", False):
                        cert_provider.generate_private_pem_pkcs_8(name, path)

                    subj = cert_config.get("subject", f"/CN={cluster_id}")
                    cert_provider.generate_site_csr(name, subj, path)

                    subject_alt_name = cert_config.get("subject_alt_name",
                                                       f"DNS.1:localhost,IP.1:127.0.0.1,IP.2:{agent_ip}")
                    cert_provider.generate_site_public(name, subject_alt_name, ttl, path)
                    command_helper.command_local(f"rm -f {path}/{name}.csr")


if __name__ == "__main__":
    sync()
