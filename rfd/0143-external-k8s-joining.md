---
authors: Noah Stride (noah.stride@goteleport.com)
state: draft
---

# RFD 00143 - External Kubernetes delegated joining (jwks)

## Required approvers

* Engineering: @zmb3 && (@hugoShaka || @tigrato)
* Product: @klizhentas || @xinding33
* Security: @reedloden

## What

This RFD proposes a method for allowing entities (such as a Machine ID instance
or an Agent) to join Teleport Clusters via federation between the Kubernetes 
Cluster they reside in and a Teleport Cluster, where that Teleport Cluster does 
not reside in the same Kubernetes Cluster.

This is distinct from 
[RFD 0094 - Kubernetes node joining](./0094-kubernetes-node-joining.md) which
only supports the joining entity existing within the same Kubernetes Cluster
as the Teleport Cluster.

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

This join method will be added to the existing `ProvisionTokenV2` specification.

In order to clearly distinguish this new join method from the existing
`kubernetes` join method, this method will be named `kubernetes-remote`.

Similar to other join methods, users will be able to configure a list of allow
rules. If a joining entities presented token matches any of the allow rules 
then it will be allowed to join.

Each join token will be linked to a specific Kubernetes Cluster. It will not
be possible to create a join token which can be used for joining from multiple
Kubernetes Clusters.

Example token configuration:

```yaml
kind: token
version: v2
metadata:
  name: kubernetes-remote-token
spec:
  roles: ["Bot"]
  bot_name: argocd
  join_method: "kubernetes-remote",
  kubernetes_remote:
    clusters:
    - name: my-cluster
      # static_jwks is obtained by the user by following the steps after this 
      # example configuration.
      static_jwks: |
        {"keys":[{"use":"sig","kty":"RSA","kid":"-snip-","alg":"RS256","n":"-snip-","e":"-snip-"}]}
    - name: my-other-cluster
      static_jwks: |
        {"keys":[{"use":"sig","kty":"RSA","kid":"-snip-","alg":"RS256","n":"-snip-","e":"-snip-"}]}
    allow:
    # Matches a SA JWT from any configured cluster in the namespace
    # `my-namespace` and named `my-service-account`.
    - service_account: "my-namespace:my-service-account"
    # Matches only SA JWT signed by the configured cluster above named
    # `my-other-cluster` and from the `my-namespace` namespace and named
    # `my-other-service-account`.
    - service_account: "my-namespace:my-other-service-account"
      cluster: my-other-cluster
```

This configuration is defined in Protobuf form as:

```protobuf
syntax = "proto3";

message ProvisionTokenSpecV2KubernetesJWKS {
  // Rule is a set of properties the Kubernetes-issued token might have to be
  // allowed to use this ProvisionToken
  message Rule {
    // ServiceAccount is the namespaced name of the Kubernetes service account.
    // Its format is "namespace:service-account".
    string ServiceAccount = 1;
    // Clusters specifies which of the clusters this allow rule applies to.
    // If unspecified, this allow rule will not be restricted to a specific
    // cluster.
    repeated string Clusters = 2;
  }

  // Cluster is a set of properties that describe a cluster that an entity may
  // attempt to join from.
  message Cluster {
    // Name is an identifier configured for the cluster. This value is chosen
    // by the user, and does not have to match a specific value configured in
    // Kubernetes.
    string Name = 1;
    // Source is how the public signing keys for the cluster will be obtained
    // to use for validating a provided JWT.
    oneof Source {
      // StaticJWKS allows the public signing keys for a cluster to be
      // specified statically with the JSON body of the JWKS endpoint.
      string StaticJWKS = 2;
    }
  }
  
  // Clusters is a list of Cluster that are accepted sources of JWTs to use
  // against this token. A Service Account JWT signed by a cluster not listed
  // here will be rejected.
  repeated Cluster Clusters = 1;
  // Allow is a list of Rules, nodes using this token must match one
  // allow rule to use this token.
  repeated Rule Allow = 2;
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

#### Configuring Kubernetes RBAC

The pod will require a service account that grants it access to generate a token
for the service account used for joining. It is our recommendation that these
two service accounts are seperate. The Kubernetes RBAC for generating tokens
is not finely grained, and creating a service account which can impersonate
itself gives a bad actor who has access to the pod the ability to create
long-lived and unbound tokens. By using a second service account for joining, 
the primary service account is no longer given the permission to generate a
token for itself and can instead only generate tokens for the permissionless
secondary service account.

E.g for a service account named: "foo":

```yaml
# This is the primary SA that will be bound to the pod.
apiVersion: v1
kind: ServiceAccount
metadata:
  name: foo
