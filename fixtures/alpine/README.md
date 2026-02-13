# `alpine-webserver:v1`

## Why

We build a custom image for Kubernetes integration tests so that we don't have to 
rely on external dependencies in CI. This approach mitigates issues with Docker Hub 
and GitHub Actions network failures which have been a common cause of integration test failures
in the past.

## How

The `make build-image` performs the following steps to produce the image.

We download `alpine` `minirootfs` assets directly from the alpine CDN, compile the webserver,
and build a minimal docker image.

The build process validates both the SHA-256 checksum and the GPG signature. The signature verification 
uses `alpine-ncopa.at.alpinelinux.org.asc`, which is the official public key used by Alpine Linux to 
sign its assets. This public key is available on the [Alpine Linux Downloads page](https://www.alpinelinux.org/downloads/).

The docker image produced is tagged with `:v1` instead of `:latest` to prevent Kubernetes from 
pulling the image from Docker Hub in an attempt to ensure the most recent image is used.

After a successful build, the image can be loaded into a `kind` cluster via `make load-image`.