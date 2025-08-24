FROM fedora:42

RUN dnf update -y && dnf install -y \
    python3 python3-pip rsync openssl openssh-clients tini \
    glibc-locale-source glibc-langpack-en \
    --setopt=install_weak_deps=False --skip-broken && \
    dnf clean all

RUN localedef -i en_US -f UTF-8 en_US.UTF-8

ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV PYTHONIOENCODING=utf-8

COPY requirements.txt /requirements.txt
RUN pip install --no-cache-dir --upgrade pip && pip install --no-cache-dir -r /requirements.txt

WORKDIR /bucket