---
# This is the secondary SA that has no permissions within Kubernetes but is 
# used by Teleport for joining.
apiVersion: v1
kind: ServiceAccount
metadata:
  name: foo-join
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: Role
metadata:
  name: foo-join-impersonator
rules:
- apiGroups: [""]
  resources:
  - "serviceaccounts/token"
  verbs:
  - "create"
  # Only grant the ability to create tokens for the secondary service account.
  # WARNING: Missing this field leads to a dangerously powerful role.
  resourceNames:
    - foo-join
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: foo-join-impersonator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: foo-join-impersonator
subjects:
- kind: ServiceAccount
  name: foo
```

### Implementation

The `kubernetes-remote` flow will leverage a challenge and response flow, similar
to that implemented for IAM joining. The flow will run as follows:

1. The client wishing to join calls the `RegisterUsingKubernetesRemoteMethod`
  RPC. It submits the name of the token it wants to use.
2. The Auth Server sends a response with the audience that should be set in the 
  token. This audience is a concatenation of the cluster name, and 24 bytes of
  cryptographically random data encoded in base64 URL encoding, e.g:
  `example.teleport.sh/YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4`
3. The client uses its Kubernetes Service Account to call TokenRequest. It
  requests a JWT for the secondary service account, with the audience specified
  by the Auth Server and with a short time-to-live.
4. The client sends a further request up the stream including this token.
5. The server validates:
  a. The JWT is signed by the keys present in the `jwks` field of the token 
    resource.
  b. The JWT is not expired, and at the time of issue, the JWTs TTL was short
   (e.g within a similar TTL to the 30s TTL we set)
  c. The JWTs audience claim matches the one the server challenged the client
    with.
  d. The JWT includes the `kubernetes.io` claim which binds it to a specified
    namespace and pod.
  e. The JWT subject claim is for a service account, and this service account 
    matches one of those configured in the allow rules of the token resource.
6. If the JWT passes validation, certificates are signed and returned to the 
  client.

This flow leverages the follow gRPC RPCs and Protobuf messages:

```protobuf
syntax = "proto3";

package proto;

message RegisterUsingKubernetesRemoteMethodRequest {
  oneof payload {
    // register_using_token_request holds registration parameters common to all
    // join methods.
    types.RegisterUsingTokenRequest register_using_token_request = 1;
    // jwt is the solution to the registration challenge-response. It is a
    // JWT issued by the Kubernetes CA containing the challenge audience sent by
    // the Auth Server.
    string jwt = 2;
  }
}

message RegisterUsingKubernetesRemoteMethodResponse {
  oneof payload {
    // challenge_audience is the audience that the client should use when
    // calling the Kubernetes API TokenRequest method.
    string challenge_audience = 1;
    // certs is the returned signed certs if registration succeeds.
    Certs certs = 2;
  }
}

