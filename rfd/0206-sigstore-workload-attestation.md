---
authors: Dan Upton (daniel.upton@goteleport.com)
state: implemented (17.5)
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
problems in supply chain security, including key management and traceability.

It can be used to sign artifacts such as container images and binaries, as well
as attestations such as SBOMs and SLSA provenance documents.

Sigstore is the core technology behind:

* [GitHub artifact attestations](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations)
* [npm provenance statements](https://docs.npmjs.com/generating-provenance-statements)
* [Homebrew build provenance](https://blog.trailofbits.com/2023/11/06/adding-build-provenance-to-homebrew/)

By integrating with Sigstore, we can enable users to "lock down" workload
identities to only be used by trusted software components; such that if a bad
actor were able to get malicious code into production, it would simply not be
allowed to communicate with other systems, access databases, etc. thereby
reducing the scope for lateral movement.

## Details

### Context: Sigstore 101

In addition to the [specs](https://github.com/sigstore/cosign/tree/main/specs)
that establish how signatures should be stored and discovered, the Sigstore
project comprises three main components:

- [Cosign](https://github.com/sigstore/cosign) (signing CLI)
- [Fulcio](https://github.com/sigstore/fulcio) (certificate authority)
- [Rekor](https://github.com/sigstore/rekor) (transparency log)

These components and their roles are discussed in detail below.

#### Key Management

Securely distributing and managing the lifecycle of signing keys is one of the
hardest problems to solve when designing a code signing solution. They're
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
identity token (i.e. your GitHub Actions OIDC token) as a command line flag.

On the receiving end, users can verify image signatures using the Cosign CLI
like so:

```shell
$ cosign verify <image>@sha256:<hash> \
  --certificate-identity daniel.upton@goteleport.com \
  --certificate-oidc-issuer https://accounts.google.com
```

There is also a Kubernetes [admission controller](https://docs.sigstore.dev/policy-controller/overview/)
that can be used to only allow images signed by trusted authorities to run in
your cluster. While this is conceptually similar to the feature described by
this RFD, we think of the two as complementary rather than competing, as using
both would allow for a "defense in depth" approach and because organizations may
have multiple Kubernetes clusters operated by different groups.

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

Sigstore stores signatures and attestations in an OCI registry, usually (but not
always) the same registry as the container image itself.

`tbot` will be responsible for finding the image's signatures and attestations.
This is primarily because granting the auth server access to a registry could be
challenging, particularly when using Teleport Cloud and a registry that requires
a credential helper process (e.g. [docker-credential-gcr](https://github.com/GoogleCloudPlatform/docker-credential-gcr)).

By default, `tbot` will look in the same registry and repository as the image,
but users will be able to configure additional registries. For now, we'll make
the simplifying assumption that the repository name will be the same across
registries.

As there are a number of ways to [configure authentication](https://pkg.go.dev/github.com/google/go-containerregistry/pkg/authn#section-readme)
in Docker and Podman, we won't attempt to reproduce them in the `tbot` config
file. Instead, we'll use go-containerregistry's `authn` package, which supports
loading a Docker or Podman configuration file, and allow the user to supply a
path to an existing configuration file.

Given the image name (and therefore the registry) is user-controlled, it would
technically be possible to use `tbot` to mount an SSRF attack against private
systems. To mitigate this, we'll refuse to connect to registries at private IPs
by default, and allow the user to specify a list of CIDR blocks to override this
if their registry is not publicly-routable.

```yaml
services:
  - type: workload-identity-api
    listen: unix:///tmp/tbot.sock
    selector:
      name: my-workload-identity
    attestors:
      sigstore:
        enabled: true
        additional_registries:
          - host: ghcr.io
          - host: public.ecr.aws
          - host: localhost:1234
        credentials_path: /path/to/docker/config.json
        allowed_network_prefixes:
          - "192.168.1.42/32"
    # docker:
    #   enabled: true
```

### `SigstorePolicy` Resource

Users will create policies that determine which signing identities or keys to
trust, whether artifact signatures or attestations are required, and details of
their private Fulcio and Rekor instances.

Here's an example of a policy which requires the image to have been attested by
a GitHub Actions runner in any of `mycompany`'s repositories with SLSA
provenance (i.e. using `actions/attest-build-provenance`):

```yaml
kind: sigstore_policy
version: v1
metadata:
  name: github-provenance
spec:
  keyless:
    identities:
      - issuer: https://token.actions.githubusercontent.com
        subject_regex: https://github.com/mycompany/*/.github/workflows/*@*
  requirements:
    attestations:
      - predicate_type: https://slsa.dev/provenance/v1
```

Here's an example of a policy which requires the image to have been signed using
a known trusted keypair:

```yaml
kind: sigstore_policy
version: v1
metadata:
  name: trusted-keypair
spec:
  key:
    public: |
      -----BEGIN PUBLIC KEY-----
      -----END PUBLIC KEY-----
  requirements:
    artifact_signature: true
```

When using a self-hosted Fulcio, Rekor, or timestamp authority instance; users
can supply a set of JSON-formatted ["trusted root"](https://github.com/sigstore/protobuf-specs/blob/cac7a926e0968571d3eb2e2fc8ebd40b8ebe0d58/protos/sigstore_trustroot.proto#L92-L144)
documents which include all of the information required to verify signatures
using their private infrastructure (e.g. certificate chains, validity periods).

```yaml
kind: sigstore_policy
version: v1
metadata:
  name: github-provenance
spec:
  keyless:
    identities:
      - issuer: https://token.actions.githubusercontent.com
        subject_regex: https://github.com/mycompany/*/.github/workflows/*@*
    trusted_roots:
      - |
        {
          "mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
          ...
  requirements:
    artifact_signature: true
```

This was chosen over designing our own format because it handles a lot of the
complexity around certificate rotation, etc. It's also straightforward to export
trusted roots [using the GitHub CLI](https://cli.github.com/manual/gh_attestation_trusted-root).

```shell
$ gh attestation trusted-root | jq .
{
  "mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
  "tlogs": [
    {
      "baseUrl": "https://rekor.sigstore.dev",
...
```

In the future, we'll support using a custom [TUF](https://theupdateframework.io/)
repository to keep your root of trust up-to-date automatically.

### `WorkloadIdentity` Resource Changes

In order to require a policy is satisfied before issuing a workload identity,
users will be able to refer to policies in the `WorkloadIdentity` resource's
`rules` section.

```yaml
rules:
  allow:
    - expression: sigstore.policy_satisfied("github-provenance")
```

For efficiency, we'll parse the expression ahead of time to determine which
policies to load and evaluate, rather than evaluating all policies.

## Implementation

The implementation will be split into two main parts: discovering signatures and
attestations (`tbot`-side), and verifying signatures and enforcing policies
(server-side).

### Bundles

The Sigstore project recently introduced the concept of a ["bundle"](https://docs.sigstore.dev/about/bundle/)
as a container for all of the information required to verify a signature or
attestation.

Soon, bundles will be the standard way to store and transmit signatures, so we'll
design our API around them. However, they're not fully supported in `cosign` yet,
so we'll need to handle the ["old way"](https://github.com/sigstore/cosign/blob/main/specs/SIGNATURE_SPEC.md)
based on Red Hat's [simple signing envelope](https://www.redhat.com/en/blog/container-image-signing)
too.

Thankfully, the `sigstore-go` project has [an example](https://github.com/sigstore/sigstore-go/tree/main/examples/oci-image-verification)
of manually constructing a bundle from the old format.

`tbot` will discover bundles and old-style signatures as [discussed below](#signature-and-attestation-discovery)
and attach them to the workload attributes sent in `Issue*WorkloadIdentity` RPCs.

Here is the protobuf format the bundles will be sent as:

```proto
message SigstoreVerificationPayload {
  // Sigstore bundle serialized in the protobuf encoding.
  bytes bundle = 1;

  // When the bundle was constructed by `tbot` from the old-style annotations
  // the enclosed signature will be over the simple signing envelope, not the
  // actual image manifest.
  //
  // Signature = Sign(SHA-256(SimpleSigningEnvelope(SHA-256(Image Manifest))))
  //
  // In that case, `tbot` will include the simple signing envelope, which the
  // server will hash with SHA-256 and check the signature. The server will also
  // compare the `critical.docker-manifest-digest` to the image digest produced
  // by the Podman, Docker, or Kubernetes attestor.
  //
  // When simple_signing_envelope is not provided, the server will assert the
  // bundle contains an in-toto attestation, enclosed with DSSE, where the
  // subject matches the image digest from the Podman, Docker, or Kubernetes
  // attestor.
  optional bytes simple_signing_envelope = 2;
}
```

### Signature and Attestation Discovery

There are a number of different ways signatures and attestations are stored in
an OCI registry.

For old-style image signatures, the simple signature envelope is uploaded as a
blob of type `application/vnd.dev.sigstore.bundle.v0.3+json` which is then added
as a layer in an image manifest. The signature, certificate, etc. are added as
annotations on the layer. The manifest is then tagged with `sha256-<hash>.sig`
for discovery.

`cosign` currently stores in-toto attestations in a similar way (except tagged
with `sha256-<hash>.att`) unless you explicitly opt-in to the new bundle format.

GitHub provenance attestations use the new bundle format, where attestations are
linked back to the image using the `subject` field and Referrers API. For
registries that don't support the Referrers API, you can discover signatures
using the ["tag schema"](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#referrers-tag-schema)
where clients manually maintain an image index with pointers to all of the
linked manifests.

Here's the discovery process `tbot` will follow, for each of the configured
registries:

1. Fetch `sha256-<hash>.sig`
2. Pull each layer where the `mediaType` is a simple signing envelope and
   construct a bundle from its annotations
3. Call the `/v2/<name>/referrers/<digest>` API
  a. If it returns `404` the registry doesn't support the Referrers API so fall
     back to the tag schema
4. Pull each layer found in step 3 where the `mediaType` is a bundle

For now, we won't pull the old-style attestations (i.e. `sha256-<hash>.att`)
because they don't appear to be widely used, and the new bundle format will
shortly replace them.

As this is a relatively expensive process involving multiple HTTP requests,
`tbot` will cache the discovered bundles and only refresh them if obtaining a
workload identity fails.

### Policy Enforcement

On the server side, when considering whether to issue a SPIFFE SVID we will
check to see if any of the workload identity's `rules` uses the
`sigstore.policy_satisfied` function. If so, we will load each of the named
policies and evaluate them against the presented bundles (using sigstore-go's
`verify` package).

Whether or not the workload identity is issued depends on the rule's expression
(as users may require multiple policies to pass using `&&` or `||` operators),
but the return value of `sigstore.policy_satisfied(name)` will be `true` for that
policy if **any** of the bundles were successfully verified.

### Audit Logs

The event emitted when a workload identity is issued (or fails to be issued)
contains the attributes used to make the decision, including those from `tbot`s
workload attestors. Rather than including the raw Sigstore bundles in these
events (which are quite large and of little value in this context) we will
replace them with the policy outcomes.

```json
{
  "attributes": {
    "workload": {
      "podman": {
        "container": {
          "name": "foo"
        }
      },
      "sigstore": {
        "policies": {
          "github-provenance": {
            "pass": true
          },
          "sbom": {
            "pass": false,
            "reason": "No attestation with predicate 'https://spdx.dev/Document' found."
          }
        }
      }
    }
  }
}
```

### Security Considerations

One obvious downside of this approach is that it relies on trusting the image
digest presented in `Issue*WorkloadIdentity` RPCs, which could be falsified if
an attacker were to steal a bot's credentials.

This isn't really any different to the other `workload` attributes, and is
in-fact a very slight improvement because signatures are verified on the server
side, so an attacker must present a signature that matches the image digest.

## Future Work

### Non-Container Artifacts

While this RFD focuses on integrating Sigstore with our container-based workload
attestors (i.e. Kubernetes, Docker, Podman), it's possible to sign other types
of artifacts such as binaries, `.jar`, and `.wasm` files too. Now that our Unix
attestor is [able to](https://github.com/gravitational/teleport/pull/52736)
capture the SHA-256 hash of a running process' executable, it would be
straightforward to discover signatures and attestations for the binary using
this hash.

### Notary v2

There are a number of other projects in the code signing space with varying
levels of adoption. One such project with backing from Docker, Microsoft, and
AWS is [Notary](https://notaryproject.dev), and we should consider a similar
integration with it.

### CUE and Rego Policies

Lots of tools in the Sigstore ecosystem support using CUE or Rego to make
assertions on attestations (e.g. checking the contents of a provenance document)
so we should consider adding this too if there's user demand.

### KMS Integration

Cosign support signing and verifying using a KMS-managed key. We've avoided it
in this RFD to keep scope small, which is acceptable because we only need the
public key. In the future, it might be worth exploring matching cosign's support
for reading the public key from a KMS.

### Custom TUF Repositories

Companies operating their own Sigstore infrastructure will likely bootstrap
their root of trust using [TUF](https://theupdateframework.io/), similar to the
public good instance.

For simplicity, we chose not to explore an integration with custom TUF
repositories in this RFD. Instead, users can manually supply a "trusted root"
document directly in their `SigstorePolicy` resources, which feels like an
acceptable trade-off until there's clear customer demand for TUF support.

## Abandoned Ideas

### Using Rekor for Discovery

As signatures are recorded in Rekor's transparency log, it's technically
possible to use its API for signature discovery (e.g. if you do not have access
to the OCI repository).

However, the `/api/v1/index/retrieve` endpoint is marked as deprecated and can
return incomplete results.

### No `SigstorePolicy` Resource

We considered adding Sigstore policies directly to the `WorkloadIdentity`
resource. As well as the benefits of being able to reuse a policy between
workload identities, we abandoned this idea because it made the relationship
between policies and the `rules` ambiguous.

### Verifying Signatures On The `tbot` Side

All of the `workload` attributes produced by `tbot`s attestors are treated as
less trustworthy than the `user` or `join` attributes because of the possibility
of `tbot` being compromised. As this includes the image digest, we could avoid
the overhead of sending bundles to the server by verifying signatures on the
client side, without meaningfully changing the security posture.

We chose not to pursue this because it's more manageable to centralize the
configuration of policies and trusted signing authorities on the server.
