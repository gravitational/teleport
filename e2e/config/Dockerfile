FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y \
    curl \
    ca-certificates \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*

RUN curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64" && \
    chmod +x mkcert-v*-linux-amd64 && \
    cp mkcert-v*-linux-amd64 /usr/local/bin/mkcert

RUN mkdir -p /etc/teleport/certs && \
    mkcert -install && \
    mkcert -cert-file /etc/teleport/certs/tls.crt -key-file /etc/teleport/certs/tls.key localhost \
    "*.localhost" "teleport"

COPY ./build/teleport /usr/local/bin/teleport
COPY ./build/tctl /usr/local/bin/tctl

CMD ["/usr/local/bin/teleport", "start", "-c", "/etc/teleport/teleport.yaml"]