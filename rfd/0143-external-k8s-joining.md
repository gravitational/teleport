---
authors: Noah Stride (noah.stride@goteleport.com)
state: draft
---

# RFD 00143 - Static JWKS External Kubernetes Joining

## Required approvers

* Engineering: @zmb3 && (@hugoShaka || @tigrato)
* Product: @klizhentas || @xinding33
* Security: @reedloden || @jentfoo

## What

This RFD proposes a method for allowing entities (such as a Machine ID instance
or an Agent) to join Teleport Clusters via federation between the Kubernetes 
Cluster they reside in and a Teleport Cluster, where that Teleport Cluster does 
not reside in the same Kubernetes Cluster.

This extends
[RFD 0094 - Kubernetes node joining](./0094-kubernetes-node-joining.md) which
only supports the joining entity existing within the same Kubernetes Cluster
as the Teleport Cluster at this time.

## Why

It is not unusual for users to wish to deploy Machine ID bots and Teleport
Agents in Kubernetes Clusters which is not the same cluster in which their
Auth Service resides. For example, they may operate many Kubernetes Clusters or
they may use Teleport Cloud (in which we host their Teleport Cluster).

Currently, these users must use a different join method, and, for users
deploying into environments where there is no platform-specific delegated
join method, they must fallback to using the `token` join method which is a
sensitive, long-lived, secret. This also requires some element of state, and
is inconvenient in more ephemeral use-cases.

To provide a more concrete use-case, many users currently deploy Teleport
Plugins (such as the Slack Access Request plugin). Adding support for External
Kubernetes Joining will provide a golden pathway for this deployment using
Machine ID that avoids long-lived shared secrets and provides a seamless 
experience.

## Details

### Configuration

To configure the Teleport for external Kubernetes joining, the existing
`kubernetes` join method will be used.

A new `type` field will be introduced with two possible values:

- `in_cluster`: the current behaviour of Kubernetes joining based on TokenReview.
- `static_jwks`: enables the new behaviour based on validation of the token
  against a known JWKS.

Where no value has been provided, this will default to `in_cluster`.

Example token configuration:

```yaml
kind: token
version: v2
metadata:
  name: kubernetes-remote-token
spec:
  roles: ["Bot"]
  bot_name: argocd
  join_method: "kubernetes"
  kubernetes:
    type: static_jwks
    static_jwks:
    - jwks: |
        {"keys":[{"use":"sig","kty":"RSA","kid":"-snip-","alg":"RS256","n":"-snip-","e":"-snip-"}]}
    allow:
    # Matches a SA JWT from any configured cluster in the namespace
    # `my-namespace` and named `my-service-account`.
    - service_account: "my-namespace:my-service-account"
```

This configuration is defined in Protobuf form as:

```protobuf
syntax = "proto3";

message ProvisionTokenSpecV2KubernetesStaticJWKS{
  // JWKS contains the JSON Web Keys to validate a service account token
  // against. This should be copied from the Kubernetes server's JWKS endpoint
  // (/openid/v1/jwks).
  string JWKS = 1;
}

message ProvisionTokenSpecV2Kubernetes{
  // Rule is a set of properties the Kubernetes-issued token might have to be
  // allowed to use this ProvisionToken
  message Rule {
    // ServiceAccount is the namespaced name of the Kubernetes service account.
    // Its format is "namespace:service-account".
    string ServiceAccount = 1;
  }
  
  // Allow is a list of Rules, nodes using this token must match one
  // allow rule to use this token.
  repeated Rule Allow = 1;

  // Type specifies what behaviour should be used for validating a
  // Kubernetes Service Account token. This must be
  // - `in_cluster`
  // - `static_jwks`
  // If omitted, it will default to `in_cluster`.
  string Type = 2;

  // StaticJWKS contains configuration specific to the `static_jwks` type.
  ProvisionTokenSpecV2KubernetesStaticJWKS StaticJWKS = 3;
}
```

In order to obtain the `jwks` value for their Kubernetes Cluster, operators
will need to run the following steps from a machine already authenticated
with their cluster:

- `kubectl proxy -p 8080`
- `curl http://localhost:8080/openid/v1/jwks`

In the case that an operator rotates the certificate authority used by their
Kubernetes Cluster to sign Service Account JWTS, the `jwks` field will also
need to be updated.

#### Configuring Kubernetes

The joining entity will need to exist in a pod where a Service Account token
has been projected with an audience equal to the name of the Teleport cluster.

E.g for a Teleport cluster named: "example.teleport.sh":

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service-account
---
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
    - image: nginx
      name: nginx
      volumeMounts:
        - mountPath: /var/run/secrets/tokens
          name: my-service-account
  serviceAccountName: my-service-account
  volumes:
    - name: my-service-account
      projected:
        sources:
          - serviceAccountToken:
              path: my-service-account
              expirationSeconds: 600
              audience: example.teleport.sh
