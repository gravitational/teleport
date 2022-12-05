---
authors: Trent Clarke (trent@goteleport.com)
state: draft
---

# RFD 0098 - Delivering secure Teleport OCI Images

# Required Approvers
* Engineering: @r0mant
* Security: @reed
* Product: (@xinding33 || @klizhentas)

## What

This RFD discusses structures and processes to increase the security of the OCI 
images we use to deliver Teleport to our customers.

## Why

One of our shipping channels is a Docker image. As the provider of that docker
image we are (at least partialy) responsible for everything in it, not just
Teleport. We should not ship vulnerabilities to our clients, even if those 
vulnerabilities are not directly in our software.

## Details

### Goals

For the sake of this RFD, I will define _delivering a secure image_ as 

> Reliably producing an OCI image that:
>
>
>  1. a priori is unlikely to contain vulnerabilities in and of itself, 
>  2. contains software of known provenance,
>  3. has no warnings or vulnerabilities flagged when run through a reputable 
>     scanner at the time of creation, and
>  4. flags vulnerabilities even after creation, either in Teleport itself or 
>     a dependency.

### Non-goals

This RFD will not discuss building images for and/or running Teleport on
platforms other than Linux.

### Approach

This RFD describes a 2-pronged approach for meeting the above goals:

   1. Switching to distroless base images, and 
   2. Automated, ongoing monitoring of images using a scanning service

### 1. Distroless Images

Distroless images contain only an application and the minimal set of
dependencies for it. Google offers several base images that contain minimal 
Linux distribution that we can use as a starting point.

Switching to distroless images drastically reduces the number of software components 
we ship as part of a Teleport distribution. This both reduces the size of the potential
attack surface, and reduces the potential for high-noise reports from automated
scanners. 

The Google-supplied distroless base images also [provide a mechanism for verifying the
provenance of a given image](https://github.com/GoogleContainerTools/distroless#how-do-i-verify-distroless-images) 
using `cosign` and a public key. Stronger, SLSA-2 level guarantees [can be verified with additional
tooling](https://security.googleblog.com/2021/09/distroless-builds-are-now-slsa-2.html). 

**Note** that we are _already_  using Distroless images to distribute some Teleport 
plugins. This would extend their use to Teleport proper.

### 2. Ongoing scanning

Using an _ongoing_ automated scanner means that we do not just check for 
vulnerabilities at image creation time, instead we _continually_ scan for any vulnerabilities 
that may be discovered during the image's lifetime.

## Implementation Details

## 1. Image construction

### Teleport Image Requirements

What is the minimal set of requirements to run Teleport on Linux in a container?

   1. Teleport
   2. `GLIBC` >= 2.17
   3. `dumb-init` is required for correct signal and child processes handling
      inside a container.
   4. CA certificates

Requirement (1) (i.e. Teleport itself) is provided by our CI process. 

Requirements (2) and (4) are satisfied automatically by using the the google-
provided base image [`gcr.io/distroless/cc-debian11`](https://github.com/GoogleContainerTools/distroless#what-images-are-available), which is configured for "mostly statically compiled" languages that require libc.

Requirement (3) (`dumb-init`) can be sourced either from the upstream Debian repository, or
downloaded directly from the project's GitHub release page. Sourcing `dumb-init` from the 
Ubuntu or Debian package repositories implies some minimal provenance checking by the debian
packaging tools, so we will prefer that to sourcing it from GitHub.

### Base image verification

The distroless base image will be pulled and verified prior to constructing the
Teleport image, using the `cosign` tool as described [here](https://github.com/GoogleContainerTools/distroless#how-do-i-verify-distroless-images).

This will allow us to specify a floating tag for the base image (and thus
automatically include the latest version of the base image, with any security
fixes, etc. included) while still validating the provenance of the base image
itself.

It is technically possible for the image to be poisoned post-validation (e.g.
in a shared build environment, where a malicious peer could re-tag a malicious
image as the base).

While we _could_ verify that the validated base image appears in the parent 
chain of the final image, this is still no protection against the malicious 
image being based on the same parent. We could _also_ assert that the final 
image's parent chain has an expecetd number of steps (inferred from the number
of steps in the Docker file), but this is erorr-prone and would drastically 
reduce the flexibility of the build system.

For the above reasons, Teleport images for public consumption must not be 
built in such a shared environment.

### Building the image

The image will be built from a multi-stage docker file, using build stages to download
and unpack the required debian packages and copy them into place on the distroless 
image. 

An example Dockerfile, assuming the Teleport Debian package is supplied by the 
CI system, might look like something like:

```Docker
FROM debian:11 as dumb-init
RUN apt update && apt-get download dumb-init && dpkg-deb -R dumb-init*.deb /opt/dumb-init

FROM debian:11 as teleport
COPY teleport*.deb
RUN dpkg-deb -R teleport*.deb /opt/teleport

FROM gcr.io/distroless/cc-debian11
COPY --from=dumb-init /opt/dumb-init/bin/dumb-init /bin
COPY --from=teleport /opt/teleport/bin/* /bin
ENTRYPOINT ["/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml"]
```

### Troubleshooting Images

Troubleshooting a distroless image is hard, as there are no tools baked into
the image to aid in debugging a deployment.

If we need to add tooling in order to aid troubleshooting, I propose that we
add a `teleport-debug` image, that builds on the distroless image to include
things like a shell, tools from `busybox`, and so on. I would also suggest 
that, while we should take as much care as possible when constructing and 
monitoring this image, use of the debug image is considered "at your own risk".

### Compatibility Guarantees

We have clients relying on the existing behaviour (and contents) of our images. We
should treat releaseing these distroless images as a compatability break, and make 
our customers aware of our intentions well in advance so that they can prepare.

## 2. Scanning and monitoring the image

There are many options for scanning and monitoring, but given we are already using the
Amazon ECR, it seems most logical to use the built-in ECR scanning tools to detect
known vulnerabilities in the final images.

We are, in fact, already doing this on image upload, although I don't know if
the signal from the scan is monitored in any way.

I propose that we use the ECR Advanced Scanning tools, that can be set to run at
intervals. It is also possible to attach actions to the result of these scans.
Initially, I suggest a simple Slack channel notification on any detected
vulnerability, as shown (here)[https://www.kostavro.eu/posts/2021-05-26-ecr-scan-on-push-slack-notification/]. 

(The example is for scan-on-push, but it should be amenable to extension to 
intermittent, ongoing scans.)
