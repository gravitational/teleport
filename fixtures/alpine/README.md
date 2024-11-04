# `alpine-webserver:v1` Build Process

## Source

The `alpine-webserver:v1` image is based on `alpine` `minirootfs`, but instead of relying on Docker Hub's official Alpine image, we source the original files directly from Alpine's CDN. This approach mitigates issues with Docker Hub and GitHub Action network failures, which have been a common cause of integration test failures.

The build process is specified in the `.github/workflows/kube-integration-tests-non-root.yaml` file.

## Download

To download the new `alpine-minirootfs` image, follow the instructions:

```bash
$ export ALPINE_VERSION=3.20.3
$ export SHORT_VERSION=${ALPINE_VERSION%.*}

# download alpine minirootfs and signature
$ curl -fSsLO https://dl-cdn.alpinelinux.org/alpine/v$SHORT_VERSION/releases/x86_64/alpine-minirootfs-$ALPINE_VERSION-x86_64.tar.gz
$ curl -fSsLO https://dl-cdn.alpinelinux.org/alpine/v$SHORT_VERSION/releases/x86_64/alpine-minirootfs-$ALPINE_VERSION-x86_64.tar.gz.asc
$ curl -fSsLO https://dl-cdn.alpinelinux.org/alpine/v$SHORT_VERSION/releases/x86_64/alpine-minirootfs-$ALPINE_VERSION-x86_64.tar.gz.sha256
          
```

## Source Validation

The build process in `.github/workflows/kube-integration-tests-non-root.yaml` validates both the SHA-256 checksum and the GPG signature. The signature verification uses `alpine-ncopa.at.alpinelinux.org.asc`, which is the official public key used by Alpine Linux to sign its assets. This public key is available on the [Alpine Linux Downloads page](https://www.alpinelinux.org/downloads/).

## Image Build Process

The image is constructed from a scratch filesystem and incorporates only the necessary components to run the web server. Here’s the basic Dockerfile configuration:

```Dockerfile
FROM scratch

ARG ALPINE_VERSION

ADD alpine-minirootfs-$ALPINE_VERSION-x86_64.tar.gz /

COPY webserver /webserver

CMD [ "/webserver" ]
```

This minimalist configuration ensures the image remains lightweight, secure, and tailored to only the required functionalities.

## Image distribution

After sucessfull build, the image is loaded into our `kind` cluster with the tag `alpine-webserver:v1`.

Note: `:latest` can't be used otherwise Kubernetes will try loading the image from dockerhub and fail.