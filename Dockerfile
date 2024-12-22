FROM fedora:42

RUN yum update -y
RUN yum install -y python3 python3-pip jq wget rsync tree openssl openssh-clients tini docker unzip sqlite3

COPY requirements.txt /requirements.txt
RUN pip install --no-cache-dir --upgrade pip && pip install --no-cache-dir -r requirements.txt

COPY src /maand
RUN chmod +x /maand/*.sh
ENV PYTHONPATH=/maand

ENTRYPOINT ["tini", "-g", "-p", "SIGTERM", "--", "bash", "/maand/start.sh"]
