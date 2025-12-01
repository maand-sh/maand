FROM fedora:42 AS builder
RUN dnf update -y && dnf install -y golang gcc make

RUN mkdir /maand

WORKDIR /maand
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o maand && chmod +x maand

RUN cp maand /maand
