---
authors: Noah Stride <noah@goteleport.com>
state: draft
---

# RFD 79 - OIDC JWT Joining

## Required Approvers

* Engineering @zmb3 && @nklaassen
* Security @reedloden
* Product: (@xinding33 || @klizhentas)

## Terminology

- OIDC: OpenID Connect. A federated authentication protocol built on top of OAuth 2.0.
- OP: OpenID Provider.
- Issuer: an OP that has issued a specific token.
- Workload Identity: an identity corresponding to a running workload, such as a container or VM. This is in contrast to the more well-known use of OAuth in identifying users. A workload identity token is often made available to a workload running on a platform via a metadata server or an environment variable.

## What

Teleport should support trusting a third party OP and the JWTs that it issues when authenticating a client for the cluster join process. This is similar to the support for IAM joining, and will allow joining a Teleport cluster without the need to distribute a token on several platforms.

Users will need to be able to configure trust in an OP, and rules that determine what identities are allowed to join the cluster.

## Why

This feature reduces the friction involved in adding many new nodes to Teleport on a variety of platforms. This is also more secure, as the user does not need to distribute a token which is liable to exfilitration.

Whilst multiple providers offer OIDC identities to workloads running on their platform, we will start by targetting GCP GCE since this represents a large portion of the market. However, the work towards this feature will also enable us to simply add other providers that support OIDC such as:

- GitHub Actions: a key platform for growing usage of Machine ID.
- GitLab CI/CD
- CircleCI
- GCP GCB

## Details

The work on OIDC joining is broken down into two parts:

- Support in the Auth server for trusting an OP issued JWT. This work is general, and will be applicable to all providers.
- Support in nodes for fetching their workload identity token from their environment. This work will be specific to each platform we intend to support.

OIDC supports multiple types of token (`id_token`: a JWT encoding information about the identity, which can be verified using the issuer's public key and `access_token`: an opaque token that can be used with a `userinfo` endpoint on the issuer to obtain information about the identity). However, in the case of workload identities, `id_token` is the most prevelant. For this reason, our initial implementation will solely support `id_token`.

### Auth server support

#### Configuration

We will leverage the existing Token configuration kind as used by static tokens and IAM joining:

```yaml
kind: token
version: v2
metadata:
  name: my-gce-token
  expires: "3000-01-01T00:00:00Z"
spec:
  roles: [Node]
  join_method: oidc-jwt
  issuer_url: https://accounts.google.com
  allow: claims.google.compute_engine.project_id == "my-project" && claims.google.compute_engine.instance_name == "an-instance"
```

To allow the user to configure rules for what identities will be accepted, we will use the [Common Expression Language (CEL)](https://github.com/google/cel-spec). This allows a large degree of flexibility in the complexity of rules users can configure, but still allows simple expressions.

Users must also configure the `issuer_url`. This must be a host on which there is a compliant `/.well-known/openid-configuration` endpoint.

### Node support

Node here not only refers to a Teleport node, but also to a `tbot` instance.

### Security Considerations

#### Ease of misconfiguration

It is possible for users to create a trust configuration that would allow a malicious actor to craft an identity that would be able to join their cluster.

For example, if the trust was configured with an allow rule such as:

```go
claims.google.compute_engine.instance_name == "an-instance"
```

Then a malicious actor would be able to create their own GCP project, create an instance with that name, and use it to obtain a jwt that would be trusted by Teleport.

It is imperative that users include some rule that filters it to their own project e.g:

```go
claims.google.compute_engine.project_id == "my-project" && claims.google.compute_engine.instance_name == "an-instance"
```

or that they restrict it to a specific subject (as this will be unique to the issuer):

```go
claims.sub == "777666555444333222111"
```

#### MITM of the connection to the issuer

In order to verify the JWT, we have to fetch the public key from the issuers's JWKS endpoint. If this connection is not secured (e.g HTTPS), it would be possible for a malicious actor to intercept the request and return a public key they've used to sign the JWT with.

We should require that the configured issuer URL is HTTPS to mitigate this.

## References and Resources

Providers of Workload Identity. These are platforms we can support once OIDC joining is added:

- [Github Actions](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect)
- GCP (GCE, GCB)
- [CircleCI](https://circleci.com/docs/openid-connect-tokens)
- [GitLab](https://docs.gitlab.com/ee/ci/cloud_services/)
- [SPIFFE/SPIRE](https://spiffe.io/docs/latest/keyless/): This presents an interesting use case for tokenless joining in a variety of environment.

Similar implementations:

- [GCP Workload identity federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [HashiCorp Vault](https://www.vaultproject.io/docs/auth/jwt)
- [AWS IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html)
- [Azure AD](https://docs.microsoft.com/en-us/azure/active-directory/develop/workload-identity-federation)