service JoinService {
  rpc RegisterUsingKubernetesRemoteMethod(stream RegisterUsingKubernetesRemoteMethodRequest) returns (stream RegisterUsingKubernetesRemoteMethodResponse);
}
```

## Security Considerations

It's important to note that this join method will be less secure than the 
`kubernetes` join method. This should be explicitly mentioned in documentation
for this join method, and encourage the use of `kubernetes` where possible.

This is because the `kubernetes` join method makes use of the TokenReview 
endpoint which performs additional checks to the validity of the token. This 
includes checking that the pod and service account listed within the JWT claims 
exist and are running. This increases the complexity of falsifying a 
token and also ensures that a token cannot be used beyond the lifetime of the 
pod it is bound to. We can mimic this by using a very short time to live for 
the JWT (30 seconds).

However, this join method is still more secure than the long-lived secret
based `token` join method that operators are forced to use due to this
external Kubernetes join method not existing.

As with any delegated join method, sufficient compromise of the trusted party 
(in this case, the Kubernetes Clusters OIDC CA private key) means that a
malicious actor is able to use any join token that delegates trust to the
compromised party. There is little mitigation we can apply here, beyond
reminding operators of this fact and encouraging them to use security best
practices to secure the Kubernetes CAs. 

### Token reuse

Token reuse refers to the potential for a malicious actor to intercept
communications between the client and the server, and to use this interception
to capture the JWT and then use this elsewhere to obtain credentials.

Token reuse is mitigated in two ways:

- A challenge and solution flow is used with a nonce. This means that each
  JWT is only valid once for a specific join. In order to use this JWT, the
  malicious actor would need to cause the real join flow to fail.
- A short time-to-live will be used on JWTs issued for the join process. This
  limits the malicious actors options and forces them to use the JWT
  immediately.

### Kubernetes CA Compromise

One of the largest risks to any delegated join method is the trusted parties
issuer becoming compromised. This is no different for this new Kubernetes join
method. If the Kubernetes Cluster's JWT CA is compromised, then a malicious 
actor will be able to issue a JWT posing as any theoretical service account.

There is no mitigation route for this risk. It is up to the operator to secure
their Kubernetes cluster appropriately and to ensure that join tokens that
delegate trust to this cluster are appropriately scoped in privileges to reduce
the blast radius if their Kubernetes CA were to become compromised.

## Alternatives

### Extending the existing `kubernetes` join method

Extending the existing `kubernetes` join method to support external joining
has two key issues.

The first issue is connectivity. The `kubernetes` join method relies on the Auth
Service's ability to call the TokenReview endpoint exposed by the Kubernetes API
server. This means there must be network connectivity and the appropriate 
firewall rules to allow the request to pass. Many users operate clusters that
are not exposed to the internet, and this means Teleport Cloud would not work
without some additional way to pass traffic to the external Kubernetes cluster
API server.

The second issue is authentication. Calling the TokenReview RPC requires some 
form of authentication. At the moment, the Auth Service relies on a Kubernetes
service account available to it by virtue of being a workload within the
cluster.

The issue with authentication could be solved by allowing an operator to
configure credentials for the Kubernetes cluster as part of the join token
specification. This, however, does not solve the issue of connectivity and
introduces further complexities as to how we would safeguard customer
credentials in Teleport Cloud.

One solution that would solve the connectivity and authentication problems
would be to use an existing Teleport Kubernetes Agent deployed into a
Kubernetes Cluster as a "stepping stone". Existing methods for executing a
Kubernetes request could be leveraged by the Auth Server and identity
impersonation used to ensure the request is completed with a group that has
access to the TokenReview RPC.

This does raise a few concerns:

- This method would not be suitable for deploying the first Kubernetes Agent
  into a cluster.
- This raises the potential fallout from a compromised Kubernetes Agent. A
  compromised agent could be used to impersonate any service account.
- There is no prior art for the Auth Server connecting directly to an Agent.
  It's also possible that a Kubernetes Agent may not live within a Kubernetes
  Cluster when using auto-discovery.

We should revisit this in the long term and determine if there are more
elegant ways for us to allow external Kubernetes cluster joining with the
`kubernetes` join method.