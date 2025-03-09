FROM fedora:42

RUN dnf update -y && dnf install -y \
    python3 python3-pip jq wget rsync tree openssl openssh-clients tini \
    --setopt=install_weak_deps=False --skip-broken && \
    dnf clean all

RUN pip install --no-cache-dir --upgrade pip
COPY requirements.txt /requirements.txt
RUN pip install --no-cache-dir -r /requirements.txt

WORKDIR /bucket