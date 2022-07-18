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

Teleport should support trusting a third party OP and the JWTs that it issues when authenticating a client for the cluster join process. This is similar to the support for AWS IAM joining, and will allow joining a Teleport cluster without the need to distribute a static token on several platforms.

Users will need to be able to configure trust in an OP, and rules that determine what identities are allowed to join the cluster.

## Why

This feature reduces the friction involved in dynamically adding many new nodes to Teleport on a variety of platforms. This is also more secure, as the user does not need to distribute a token which is liable to exfilitration.

Whilst multiple providers offer OIDC identities to workloads running on their platform, we will start by targetting GCP GCE since this represents a key platform for Teleport, and is also well documented and easy to test on. However, the work towards this feature will also enable us to simply add other providers that support OIDC workload identity (see the references for more), this particularly ties into Machine ID goals as we aim to support several CI/CD providers that offer workload identity.

## Details

The work on OIDC joining is broken down into two parts:

- Support in the Auth server for trusting an OP issued JWT. This work is general, and will be applicable to all providers.
- Support in nodes for fetching their workload identity token from their environment. This work will be specific to each platform we intend to support.

OIDC supports multiple types of token (`id_token`: a JWT encoding information about the identity, which can be verified using the issuer's public key and `access_token`: an opaque token that can be used with a `userinfo` endpoint on the issuer to obtain information about the identity). However, in the case of workload identities, `id_token` is the most prevelant. For this reason, our initial implementation will solely support `id_token`.

### Auth server support

We will re-use the existing endpoints around joining as much as possible. This means that the main entry-point for joining will be the existing `RegisterUsingToken` method.

We will introduce a new token type, `oidc-jwt`, and add an additional field to the Token resource to allow the issuer URL to be specified (`issuer_url`).

Registration flow:

1. Client is configured by the user to use `oidc-jwt` joining with a specific provider. The client then uses the provider-specific logic to obtain a token.
2. The client will call the `RegisterUsingToken` endpoint, providing the OIDC JWT token that it has collected, and specifying the name of the Teleport provisioning token which should be used to verify it.
3. The server will attempt to fetch the Token resource for the specified token.
4. The server will check JWT header to ensure the `alg` is one we have allow-listed (RS[256, 384, 512])
5. The server will check the `kid` of the JWT header, and obtain the relevant JWK from the cache or from the specified issuers well-known JWKS endpoint. It will then use the JWK to validate the token has been signed by the issuer.
6. Other key claims of the JWT will be validated:
  - Ensure the Issued At Time (iat) is in the past.
  - Ensure the Expiry Time (exp) is in the future.
7. The user's [configured Common Expression Language rule](#configuration) for the token will be evaluated against the claims, to ensure that the token is allowed to register with the Teleport cluster.
8. Certificates will be generated for the client. In the case of bot certificates, they will be treated as non-renewable, to match the behaviour of IAM registration for bot certificates.

We will re-use `go-jose@v2` for validation of JWTs, since this library is already in use within Teleport.

#### Caching JWKs

Special attention should be given to the logic around caching the JWKs.

We will cache these for two reasons:

- Improve the performance of validating JWTs, as we will not need to make a HTTPS request to the issuer.
- Improve the reliability, as we can validate JWTs even if the issuer is experiencing some downtime.
- Reduce the impact of Teleport on an issuer. If onboarding a large number of nodes, we do not want to unduly place pressure on the issuer. 

We should keep in mind the following considerations:

- When we are presented with a JWT with a previously unseen `kid`, we should re-check the issuer's JWKs, as they may have begun issuing tokens with a new JWK.
- We should ensure that the TTL of the cache entries is relatively short, as we want to allow an issuer to revoke a JWK that has been stolen.

We will implement this cache in-memory, as the data set is relatively small and it's cheap for us to repopulate this after a service restart.

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
  allow: claims.aud == "noah.teleport.sh" && claims.google.compute_engine.project_id == "my-project" && claims.google.compute_engine.instance_name == "an-instance"
```

To allow the user to configure rules for what identities will be accepted, we will use the [Common Expression Language (CEL)](https://github.com/google/cel-spec). This allows a large degree of flexibility in the complexity of rules users can configure, but still allows simple expressions.

Users must also configure the `issuer_url`. This must be a host on which there is a compliant `/.well-known/openid-configuration` endpoint.

### Node support

Node here not only refers to a Teleport node, but also to various other participants within a Teleport cluster (e.g tbot, kube agent etc).

We will need to support collecting the token from the environment. This will differ on each platform. Some offer the token via a metadata service, and others directly inject it via an environment variable. Where possible, we should encourage the user to configure the token to be generated with an audience of their Teleport cluster, however, not all providers support this (e.g GitLab CI/CD).

For GCP, a HTTP request is made to a metadata service. In this request, a query parameter controls the audience of the generated token. E.g

```
GET http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=https://noah.teleport.sh&format=full
```

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

#### Vulnerabilities in JWT and JWT signing algorithms

This section is included, since historically, there have been a large number of cases of vulnerabilities introduced into software because of misunderstandings and mistakes in JWT validation.

One of the largest vulnerabilities in JWT validation relates to the fact that the JWT itself specifies which signing algorithm has been used, and should be used for validation. In cases where the server does not ensure that this algorithm falls within a set, there are two key exploitation paths:

- Leaving a JWT unsigned, and setting the algorithm header to `none` means that JWT validation will succeed in libraries that have not been designed to prevent this flaw.
- In cases where a service uses asymmetric algorithms for JWT signing, some libraries are vulnerable to accepting a JWT that has been signed used a symmetric algorithm, with the public key of the issuer used as the pre-shared key.

By enforcing an allow-list (to only common battle-tested asymmetric algorithms) of algorithms, and checking this list as part of JWT validation, we mitigate these two vulnerabilities.

Configuring an allow-list also allows us to remove algorithms if a vulnerability is discovered in a specific one.

## References and Resources

OIDC Specifications:

- [OpenID Connect Core](https://openid.net/specs/openid-connect-core-1_0.html)
- [OpenID connect Discovery](https://openid.net/specs/openid-connect-discovery-1_0.html)


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