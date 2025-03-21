---
authors: Dan Upton (daniel.upton@goteleport.com)
state: draft
---

# RFD 206 - Sigstore Workload Attestation

## Required Approvers

* Engineering: @strideynet && @zmb3
* Product: @thedevelopnik

## What

Building supply chain security into our workload attestation process by
integrating with the [Sigstore](https://sigstore.dev) project.

## Why

With supply chain attacks becoming a pervasive security threat to critical
systems, there's a growing movement towards cryptographically signing software
artifacts to prove their integrity, tracking dependencies using SBOMs, and
auditing software build processes.

Sigstore is a framework and suite of tools that solve some of the hardest
problems in signing artifacts, including key management and traceability.

It can be used to sign artifacts such as container images and binaries, as well
as attestations such as SBOMs and SLSA provenance documents.

Sigstore is the core technology behind:

* [GitHub artifact attestations](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations)
* [npm provenance statements](https://docs.npmjs.com/generating-provenance-statements)
* [Homebrew build provenance](https://blog.trailofbits.com/2023/11/06/adding-build-provenance-to-homebrew/)

By integrating with Sigstore, we can enable users to "lock down" workload
identities to only be used by trusted software components; such that if a bad
actor were able to get malicious code into production, it would simply not be
allowed to communicate with other services, access databases, etc. reducing the
scope for lateral movement.

## Details

### Context: Sigstore 101

In addition to the [specs](https://github.com/sigstore/cosign/tree/main/specs)
that establish how signatures should be stored and discovered, the Sigstore
project comprises three main components:

- [Cosign](https://github.com/sigstore/cosign)
- [Fulcio](https://github.com/sigstore/fulcio)
- [Rekor](https://github.com/sigstore/rekor)

These components and their roles are discussed in detail below.

#### Key Management

Securely distributing and managing the lifecycle of signing keys is one of the
hardest problems to solve when designing a software signing solution. They're
essentially highly sensitive and long-lived secrets, which are problematic for
reasons we're all familiar with!

Sigstore takes a novel approach to this problem by binding signatures to an OIDC
**identity** rather than a long-lived key. It does so by running a certificate
authority called Fulcio, which exchanges an OIDC identity token for a
short-lived signing certificate which is used once and then immediately
discarded.

When using the Cosign CLI to sign an image locally, the user simply runs:

```shell
$ cosign sign <image>@sha256:<hash>
```

Which opens a browser window for them to log in with their Google, Microsoft, or
GitHub account, signs the artifact, and uploads the signature to the container
registry.

When running `cosign` non-interactively in CI platforms, etc. you can pass the
identity token (i.e. their GitHub Actions OIDC token) as a command line flag.

On the receiving end, users can verify image signatures using the Cosign CLI
like so:

```shell
$ cosign verify <image>@sha256:<hash> \
  --certificate-identity daniel.upton@goteleport.com \
  --certificate-oidc-issuer https://accounts.google.com
```

There is also a Kubernetes [admission controller](https://docs.sigstore.dev/policy-controller/overview/)
that can be used to only allow images signed by trusted authorities to run in
your cluster. This is conceptually very similar to the feature described by this
RFD.

#### Transparency Log

Cosign also records the signature in Rekor, the signature transparency log. This
solves a number of problems, including:

1. Because the certificates Fulcio issues are so short-lived (default 10 minutes)
   they will almost certainly have expired by the time the artifact signature is
   verified. As such, you need another way to prove the certificate was valid at
   the time the signature was generated. Rekor solves this by, in essence,
   timestamping the signature in its transparency log.

2. In the same way that a certificate transparency log allows you to monitor for
   abuse of your organization's domains, and discover if the CA itself has been
   compromised; the signature transparency log enables you to find out if an
   unauthorized party is signing artifacts on your behalf, or if the Sigstore
   infrastructure has been compromised.

You can also opt to counter-sign the signature using your own RFC 3161 timestamp
authority by passing the `--timestamp-server-url` flag.

#### Public Good Infrastructure

The Sigstore community operates a centralized "public good" instance of Fulcio
and Rekor, which Cosign uses by default. Although it's convenient, it may not be
appropriate for organizations which need tighter control over the cryptographic
material (e.g. for regulatory reasons) or do not want their artifact signatures
to be recorded in a public transparency log.

It's possible to run your own instances of Fulcio and Rekor, and establish your
own [root of trust](https://blog.sigstore.dev/sigstore-bring-your-own-stuf-with-tuf-40febfd2badd/).

You can also opt to use a self-managed signing key (on disk or via a KMS) and/or
disable the transparency log entirely, but this forgoes a lot of the benefits of
using Sigstore.

GitHub operates an internal instance of Fulcio, as well as an RFC 3161 timestamp
authority which is used in-lieu of Rekor when you use the `actions/attest`
action from a private repository.

## User Experience

Using Cosign for the first time feels like magic because of its sensible and
secure defaults. I think we can replicate this low barrier to entry in our
integration.

In the simplest case (using the "public good" instance and public container
images), users will simply need to enable the sigstore integration, configure
their trusted signing identities, and add a rule to their `WorkloadIdentity`
resources to require valid signatures or attestations.

Then, whenever a Kubernetes, Docker, or Podman-based workload requests a SVID
from `tbot`s SPIFFE workload identity API, we'll resolve the signatures and
attestations for the workload's container image and verify them before issuing
the SVID.

In more complex cases, users may need to configure their private Rekor and
Fulcio instance addresses, trusted public keys, timestamp authorities, or
registry credentials. We can take a lot of inspiration from the Sigstore
[policy-controller](https://docs.sigstore.dev/policy-controller/overview/)'s
configuration here.

### `tbot` Configuration

```yaml
```

### `SigstorePolicy` Resource

### `WorkloadIdentity` Resource Changes

## Protobuf Schema

## Implementation

### Signature and Attestation Discovery

## Future Work

### Non-container

## Abandoned Ideas
