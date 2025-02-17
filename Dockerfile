FROM fedora:42 AS builder
RUN dnf update -y && dnf install -y golang gcc make

WORKDIR /maand
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o maand && chmod +x maand

FROM fedora:42
RUN dnf update -y && dnf install -y \
    python3 python3-pip jq wget rsync tree openssl openssh-clients tini \
    --setopt=install_weak_deps=False --skip-broken && \
    dnf clean all

RUN pip install --no-cache-dir --upgrade pip
COPY --from=builder --chmod=0755 /maand/maand /maand

ENV CONTAINER=1
WORKDIR /bucket
ENTRYPOINT ["tini", "-g", "-p", "SIGTERM", "--", "/maand"]
