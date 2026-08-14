# syntax=docker/dockerfile:1
FROM alpine:3.24
ARG TARGETARCH
ARG MKCERT_VERSION=1.4.4

RUN apk add --no-cache \
    bash \
    ca-certificates \
    nss-tools \
    openssl \
    wget \
    && wget -O /usr/local/bin/mkcert \
       "https://github.com/FiloSottile/mkcert/releases/download/v${MKCERT_VERSION}/mkcert-v${MKCERT_VERSION}-linux-${TARGETARCH}" \
    && chmod +x /usr/local/bin/mkcert

COPY setup.sh /setup.sh
RUN chmod +x /setup.sh

ENTRYPOINT ["/setup.sh"]
