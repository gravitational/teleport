ARG TELEPORT_VERSION
FROM quay.io/gravitational/debian-grande:latest

ARG KUBECTL_VERSION="v1.12.5"
ARG CURL_OPTS="-L --retry 100 --retry-delay 0 --connect-timeout 10 --max-time 300"

# Update packages
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && \
    apt-get -y install curl jq python2.7 build-essential python-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt

# install kubectl
RUN curl ${CURL_OPTS} https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl --output /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

# Install certbot to get/rotate certificates, add certbot-dns-cloudflare for registration
RUN curl ${CURL_OPTS} -O https://bootstrap.pypa.io/get-pip.py && \
    python2.7 get-pip.py && \
    pip install certbot certbot-dns-cloudflare

COPY rootfs/ /

ENTRYPOINT ["/usr/bin/dumb-init", "/scripts/cloudflare-agent.sh"]