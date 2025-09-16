FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG BUILDARCH
RUN curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/${BUILDARCH}" && \
    chmod +x mkcert-v*-linux-${BUILDARCH} && \
    cp mkcert-v*-linux-${BUILDARCH} /usr/local/bin/mkcert

RUN mkdir -p /etc/teleport/certs && \
    mkcert -install && \
    mkcert -cert-file /etc/teleport/certs/tls.crt -key-file /etc/teleport/certs/tls.key localhost \
    "*.localhost" "teleport" && \
    cp $(mkcert -CAROOT)/rootCA.pem /etc/teleport/certs/

CMD ["/usr/local/bin/teleport", "start", "-c", "/etc/teleport/teleport.yaml", "--bootstrap", "/etc/teleport/state.yaml"]