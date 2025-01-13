from datetime import datetime, timedelta

from OpenSSL import crypto

from core import command_manager
from core import const


def generate_ca_private():
    command_manager.command_local(f"openssl genrsa -out {const.SECRETS_PATH}/ca.key 4096")


def generate_ca_public(common_name, ttl):
    command_manager.command_local(f"openssl req -new -x509 -sha256 -days {ttl} -subj '/CN={common_name}' -key "
                                  f"{const.SECRETS_PATH}/ca.key -out {const.SECRETS_PATH}/ca.crt")


def generate_site_private(name, path):
    command_manager.command_local(f"openssl genrsa -out {path}/{name}.key 4096")


def generate_site_csr(name, subj, path):
    command_manager.command_local(f"openssl req -new -sha256 -subj '{subj}' -key {path}/{name}.key "
                                  f"-out {path}/{name}.csr")


def generate_private_pem_pkcs_8(name, path):
    command_manager.command_local(f"openssl pkcs8 -inform PEM -outform PEM -in {path}/{name}.key -topk8 -nocrypt "
                                  f"-v1 PBE-SHA1-3DES -out {path}/{name}.pem")


def generate_site_public(name, san, ttl, path):
    with open("/tmp/extfile.conf", "w") as f:
        f.writelines(f"subjectAltName={san}")
    command_manager.command_local(f"openssl x509 -req -sha256 -days {ttl} -in {path}/{name}.csr "
                                  f"-CA {const.SECRETS_PATH}/ca.crt -CAkey {const.SECRETS_PATH}/ca.key "
                                  f"-out {path}/{name}.crt -extfile /tmp/extfile.conf -CAcreateserial 2> /dev/null")


def trust_ca_public():
    command_manager.command_local("""
        update-ca-trust force-enable
        cp /opt/agent/certs/ca.crt /etc/pki/ca-trust/source/anchors/
        update-ca-trust extract
    """)


def generate_encryption_key():
    result = command_manager.command_local("openssl rand -base64 32")
    return result.stdout.decode('utf-8').strip()


def is_certificate_expiring_soon(cert_file_path, days=15):
    # Load the certificate from the file
    with open(cert_file_path, 'rb') as cert_file:
        cert_data = cert_file.read()

    # Load the certificate using pyOpenSSL
    cert = crypto.load_certificate(crypto.FILETYPE_PEM, cert_data)

    # Get the expiration date of the certificate
    expiry_date = cert.get_notAfter().decode('ascii')
    expiry_date = datetime.strptime(expiry_date, '%Y%m%d%H%M%SZ')

    # Check if the certificate will expire within the specified number of days
    return expiry_date <= datetime.utcnow() + timedelta(days=days)
