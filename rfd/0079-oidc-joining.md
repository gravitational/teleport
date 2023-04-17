---
authors: Noah Stride <noah@goteleport.com>
state: implemented (v11.0.0)
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

This feature reduces the friction involved in dynamically adding many new nodes to Teleport on a variety of platforms. This is also more secure, as the user does not need to distribute a token which is liable to ex-filtration.

Whilst multiple providers offer OIDC identities to workloads running on their platform, we will start by targetting GHA since this represents a key platform for Teleport, and is also well documented and easy to test on. However, the work towards this feature will also enable us to simply add other providers that support OIDC workload identity (see the references for more), this particularly ties into Machine ID goals as we aim to support several CI/CD providers that offer workload identity.

## Details

The work on OIDC joining is broken down into two parts:

- Support in the Auth server for trusting an OP issued JWT. This work will have a general base that can be shared between all providers, and then have a small, provider-specific part to provide additional validation that lets us guide users to correct and safe configurations.
- Support in nodes for fetching their workload identity token from their environment. This work will be specific to each platform we intend to support.

OIDC supports multiple types of token (`id_token`: a JWT encoding information about the identity, which can be verified using the issuer's public key and `access_token`: an opaque token that can be used with a `userinfo` endpoint on the issuer to obtain information about the identity). However, in the case of workload identities, `id_token` is the most prevalent. For this reason, our initial implementation will solely support `id_token`.

### Auth server support

We will re-use the existing endpoints around joining as much as possible. This means that the main entry-point for joining will be the existing `RegisterUsingToken` method.

We will introduce a new token join type for each provider. In the case of GHA this will be `github`.

Registration flow:

1. Client is configured by the user to use `github` joining. The client will then query the GitHub Actions internal API to obtain a JWT using the request token environment variable.
2. The client will call the `RegisterUsingToken` endpoint, providing the OIDC JWT that it has collected, and specifying the name of the Teleport provisioning token which should be used to verify it.
3. The server will attempt to fetch the Token resource for the specified token.
4. The server will check JWT header to ensure the `alg` is one we have allow-listed (RS[256, 384, 512])
5. The server will check the `kid` of the JWT header, and obtain the relevant JWK from the cache or from the specified issuers well-known JWKS endpoint. It will then use the JWK to validate the token has been signed by the issuer.
6. Other key claims of the JWT will be validated:
  - Ensure the audience is correct (for providers where the audience can be customised, this should be the Teleport cluster name.)
  - Ensure the Issued At Time (iat) is in the past (allowing 30 seconds of skew).
  - Ensure the Expiry Time (exp) is in the future (allowing 30 seconds of skew).
7. The user's [allow rules](#configuration) for the token will be evaluated against the claims, to ensure that the token is allowed to register with the Teleport cluster.
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

Whilst out of scope for an initial implementation, we should eventually allow the user to tweak the behaviour of the cache:

- Custom rate limit for refreshing the JWKS, for cases where the user is aware of a specific rate limit that could be abused to deny them service.
- Custom timeout for the request made to the OIDC well-known endpoint and JWKS endpoint, to allow the user to reduce the likelihood of resource exhaustion.

#### Configuration

In order to introduce support for GHA joining, we will introduce a new field to `ProvisionTokenV2` called `github`. This RFD sets a new standard for expansion of the `ProvisionTokenV2` with all future providers recommended to create their own top level fields, rather than continuing to expand the existing `Allow` field. Work will eventually begin to migrate IAM and EC2 joining to their own fields.

`ProvisionTokenSpecV2` with fields added to support GHA:

```proto
// ProvisionTokenSpecV2 is a specification for V2 token
message ProvisionTokenSpecV2 {
  // Roles is a list of roles associated with the token,
  // that will be converted to metadata in the SSH and X509
  // certificates issued to the user of the token
  repeated string Roles = 1 [
    (gogoproto.jsontag) = "roles",
    (gogoproto.casttype) = "SystemRole"
  ];
  // Allow is a list of TokenRules, nodes using this token must match one
  // allow rule to use this token.
  repeated TokenRule Allow = 2 [(gogoproto.jsontag) = "allow,omitempty"];
  // AWSIIDTTL is the TTL to use for AWS EC2 Instance Identity Documents used
  // to join the cluster with this token.
  int64 AWSIIDTTL = 3 [
    (gogoproto.jsontag) = "aws_iid_ttl,omitempty",
    (gogoproto.casttype) = "Duration"
  ];
  // JoinMethod is the joining method required in order to use this token.
  // Supported joining methods include "token", "ec2", and "iam".
  string JoinMethod = 4 [
    (gogoproto.jsontag) = "join_method",
    (gogoproto.casttype) = "JoinMethod"
  ];
  // BotName is the name of the bot this token grants access to, if any
  string BotName = 5 [(gogoproto.jsontag) = "bot_name,omitempty"];
  // SuggestedLabels is a set of labels that resources should set when using this token to enroll
  // themselves in the cluster
  wrappers.LabelValues SuggestedLabels = 6 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "suggested_labels,omitempty",
    (gogoproto.customtype) = "Labels"
  ];
  // GitHub allows the configuration of options specific to the "github" join method.
  ProvisionTokenSpecV2GitHub GitHub = 7 [(gogoproto.jsontag) = "github,omitempty"];
}

message ProvisionTokenSpecV2GitHub {
  // Rule includes fields mapped from `lib/githubactions.IDToken`
  // Not all fields should be included, only ones that we expect to be useful
  // when trying to create rules around which workflows should be allowed to
  // authenticate against a cluster.
  message Rule {
    // Sub also known as Subject is a string that roughly uniquely identifies
    // the workload. The format of this varies depending on the type of
    // github action run.
    string Sub = 1 [(gogoproto.jsontag) = "sub,omitempty"];
    // The repository from where the workflow is running.
    // This includes the name of the owner e.g `gravitational/teleport`
    string Repository = 2 [(gogoproto.jsontag) = "repository,omitempty"];
    // The name of the organization in which the repository is stored.
    string RepositoryOwner = 3 [(gogoproto.jsontag) = "repository_owner,omitempty"];
    // The name of the workflow.
    string Workflow = 4 [(gogoproto.jsontag) = "workflow,omitempty"];
    // The name of the environment used by the job.
    string Environment = 5 [(gogoproto.jsontag) = "environment,omitempty"];
    // The personal account that initiated the workflow run.
    string Actor = 6 [(gogoproto.jsontag) = "actor,omitempty"];
    // The git ref that triggered the workflow run.
    string Ref = 7 [(gogoproto.jsontag) = "ref,omitempty"];
    // The type of ref, for example: "branch".
    string RefType = 8 [(gogoproto.jsontag) = "ref_type,omitempty"];
  }
  // Allow is a list of TokenRules, nodes using this token must match one
  // allow rule to use this token.
  repeated Rule Allow = 1 [(gogoproto.jsontag) = "allow,omitempty"];
}
```

##### Configuration for OIDC GHA

```yaml
kind: token
version: v2
metadata:
  name: github-bot
  expires: "3000-01-01T00:00:00Z"
spec:
  roles: [Bot]
  join_method: github
  bot_name: robot
  github:
    allow:
      - repository: strideynet/sandbox
        repository_owner: strideynet
```

For GHA joining, they will set `join_method` to `github`.

To allow the user to configure rules for what JWTs will be accepted, we will leverage the `allow` field similar to how it works with the IAM joining. Each element of the slice is a set of conditions that must all be satisfied, and a token is authorised as long as at least one allow block is satisfied.

For GHA, we will map the [following JWT claims](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect#understanding-the-oidc-token) to configuration values:

- sub
- repository
- repository_owner
- workflow
- environment
- actor
- ref
- ref_type

To ensure that we guide users to creating secure configurations, we will also ensure that at least one of the following fields is included in each allow block:

- repository
- repository_owner
- sub

This ensures that a token produced in another GitHub organization cannot be used against their Teleport cluster.

#### Extracting claims as metadata for generated credentials

The JWTs issued by providers often contain claims that would be useful when auditing actions. We can extract these claims, and embed them into the certificates during registration. This additional metadata can then be audit logged when the certificates are used, allowing actions to be attributed to specific CI runs or individual VMs.

In order to implement OIDC joining in a timely manner, we should consider this out of scope for this initial implementation.

### Node support

Node here not only refers to a Teleport node, but also to various other participants within a Teleport cluster (e.g tbot, kube agent etc).

We will need to support collecting the token from the environment. This will differ on each platform. Some offer the token via a metadata service, and others directly inject it via an environment variable. Where possible, we should encourage the user to configure the token to be generated with an audience of their Teleport cluster, however, not all providers support this (e.g GitLab CI/CD).

For GHA, a HTTP request is made to an internal service accessible to the workflow runner. The address of this service is injected into the environment as `ACTIONS_ID_TOKEN_REQUEST_URL`, and a token which must be provided as an Authorization header is injected as `ACTIONS_RUNTIME_TOKEN`. Making this GET request returns a JSON object including a field with the signed JWT.

### Security Considerations

#### Secure authorization rules

It is imperative that we validate the user's provided `allow` rules so that they do not accidentally allow tokens produced in other projects to be used against their teleport cluster. For example, if they provided an `allow` rule that only included `instance_name`, a malicious actor would be able to create an instance with the same name, and use this to generate a valid token.

#### MITM of the connection to the issuer

In order to verify the JWT, we have to fetch the public key from the issuers's JWKS endpoint. If this connection is not secured (e.g HTTP), it would be possible for a malicious actor to intercept the request and return a public key they've used to sign the JWT with.

We should require that the configured issuer URL is HTTPS to mitigate this.

Another potential mitigation would be to allow users to pin a certain certificate signature they expect Teleport to receive when connecting to the issuer. Whilst this is not particularly useful for GHA (since they rotate certificates on a regular basis), this could be useful if we support custom OIDC providers where the user controls the certificate used for the endpoint and would be able to rotate the pinned certificate in step with their issuer. As this is not useful for GCP, nor GHA, it is out of scope for the initial implementation.

#### Vulnerabilities in JWT and JWT signing algorithms

This section is included, since historically, there have been a large number of cases of vulnerabilities introduced into software because of misunderstandings and mistakes in JWT validation.

One of the largest vulnerabilities in JWT validation relates to the fact that the JWT itself specifies which signing algorithm has been used, and should be used for validation. In cases where the server does not ensure that this algorithm falls within a set, there are two key exploitation paths:

- Leaving a JWT unsigned, and setting the algorithm header to `none` means that JWT validation will succeed in libraries that have not been designed to prevent this flaw.
- In cases where a service uses asymmetric algorithms for JWT signing, some libraries are vulnerable to accepting a JWT that has been signed used a symmetric algorithm, with the public key of the issuer used as the pre-shared key.

By enforcing an allow-list (to only common battle-tested asymmetric algorithms) of algorithms, and checking this list as part of JWT validation, we mitigate these two vulnerabilities.

Configuring an allow-list also allows us to remove algorithms if a vulnerability is discovered in a specific one.

#### JWT Exfiltration

This broadly falls into two categories:

The first involves a malicious actor intercepting the JWT sent by the node during the join process. Whilst TLS prevents this in most cases, a malicious actor with sufficient access to the environment would be able to insert their own CA into the root store and use this to man-in-the-middle the connection.

The second involves a malicious actor with direct access to executing code within the workload environment. Here they can follow the same steps the node would take in obtaining the JWT.

In both cases, the malicious actor can then use this JWT to gain Teleport credentials and access to customer infrastructure.

Mitigations:

Firstly, we should ensure in documentation that we stress that an attacker with access to the workload environment will be able to access resources that the workload would have access to. The principle of least privilege should be encouraged, with workloads given access to only the resources they require.

In most cases, it appears the providers issue these workload identity JWTs with relatively short lives of between 5 and 10 minutes (GHA and GCP respectively). This does limit a malicious actors ability to continually use these credentials if they lose access to the environment in which they are generated, but, if the attacker exchanges these for a long-lived set of Teleport credentials then this point is relatively moot. We can further limit their access by reducing the TTLs of certificates we offer in exchange for OIDC JWTs, and preventing renewals: requiring a fresh JWT from the environment each time they want to generate certificates. This would be a significant change to the current model whereby Teleport nodes receive long lived certificates (10/infinity years), and present complications in how we currently handle CA rotations. This does feel like the most effective mitigation, as it requires the malicious actor maintain access to the compromised workload environment for roughly as long as they want to access infrastructure.

Another option we have would be to ensure that a JWT could only be used once by caching them and sharing this cache across auth servers. This specifically assists with preventing an attacker who has intercepted a JWT from transparently using it to generate certificates, as this would then either cause the node's attempt to join to fail or theirs. It feels like in most cases this is unlikely to be noticed by users, as the node would simply retry.

One suggested mitigation has been to introduce some kind of challenge process, and require that the nodes request the generation of a JWT including a challenge string. On some providers (GCP, GHA) we would be able to leverage the `aud` claim for this as the provider allows the workload to request a JWT that has been generated with a certain `aud` set. However, not all providers allow this. This mitigation works in two ways. Firstly, it prevents a token from being used to generate certificates more than once, this is similar to the mitigation involving caching used JWTs, and by no means prevents a malicious actor gaining access but does force them to give up the transparency of doing so. Secondly, it increases the complexity of an attack for a malicious actor stealing a JWT directly from the environment. They would need to first perform the first stage of the join to determine the challenge string to generate the JWT with. It does not prevent an attacker with access to the environment from gaining access.

It is of note that neither GCP or Vault's implementation of OIDC trust federation implement additional mitigations. They rely on the user to ensure that their workload environment is secure. This does feel like an acceptable posture to take. Ultimately, most mitigations seem ineffective if the environment that the user has configured to trust is compromised. The mitigation which seems most effective in limiting access is the first choice: forcing shorter TTLs onto certificate produced by OIDC token joining and requiring that a fresh JWT is used to gain credentials when those expire, rather than renewing. However, this seems like a significant change in the current operating model of Teleport and feels like this could be pushed as a future improvement.

## References and Resources

OIDC Specifications:

- [OpenID Connect Core](https://openid.net/specs/openid-connect-core-1_0.html)
- [OpenID connect Discovery](https://openid.net/specs/openid-connect-discovery-1_0.html)

Providers of Workload Identity. These are platforms we can support once OIDC joining is added:

- [Github Actions](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect)
- [GCP (GCE, GCB)](https://cloud.google.com/compute/docs/instances/verifying-instance-identity#token_format)
- [CircleCI](https://circleci.com/docs/openid-connect-tokens)
- [GitLab](https://docs.gitlab.com/ee/ci/cloud_services/)
- [SPIFFE/SPIRE](https://spiffe.io/docs/latest/keyless/): This presents an interesting use case for tokenless joining in a variety of environment.

Similar implementations:

- [GCP Workload identity federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [HashiCorp Vault](https://www.vaultproject.io/docs/auth/jwt)
- [AWS IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc.html)
- [Azure AD](https://docs.microsoft.com/en-us/azure/active-directory/develop/workload-identity-federation)
