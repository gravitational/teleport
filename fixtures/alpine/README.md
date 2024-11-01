# Alpine Docker Image

The image located at `fixtures/alpine/alpine-3.20.3.amd64.tar` is a vendored copy of `docker.io/library/alpine:3.20.3`.

This image is vendored because its download process often fails, which disrupts our Kubernetes Integration Tests.

## Generating the Image

To create the tar file, use the following command:

```
$ docker save -o alpine-3.20.3.amd64.tar docker.io/library/alpine:3.20.3
```

## Dockerfile

The Dockerfile copies a pre-compiled version of `webserver.go` into the image and sets the default command to `/webserver`.