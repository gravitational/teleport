ARG TELEPORT_VERSION
FROM quay.io/gravitational/debian-grande:latest

ENV KUBECTL_VERSION v1.12.4
ENV CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Update packages
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get -y install curl && \
    apt-get -y autoremove && \
    rm -rf /var/cache/apt

# install kubectl
RUN curl ${CURL_OPTS} https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl --output /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

COPY rootfs/ /

ENTRYPOINT ["/usr/bin/dumb-init", "/scripts/secret-cleaner.sh"]