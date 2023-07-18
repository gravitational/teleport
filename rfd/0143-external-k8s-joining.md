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
`kubernetes` join method, this method will be named `kubernetes-jwks`.

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
  name: kubernetes-jwks-token
spec:
  roles: ["Bot"]
  bot_name: argocd
  join_method: "kubernetes-jwks",
  kubernetes_jwks:
    # jwks is obtained by the user by following the steps after this example
    # configuration.
    jwks: |
      {"keys":[{"use":"sig","kty":"RSA","kid":"9-MIvttbVaRsRf5ejiOtKguarpDA_dJ2skL81OgY2ck","alg":"RS256","n":"3VRj5e27ne706BVQi4LDNg2x31HJc3vrXnsYmyfOFKfRDP6cPesyteyCcTYWhoIlMy3GCKWO1gzeIINMbZgndM87Dw9Dsl0eJQeL_GFAIXOxMoavraNuptFSrV43qQ8kUVDsiC9gSGJVs6LR9bClL8vksmL7_nbSrMviUygPvj-mf4ngPRT6XnKyldKiePMwXrUnomM4FWskZ_UvPiqwWZu1aXhcuEdNA3yOFFq08H1ys71iiRAMyD2knuJV9sZgt_Ns-28ofrR45yR6nzKhmjIJf1H9Fy33o6jtXdtqxeLdqOseRJm3A8PJE4Zp1NfuCJSjsxIhZYHXXH60EPCmNw","e":"AQAB"}]}
   # allow rules will be the same as for the `kubernetes` join method.
    allow:
      - service_account: "my-namespace:my-service-account"
```

In order to obtain the `jwks` value for their Kubernetes Cluster, operators
will need to run the following steps from a machine already authenticated
with their cluster:

- `kubectl proxy -p 8080`
- `curl http://localhost:8080/openid/v1/jwks`

In the case that an operator rotates the certificate authority used by their
Kubernetes Cluster to sign Service Account JWTS, the `jwks` field will also
need to be updated.

#### Configuring a pod for `kubernetes-jwks` joining

The Pod will require a service account with a role granting it the ability to
call the TokenRequest endpoint for itself. This allows the creation of a token
with an audience of our choosing in order to include a nonce during the join
to prevent token reuse.

E.g for a service account named: "argocd":

```yaml
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: argocd-token-request
rules:
- apiGroups: [""]
  resources:
  - "serviceaccounts/token"
  verbs:
  - "create"
  # Only grant the ability to create tokens for itself. Missing this field leads
  # to a dangerously powerful role.
  resourceNames:
    - argocd
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: argocd-token-review
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: argocd-token-review
subjects:
- kind: ServiceAccount
  name: argocd
  namespace: default
```

### Implementation

<-- TODO: Write this in more detail -->
<-- TODO: Specs for RegisterUsingKubernetesJWKS RPC -->

The `kubernetes-jwks` flow will leverage a challenge and response flow, similar
to that implemented for IAM joining. The flow will run as follows:

1. The client wishing to join calls the `RegisterUsingKubernetesJWKS` RPC. It
  submits the name of the token it wants to use.
2. The Auth Server sends a response with the audience that should be set in the 
  token. This audience is a concatenation of the cluster name, and 24 bytes of
  cryptographically random data encoded in base64 URL encoding, e.g:
  `example.teleport.sh/YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4`
3. The client uses its Kubernetes Service Account to call TokenRequest. It
  requests a JWT for its own service account, with the audience specified by
  the Auth Server and with a short time-to-live.
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

The risk of token reuse will be mitigated by the inclusion of a nonce within
the audience of the JWT, as described under details.

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