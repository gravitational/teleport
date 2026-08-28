# Dockerized Teleport Build

This directory is used to produce a containerized production Teleport build.
No need to have Golang. Only Docker is required.

It is a part of Gravitational CI/CD pipeline. To build Teleport type:

```
make
```

# Safely updating build box Dockerfiles

The build box images are used in GitHub Actions. The resulting image is pushed
to ghcr.io. This means that to safely introduce changes to Dockerfiles, those changes
should be split into two stages:

1. First you open a PR which updates a Dockerfile and get the PR merged.
2. Once it's merged, GitHub Actions is going to pick it up, build a new build box
   image and push it to ghcr.io
3. Then you can open another PR which starts using the new build box image.

# DynamoDB static binary docker build

The static binary will be built along with all nodejs assets inside the container.
From the root directory of the source checkout run:

```
docker build -f build.assets/Dockerfile.dynamodb -t teleportbuilder .
```

Then you can upload the result to an S3 bucket for release.

```
docker run -it -e AWS_ACL=public-read -e S3_BUCKET=my-teleport-releases -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY teleportbuilder
```

Or simply copy the binary out of the image using a volume (it will be copied to current directory/build/teleport).

```
docker run -v $(pwd)/build:/builds -it teleportbuilder cp /gopath/src/github.com/gravitational/teleport/teleport.tgz /builds
```
