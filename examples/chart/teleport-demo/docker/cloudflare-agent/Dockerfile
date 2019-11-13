ARG TELEPORT_VERSION
FROM quay.io/gravitational/debian-grande:buster

ARG KUBECTL_VERSION="v1.12.5"
ARG CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Update packages
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && \
    apt-get -y install curl jq python3 python3-pip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt

# install kubectl
RUN curl ${CURL_OPTS} https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl --output /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

# Install certbot to get/rotate certificates, add certbot-dns-cloudflare for registration
RUN pip3 install wheel && \
    pip3 install certbot certbot-dns-cloudflare

COPY rootfs/ /

ENTRYPOINT ["/usr/bin/dumb-init", "/scripts/cloudflare-agent.sh"]