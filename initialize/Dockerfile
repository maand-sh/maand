FROM maand_base

WORKDIR /
COPY ./requirements.txt /requirements.txt
RUN pip install --no-cache-dir -r /requirements.txt

WORKDIR /bucket
ENTRYPOINT ["tini", "-g", "-p", "SIGTERM", "--", "/maand"]