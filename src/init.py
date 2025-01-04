import configparser
import os
import sys

import cert_provider
import command_manager
import const
import job_data
import kv_manager
import maand_data
import utils

logger = utils.get_logger()


def build_maand_conf():
    config = {
        "use_sudo": "1",
        "ssh_user": "agent",
        "ssh_key": "agent.key",
        "certs_ttl": "60"
    }

    config_parser = configparser.ConfigParser()
    config_parser.add_section("default")
    for key, value in config.items():
        config_parser.set('default', key, value)

    if not os.path.isfile(const.CONF_PATH):
        with open(const.CONF_PATH, 'w') as f:
            config_parser.write(f)


def init():
    try:
        if os.path.isfile(const.MAAND_DB_PATH):
            raise Exception("bucket is already initialized")

        command_manager.command_local(f"mkdir -p {const.BUCKET_PATH}/{{workspace,secrets,logs,data}}")
        command_manager.command_local(f"touch {const.WORKSPACE_PATH}/agents.json")

        with maand_data.get_db(fail_if_not_found=False) as db:
            cursor = db.cursor()
            maand_data.setup_maand_database(cursor)
            job_data.setup_job_database(cursor)
            kv_manager.setup_kv_database(cursor)

            with open(f"{const.WORKSPACE_PATH}/agents.json", "r") as f:
                data = f.read().strip()
                if len(data) == 0:
                    command_manager.command_local(f"echo '[]' > {const.WORKSPACE_PATH}/agents.json")

            build_maand_conf()

            if not os.path.isfile(f'{const.BUCKET_PATH}/secrets/ca.key'):
                cert_provider.generate_ca_private()
                bucket_id = maand_data.get_bucket_id(cursor)
                cert_provider.generate_ca_public(bucket_id, 3650)

            command_manager.command_local("chmod -R 755 /bucket")
            command_manager.command_local("chmod -R 600 /bucket/secrets/*")

            db.commit()

    except Exception as e:
        logger.fatal(e)
        sys.exit(1)


if __name__ == '__main__':
    init()
