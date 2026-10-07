# Amplify custom build images must be glibc-based and support the build runner's
# expected shell/git/ssh tooling. Amazon Linux 2023 keeps us close to Amplify's
# managed Amazon Linux environment while providing a new enough glibc for the
# official Node 22 binaries.
FROM amazonlinux:2023

# AWS Amplify docs have a suggested list of required build tools and we install a few extras for convenience.
# Ref: https://docs.aws.amazon.com/amplify/latest/userguide/custom-build-image.html
RUN yum install -y \
      awscli \
      bash \
      ca-certificates \
      gcc-c++ \
      git \
      jq \
      openssh-clients \
      sudo \
      tar \
      wget \
      which && \
    yum clean all && \
    rm -rf /var/cache/dnf /var/cache/yum

ARG GOLANG_VERSION \
    NODE_VERSION=22.18.0

ENV PATH="/usr/local/go/bin:${PATH}"

# Amplify only runs on x86_64 Linux, so we can safely install the official Node and Go binaries for that architecture.
RUN curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.gz" | \
      tar -C /usr/local --strip-components=1 -xz
RUN curl -fsSL "https://go.dev/dl/${GOLANG_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz

# TODO: Consider making these versions configurable build args sourced from
# docs-website so this image can track docs-website runtime requirements.
RUN npm install -g yarn@1.22.22 && \
    node --version && \
    npm --version && \
    yarn --version && \
    go version && \
    jq --version && \
    aws --version && \
    curl --version && \
    git --version && \
    sudo --version && \
    ssh -V && \
    bash --version && \
    command -v sh && \
    wget --version && \
    tar --version

# Amplify docs build expects DOCS_CONTENT_DIR environment variable to be set 
# to the location of the docs content in the image.
ENV DOCS_CONTENT_DIR="/opt/docs-content"

COPY docs-content/ ${DOCS_CONTENT_DIR}/

# Include build metadata as build args so that we can use them in the image for debugging and logging purposes.
# This goes at the end of the Dockerfile so that we can leverage Docker's caching mechanism.
ARG BUILD_METADATA_REF \
    BUILD_METADATA_REPO \
    BUILD_METADATA_RUN_ID \
    BUILD_METADATA_SHA \
    BUILD_METADATA_WORKFLOW_SHA \
    BUILD_METADATA_WORKFLOW

ENV BUILD_METADATA_REF=${BUILD_METADATA_REF} \
    BUILD_METADATA_REPO=${BUILD_METADATA_REPO} \
    BUILD_METADATA_RUN_ID=${BUILD_METADATA_RUN_ID} \
    BUILD_METADATA_SHA=${BUILD_METADATA_SHA} \
    BUILD_METADATA_WORKFLOW_SHA=${BUILD_METADATA_WORKFLOW_SHA} \
    BUILD_METADATA_WORKFLOW=${BUILD_METADATA_WORKFLOW}