```

### Implementation

The `static_jwks` variant will leverage some of the same parts of the flow
as the `in_cluster` variant.

1. As with `in_cluster`, the client loads the service account JWT from the
  filesystem that was mounted by Kubelet and calls `RegisterUsingToken` with 
  this JWT and the name of the token it wishes to join using.
2. The Auth Server finds the ProvisionToken matching the name of token. If this
  is a ProvisionToken with `type: static_jwks`, the flow now diverges from the
  existing `in_cluster` implementation.
5. The Auth Server validates:
  a. The JWT is signed by the keys present in the `jwks` field of the 
    ProvisionToken.
  b. The JWT is not expired, and at the time of issue, the JWTs TTL was short.
    This ensures that users are using the recommended configured TTL value.
    As the Kubernetes minimum TTL is 10 minutes, we will ensure that this has
    been configured to at most 30 minutes to allow for some flexibility in
    configuration.
  c. The JWTs audience claim matches the cluster name.
  d. The JWT includes the `kubernetes.io` claim which binds it to a specified
    namespace and pod.
  e. The JWT subject claim is for a service account, and this service account 
    matches one of those configured in the allow rules of the token resource.
6. If the JWT passes validation, certificates are signed and returned to the 
  client.

This flow requires no modifications to the existing RPC protobufs.

## Security Considerations

It's important to note that this join method will be less secure than the 
`in_cluster` variant. This should be explicitly mentioned in documentation
for this join method, and encourage the use of `in_cluster` where possible.

This is because the `in_cluster` join method makes use of the TokenReview 
endpoint which performs additional checks to the validity of the token. This 
includes checking that the pod and service account listed within the JWT claims 
exist and are running. This increases the complexity of falsifying a 
token and also ensures that a token cannot be used beyond the lifetime of the 
pod it is bound to. We can mimic this by using a very short time to live for 
the JWT (30 seconds).

However, this join method is still more secure than the long-lived secret
based `token` join method.

### Token reuse

Token reuse refers to the potential for a malicious actor to intercept
communications between the client and the server, and to use this interception
to capture the JWT and then use this elsewhere to obtain credentials.

To mitigate this, a short time-to-live will be used on JWTs issued for the join 
process. This limits the malicious actors options and forces them to use the JWT
immediately.

In addition, the audience field is used to ensure that the Teleport Cluster is
the intended recipient of the JWT and that this has not been stolen from another
relying party.

### Kubernetes CA Compromise

One of the largest risks to any delegated join method is the trusted parties
issuer becoming compromised. This is no different for this new Kubernetes join
method. If the Kubernetes Cluster's JWT CA is compromised, then a malicious 
actor will be able to issue a JWT posing as any theoretical service account.

There is no mitigation route for this risk. It is up to the operator to secure
their Kubernetes cluster appropriately and to ensure that join tokens that
delegate trust to this cluster are appropriately scoped in privileges to reduce
the blast radius if their Kubernetes CA were to become compromised.

### Audit Events

The existing audit events for token creation, bot and agent joins cover the
functionality introduced by this PR - therefore no new audit event needs to be
added.

However, the bot and agent join audit event for Kubernetes joins will be
extended to include additional attributes from the Service Account token used
to join. This is similar to the behaviour we already have implemented for
GitHub and GitLab joining. This will provide an additional level of insight into
the join, allowing it to be traced back to the individual pod.

The fields that will be introduced can be seen under `attributes` in this
example:

```json
{
  "attributes": {
    "raw": {
      "aud": [
        "leaf.tele.ottr.sh"
      ],
      "exp": 1692026408,
      "iat": 1692025808,
      "iss": "https://kubernetes.default.svc.cluster.local",
      "kubernetes.io": {
        "namespace": "default",
        "pod": {
          "name": "ubuntu",
          "uid": "f6dd8b5e-cafc-4d92-b5ac-1a124a019d72"
        },
        "serviceaccount": {
          "name": "tbot",
          "uid": "8b77ea6d-3144-4203-9a8b-36eb5ad65596"
        }
      },
      "nbf": 1692025808,
      "sub": "system:serviceaccount:default:tbot"
    },
    "type": "static_jwks",
    "username": "system:serviceaccount:default:tbot"
  },
  "bot_name": "my-bot",
  "cluster_name": "leaf.tele.ottr.sh",
  "code": "TJ001I",
  "ei": 0,
  "event": "bot.join",
  "method": "kubernetes",
  "success": true,
  "time": "2023-08-14T15:15:59.91Z",
  "token_name": "my-bot-kubernetes-token",
  "uid": "84922598-1ba6-499f-a8cf-0dba96cab1d5"
}
```

This will provide a more in-depth audit event that allows the join to be
traced back to a specific Kubernetes pod.

## Alternatives

### Introducing a separate `kubernetes_remote` join method

One alternative implementation was to introduce a new `kubernetes_remote` join
method that would use a bi-di gRPC RPC to create a challenge and response flow
using the audience field of the token.

The Pod would use the TokenRequest Kubernetes API endpoint to request a JWT for
a Service Account including the challenge audience.

This implementation had the following concerns:
- We could no longer determine reliably the Pod that the Service Account was
  linked to. This reduced audit visibility.
- The challenge and response flow did not fully mitigate man-in-the-middle
  attacks.
- Limitations of the Kubernetes RBAC model meant we would need to leverage
  two service accounts in order to prevent TokenRequest being used to create
  an infinitely long-lived token. This was also more complex as two service
  accounts would be to be created, as well as roles and role bindings.
- A bi-di challenge-response flow is more technically complex and has more 
  points of failure.

As well as the technical concerns, the UX was also more complex. Users would
need to understand the two Kubernetes join methods and configuration would
often involve similar, but subtly different, options. The proposed
implementation is this RFD is simpler and reuses the same concepts as used in
the existing Kubernetes join method (providing better uniformity).