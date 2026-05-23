FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /maand
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o maand && chmod +x maand

FROM alpine:3.21
COPY --from=builder /maand/maand /usr/local/bin/maand
ENTRYPOINT ["maand"]
