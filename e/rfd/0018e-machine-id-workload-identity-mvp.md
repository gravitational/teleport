---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 18e - Machine ID: Workload Identity MVP

## Required Approvers

- Engineering: @zmb3
- Product: (@xinding33 || @klizhentas)

## Product Design

### What

Workload identity ultimately solves two key problems in the space of service to
service communication:

- Issuing x509 certificates that can be used for transport layer security when
  communicating between services across platforms.
- Issuing a portable form of identity that can be used for authentication and
  authorization when communicating between services across platforms.

It solves these problems by issuing short-lived x509 certificates to each
workload, encoding the identity of the workload in the certificate. These can
then be used for transport layer security but also for determining the
identity of the calling workload.

Our implementation will be compatible with a common open standard for workload
identity called SPIFFE. With the Teleport CA being used to issue these
certificates.

Teleport Workload Identity will be unlike other Teleport offerings in that it
will not leverage the Teleport proxy.

### Why

Why are workload identity solutions desired:

- Services to service communication is often highly sensitive and involves
  RPCs that are extremely powerful. This makes it a prime target for malicious
  actors.
- Legacy techniques for securing service to service communication are insecure:
  - Shared tokens are easily exfiltrated and often long-lived. 
  - Relying on network boundaries to protect services provides a deep level of 
    access to a malicious actor who has gained a foothold in the network.
  - Legacy techniques do not provide a foundation for transport security (e.g 
    TLS) and this must be implemented separately.
- Legacy techniques often do not provide detailed (or any) information about a
  caller which can be used for authorization decisions and auditing. This
  often leads to "all or nothing" access.

Common pain-points with workload identity implementations:

- In heterogeneous environments, the native identities offered by platforms
  cannot be used to identify workloads across platforms.
- Securely issuing identities to workloads is challenging. If this process is
  compromised, then the entire workload identity system is compromised. Often,
  this process involves integrating with the platform the workload runs on and
  this means building integrations for each platform used.
- Integrating workload identity with workloads is complicated by variety in the
  implementation of the workloads. Different languages and frameworks may be
  in use and engineering work is required to integrate with each of them.
- A good implementation of workload identity includes additional facets like
  auditing. This increases the complexity in developing a solution.
- Many small to medium engineering organisations lack the security skills or
  resources to implement a secure workload identity solution.

Why should **we** should build a Workload Identity product:

- Teleport's technical design is well suited to workload identity as our core
  we act as a x509 certificate authority. x509 certificates are a popular
  technology for workload identity.
- Machine ID's `tbot` already operates in similar environments, and therefore
  we already have a framework on which to build a workload identity product.
- Teleport existing customer base is well positioned to desiring a workload
  identity solution. They are security conscious and often operating in
  heterogeneous environments, which amplifies the complexities associated with
  deploying and operating a workload identity solution.
- Teleport is a respected player with sales and support infrastructure. This
  gives us an advantage over small startups which may launch in this space.

### Target Audience

Our target audience will be small and medium engineering organizations who
develop a technical product. There are roughly two categories within this:

- Organizations who have an existing workload identity solution and awareness of
  the space. They have some pain-points with their existing solution and wish to
  migrate to another solution.
- Organizations without a workload identity solution and who currently rely on
  legacy practices. These organizations may have less mature security practices
  and a varying level of understanding of what workload identity entails. They
  may be aware that their current solution is inadequate but unsure how to
  deal with this.

At this time, we may want to avoid targeting larger organizations. Their
requirements are likely to be more advanced and they are more likely to have
the resources internally to build their own solution.

#### Case Study: Vespa AI (Existing Solution)

Vespa AI is a customer currently trialling Teleport as an access solution. In
an early conversation with our sales team, they asked if Teleport offered a
workload identity solution.

Vespa have number of microservices deployed for each customer tenant. These
are written primarily in Java and C++ and are hosted on VMs across multiple
cloud providers (AWS, GCP and potentially Azure in future). They need to
secure communication between services within each tenant and ensure that
access to services is restricted to the correct tenant.

Vespa falls within the first category, they have an existing workload identity
solution, AthenZ Copper Argos. This is an open source solution developed by 
Yahoo that does not implement SPIFFE. It does issue TLS certificates and these
are used for mTLS and encode an identity for the service similar to that of
SPIFFE.

They are aware of the SPIFFE standard and are interested in adopting it as it
would provide them more flexibility and integration with off the shelf tooling.

Based on a conversation with them, the following things are motivating them to
find a solution other than AthenZ:

- Simplicity. AthenZ is fairly complex and requires that they develop several
  custom services to bridge between AthenZ and their workloads.
- Lack of support. AthenZ is not commercially supported and as they drift away
  from Yahoo, they are losing influence and access to subject-matter experts.
- Compatibility. AthenZ is not compatible with SPIFFE and this means they do
  not have access to the tooling that is available for SPIFFE.

They are interested in the following things from a solution

- Multi-cloud solution. It should integrate with their workloads regardless of
  the platform they are running on.
- Multi-language support. It should be easy to integrate with all of their
  services regardless of the language they are written in.
- A commercially supported solution. Ideally, they'd have access to support in 
  implementation but also in operation if problems occur. 
- Simplicity. The less they would need to operate the better. This is why they
  are currently pursuing Teleport Cloud. As a startup, they want to focus their
  efforts on their product rather than solving workload identity.
- Consolidation. If they are adopting Teleport for access then it would be 
  convenient to also use Teleport for workload identity and have a single place
  to manage identity.
- SPIFFE. The customer is aware of the SPIFFE standard and are interested in
  it due to the off the shelf integrations it would provide (e.g `java-spiffe`, 
  `spiffe-helper`)

A key takeaway from the call with Vespa is their motivations being driven by
wanting to simplify their operations and focus on their key product. Open-source
solutions like SPIRE and AthenZ exist and whilst are often feature-complete, 
they require significant effort to operate and integrate with.

Vespa are interested in our progress and are tentatively interested in being a
design partner.

#### Case Study: Redacted 1 (No Solution)

Redacted 1 is a post-acquisition startup that offers a SaaS product in the
hospitality sector. They have around 20 engineers.

They are not a Teleport customer and have not previously considered Teleport
for access.

Their architecture is simpler than Vespa's. There is 30~ microservices that run
within a single GKE cluster. A few other services run on GCP's serverless 
offering. These are primarily written in Go and host HTTP APIs. They have
no need to segregate tenants.

They fall within the second category and do not have an existing workload
identity solution. Communication between services is secured with a long-lived
shared secret. No transport layer security is used and all services have access
to all RPCs exposed by other services.

As redacted has matured, they have become more aware that their current solution
is inadequate. Their concerns include:

- The long-lived nature of the token means that if compromised, bad actors would
  have access to all services for a long period of time.
- The lack of transport layer security means that a bad actor with access to
  the network could easily intercept and determine this token.
- As the number of services has grown, the need to manage what services have
  access to has increased. They recently undertook work to provide each service
  with its own database credentials and put in place access control to limit
  each service to its own database. Now they face this problem with service to
  service requests.
- A lesser concern is the inability to audit which service is responsible for
  an action.

When asked how they would approach solving this problem, they were unsure. They
were not aware of SPIFFE and would start by investigating the identity solutions
offered by GCP.

An ideal solution for them would:

- Involve a limited amount of effort to implement and operate. Security is still
  not the highest priority in their organization, but they are motivated enough
  to be aware there is a problem.
- Provide good documentation and support with implementation. Solutions like
  SPIRE are "scary" because of a lack of security and ops experience within the
  organization.

An organization like redacted highlights how an untapped market for workload
identity exists - maturing technical organizations. As these organizations
mature, they become aware that their current practices are inadequate. They
often lack security expertise and therefore solutions like SPIRE are
unapproachable. It's important to realise that whilst these issues have become a
higher priority, they are often still not the highest priority, and therefore a
solution which is too complex or expensive will not be adopted.

Redacted is just one example of a SaaS startup going through the process of
maturing, and many startups exist in a similar position. There is no evidence
that to suggest that redacted's security practices are unusual.

### MVP Scope

**In Scope**

- Providing authentication for service to service requests using x509
  certificates.
- Providing the materials necessary for services to validate the identity of
  other services.
- Compatibility with the SPIFFE Workload Identity API to provide an easy way to
  integrate with Machine ID Workload Identity.
  - Compatibility with `go-spiffe` and other SPIFFE libraries.
  - Drop in compatibility for `spiffe-helper`.
- Issuing short-lived certificates and rotating these seamlessly for workloads

**Out of Scope**

- Providing Authorization for Service to Service requests
  - This would add significantly to the scope and require an in-depth
    investigation of how best to integrate with a number of different languages
    and frameworks.
  - Customers likely already have a custom or off-the-shelf authorization
    implementation that we can plug into. Intending to plug into existing
    implementations reduces the work to migrate to Machine ID Workload Identity.
  - TAG is still under development, but could be later extended to support
    making decisions on Service to Service authorization.
- Auditing of Service to Service requests
  - Since Teleport will not be proxying these requests, it’s significantly more
    challenging to provide audit capabilities.
- JWT SVIDs
  - This adds to the scope of the MVP and is not required for most service to
    service use-cases.
- Federation (e.g Trusted Clusters)
- Workload Attestation
  - This adds to the scope of the MVP and is not required for basic use-cases.
- Support embedding `tbot` within a workload implementation.
  - The agent/sidecar model proposed by this RFD is sufficient for most
    use-cases and pursuing an embedded model would increase scope significantly.

### Risks and Mitigations

- The MVP may not cover enough functionality to be useful to customers.
  - Working closely with a design partner can allow us to reduce this risk as we
    can ensure that the MVP covers their use-cases.
- A high level of friction is involved from migrating from any existing Workload
  Identity solution to another. It may be a challenge to encourage users to
  adopt our product..
  - This is mitigated by pursuing compatibility with a common standard, SPIFFE.
    By doing this, friction is reduced as Machine ID can be dropped in without
    modifications to workload implementations.
  - Further mitigation can be achieved by focussing on converting customers
    without an existing workload identity strategy. They will receive the most
    benefit by adopting Machine ID Workload Identity and this means there is
    less pressure to compete with existing solutions.
- Competition from other players in the workload identity space.
  - Open source and free:
    - SPIRE
    - AthenZ
  - Commercial:
    - spirl.com
      - Pre-launch. First announced in June 2023.
      - Led by one of the key contributors of SPIRE.
      - SPIFFE compatible and likely a commercialised offering of SPIRE.
      - Has the potential to be a strong competitor.
    - aembit.io
      - $16.6 MM raised in March 2023
      - Launched product
    - keyfactor.com
    - venafi.com
    - [HPE Cosigno](https://www.hpe.com/us/en/software/service-identity-management.html)
  - Many platforms now offer a native form of workload identity. This is not
    convenient for complex heterogeneous environments but may be enough for
    simpler use-cases. These are often free.
- The size of the market may not be as large as we anticipate.
  - Workload identity is a fairly immature yet growing space.
  - The current complexity of existing workload identity solutions prevents
    smaller organisations from adopting it and this hampers the growth of the
    market. If we nail the product, we may be able to expand this market into
    smaller businesses.
- There may not be as much value in a commercial workload identity solution for
  smaller organizations as we anticipate.
  - This impacts the willingless to perform the work to implement and the
    willingness to pay for the product.
- We may not have a strong understanding of how best to price this.
  - Fairly few public pricing models are available for similar products, in part
    due to the immaturity of the space.
  - We can mitigate this by offering an MVP for free and then exploring pricing
    options with early adopters based on the value they receive from the
    product.

### Billing

Whilst we may offer the MVP on a free basis, this product will eventually
need to be monetized.

There's roughly two metrics that could be easily used for billing:

- Number of SVIDs issued over the billing period.
  - Easy to implement.
  - Encourages poor security practices. SVIDs should be short-lived, but
    charging per SVID issued rewards customers for increasing the TTL of issued
    SVIDs. It would also encourage trying to share a single issued SVID between
    multiple instances of a service.
  - Scales with the number of workloads that a customer has.
  - Could integrate with our existing billing model as a TIA or as a distinct
    unit.
- Number of unique SPIFFE IDs (identities) over the billing period
  - Relatively simple to implement using usage events.
  - Charges based on the unique SPIFFE IDs that are issued SVIDs.

    Usually, a unique SPIFFE ID exists for each unique service.

    In simpler systems, customers are likely to have a single SPIFFE ID shared
    by replicas of a service. They would only be charged once for this service,
    regardless of the number of replicas.

    In complex multi-tenanted systems, customers are likely to have SPIFFE IDs
    namespaced by the tenant and service. This means billing scales with the
    number of tenants and the complexity of the system (the number of services
    required for a tenant)
  - Billing correlates with the complexity of a customer's system, rather than
    with the scale of the system - although to some extent, a system operating
    at greater scale will be more complex.
  - Could integrate with our existing billing model as a TPR or as a distinct
    unit.

Initial discussions with Product and Revenue suggest we will:

- Bill a TPR for each unique SPIFFE ID.
- Bill a TIA for each SVID issued.

### Success

#### Measuring Success

When a SPIFFE identity is issued, an anonymized usage event will be submitted
to PostHog. This will include:

- The anonymized ID of the Teleport cluster/license.
- The anonymized name of the bot that made the request.
- The anonymized SPIFFE ID that was requested.

Using this usage event, we can track the following measurements of success:

- The number of customers that have adopted the product and the retention of
  these customers.
- The number of unique SPIFFE IDs that have been issued. This is a useful
  measure of the complexity of customer deployments and the extent to which
  they have implemented.
- The number of certificates that have been issued. This is a useful measure of
  the scale of customer implementations of Teleport Workload Identity.

In addition to measuring adoption of the product itself, we should also measure
pageviews of the documentation and other content such as blog posts as
indicators of general interest in the space.

#### Strategy

Initially, a successful MVP would see the product adopted and retained by a
small handful of trial customers in a "white-glove" support model. We may wish
to reach out to our existing customer base to determine levels of interest in
being one of these trial customers.

The "white-glove" model provides us with immediate feedback on the product and
provides a quick feedback model, allowing us to make improvements to the product
and documentation. It also allows us to more concretely understand the use-cases
and pains organisations have with legacy methods or competing solutions. This
can inform future product direction and marketing efforts.

With the MVP refined, we can then begin to measure the organic interest and
adoption of the product over a more extended period - whilst this takes place
we may wish to take the pedal off of investing further significant efforts.

In an ideal scenario, a number of customers will adopt the product and we will
see healthy organic interest in documentation/blog posts. Even if retention
is not as strong as we would like, this will be a good indicator that the
product is viable and that we should continue to invest in it.

If we do not see indicators of organic interest
(e.g pageviews, signups, implementations) then we may wish to review the
viability of the product.

## Technical Design

**Reference material**

- https://spiffe.io/docs/latest/spiffe-about/spiffe-concepts/

A workload is a service that requires the ability to identify itself to other
services or to verify the identity of other services.

SPIFFE is an open standard for workload identity that specifies how to encode
workload identity and how workloads should request these identities.

A SPIFFE ID is a URI that identifies a specific workload identity.

A SPIFFE ID can be encoded into a x509 certificate. These abide by the SPIFFE 
specification and are known as x509 SPIFFE Verifiable Identity Documents
(SVIDs). These are used by services as part of mTLS.

Workloads will request certificates from a `tbot` agent running locally to the
workload, typically on the same host, via an API. In addition to these 
certificates, they will receive a trust bundle containing the CA certificates
that are necessary to validate the identity of other workloads.

### SPIFFE ID

**Reference material**

- https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md
- https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md

SPIFFE IDs consist of two elements, the trust domain and the path. For the
purposes of Teleport Workload Identity, the trust domain will be the Teleport
cluster. The path then identifies a specific workload within the trust domain.

For example, given a Teleport cluster called `teleport.example.com` and a
workload identity of `foo/bar`, the SPIFFE ID would be:
`spiffe://teleport.example.com/foo/bar`.

### x509 SVIDs

**Reference material**

- https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md
- https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md

An x509 SVID is an x509 certificate with a SPIFFE ID encoded into it. The SPIFFE
ID is placed within the URI SAN.

These **may** contain additional SANs of other types. In order to be compatible
with the standard TLS verification process, we should allow the user to
configure SANs to be additionally included within the x509 SVID.

The x509 SVIDs will be signed by a new purpose-specific CA. This is because
x509 SVIDs are used for both host and client identification, and using the
host or user CA would incorrectly imply otherwise.

The following is an example of creating a x509 SVID in Go:

```go
package svid

func makeSVID() {
  spiffeID := &url.URL{
    Scheme: "spiffe",
    Host:   "teleport.example.com",
    Path:   "/foo/bar",
  }
  svid := &x509.Certificate{
	  // SerialNumber can be generated randomly.
    SerialNumber: big.NewInt(0),
    NotBefore:    time.Now(),
	  // NotAfter can be adjusted to control the lifetime of the certificate.
	  // This will be controllable by the user and will typically be short-lived.
    NotAfter:     time.Now().Add(time.Hour),
    
    // SPEC(X509-SVID) 4.1. Basic Constraints:
    // - leaf certificates MUST set the cA field to false
    IsCA: false,
    
    // SPEC(X509-SVID) 4.3. Key Usage:
    // - Leaf SVIDs MUST NOT set keyCertSign or cRLSign.
    // - Leaf SVIDs MUST set digitalSignature
    // - They MAY set keyEncipherment and/or keyAgreement;
    KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageKeyAgreement,
    // SPEC(X509-SVID) 4.4. Extended Key Usage:
    // - Leaf SVIDs SHOULD include this extension, and it MAY be marked as critical.
    // - When included, fields id-kp-serverAuth and id-kp-clientAuth MUST be set.
    ExtKeyUsage: []x509.ExtKeyUsage{
        x509.ExtKeyUsageClientAuth, // id-kp-clientAuth
        x509.ExtKeyUsageServerAuth, // id-kp-serverAuth
    },
    
    // SPEC(X509-SVID) 2. SPIFFE ID:
    // - The corresponding SPIFFE ID is set as a URI type in the Subject Alternative Name extension
    // - An X.509 SVID MUST contain exactly one URI SAN, and by extension, exactly one SPIFFE ID.
    // - An X.509 SVID MAY contain any number of other SAN field types, including DNS SANs.
    URIs: []*url.URL{
        spiffeID,
    },
    DNSNames: []string{},
  }
}
```

In addition, the following Teleport specific fields should be encoded in the
x509 SVIDs:

- The username of the Bot that generated the x509 SVID.
- The name of the Teleport cluster that the x509 SVID was produced by.

The generation of x509 SVIDs will be requested by a Bot using a new RPC:

```protobuf
syntax = "proto3";

package teleport.machineid.v1;

service WorkloadIdentityService {
  // GenerateX509SVID generates a signed x509 SVID for the given SPIFFE ID.
  rpc GenerateX509SVID(GenerateX509SVIDRequest) returns (GenerateX509SVIDResponse) {};
}

// The request for an individual x509 SVID.
message SVIDRequest {
  // The path that should be included in the SPIFFE ID.
  string spiffe_id_path = 1;
  // The DNS SANs that should be included in the x509 SVID.
  repeated string dns_sans = 2;
  // The IP SANs that should be included in the x509 SVID.
  repeated string ip_sans = 3;
  // A hint that provides a way of distinguishing between SVIDs. These are
  // use configured and are sent back to the actual workload. 
  string hint = 4;
}

// The generated x509 SVID.
message SVIDResponse {
  // The signed x509 SVID.
  bytes certificate = 1;
  // The full SPIFFE ID that was included in the x509 SVID.
  string spiffe_id = 2;
  // The hint that was included in SVIDRequest in order to allow a workload to
  // distinguish an individual SVID.
  string hint = 3;
}

// The request for GenerateX509SVID.
message GenerateX509SVIDRequest {
  // The SVIDs that should be generated. This is repeated to allow a bot to
  // request multiple SVIDs at once and reduce the number of round trips.
  repeated SVIDRequest svids = 1;
}

// The response for GenerateX509SVID.
message GenerateX509SVIDResponse {
  // The generated SVIDs.
  repeated SVIDResponse svids = 1;
}
```

### Access Control

Access control of the provisioning of x509 SVIDs has two parts:

- controlling the SPIFFE IDs that a Bot should be able to request using
  the `GenerateX509SVID` RPC.
- controlling which SPIFFE IDs are available to a workload making
  requests to `tbot` over the SPIFFE Workload API.

This section covers the access control of the `GenerateX509SVID` RPC. The
access control for the SPIFFE Workload API is covered in the SPIFFE Workload
API section.

#### Option: `spec.allow.rules` resource

To control which SPIFFE IDs a Bot can request, a fake `svid` resource could be
specified within a role's `spec.allow.rules`. This is similar to how host
certificates are handled today.

For example, to grant access to a specific SPIFFE ID:

```yaml
kind: role
metadata:
  name: foo-bar-svid-requester
version: v5
spec:
  allow:
    rules:
    - resources:
      - svid
      verbs:
      - create
      where: 'equal(svid.path, "/foo/bar")'
```

It is important that users are able to grant access to a group of SPIFFE IDs
as it will be common that a Bot will be managing the identity of multiple
workloads. Creating a role for each of this IDs would be cumbersome.

This can be done using a wildcard and the `match` function:

```yaml
kind: role
metadata:
  name: foo-wildcard-svid-requester
version: v5
spec:
  allow:
    rules:
    - resources:
      - svid
      verbs:
      - create
      where: 'match(svid.path, "/foo/*")'
```

This would allow a Bot to request any SPIFFE ID that starts with `/foo/`.

Controlling which SPIFFE IDs are available to a workload making requests to
`tbot` will be configured within `tbot` and this is described in the
SPIFFE Workload API section.

#### Option: `spec.allow.svids`

As only a single `where` clause can be specified within a resource rule, this is
likely to make it difficult to grant multiple different SVIDs in a single role.

Instead, it may make more sense to introduce a new sub-schema into
Role, `allow.svids`:

```yaml
kind: role
metadata:
  name: svid-generator
version: v5
spec:
  allow:
    svids:
    - path: "/bar/overseer"
    - path: "/foo/*"
      dns_sans:
        - "*.foo.svc.example.com"
      ip_sans:
        - 10.10.10.0/24
    - expression: 'match(svid.path, "/baz/*")' 
```

This makes the use of expressions unnecessary and allows for a simpler
management experience where a Bot may need to issue multiple SVIDs.

#### Decision

Whilst introducing `spec.allow.svids` may be more complex, it provides a 
better user experience for a non-trivial workload identity setup and therefore
is the preferred option.

### Issuing x509 SVIDs using `tbot`

#### `spiffe-x509-svid` output

A new output type, `spiffe-x509-svid`, will be added to the `tbot`
configuration. This will configure `tbot` to request a x509 SVID for a static
SPIFFE ID and write this to a destination.

Whilst this is not a typical SPIFFE implementation, it is a natural step towards
the full implementation. It will allow us to debug the SVID generation process
and provides an escape hatch when a SPIFFE Workload API client SDK is not
available for a specific language in which a user intends to implement workload
identity.

Example configuration:

```yaml
outputs:
- type: spiffe-x509-svid
  destination:
    type: directory
    path: /opt/machine-id
  svids:
  - path: /tenant/foo/service/bar
    hint: foo-bar-bizz
    sans:
      dns: ["bar.foo.svc.cluster.local"]
```

This will replicate behaviour similar to an existing SPIFFE
utility,
[`spiffe-helper`](https://github.com/spiffe/spiffe-helper/tree/main) - where
possible we should aim to be compatible with the structure of the directory
that this utility produces. This will make it easier for users to migrate to
`tbot` from this utility.

#### SPIFFE Workload API

**Reference material**

- https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE.md
- https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_API.md
- https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_Endpoint.md
- https://github.com/spiffe/spiffe/blob/main/standards/workloadapi.proto

The SPIFFE Workload API is a standardised gRPC service that is used by workloads
to request x509 SVIDs. The protobuf specification for this service can be found
at https://github.com/spiffe/spiffe/blob/main/standards/workloadapi.proto .

A new section of the `tbot` configuration will be introduced `services`. This
will allow long-lived services to be defined, and could be reused in future
to provide services other than a SPIFFE Workload API endpoint. If a service is
defined, `tbot` will error when executed with `--oneshot`.

It will be possible to define multiple SPIFFE Workload API endpoints within
`tbot`. Each endpoint will be configured with:

- A Unix or TCP socket to listen on.
- A list of SVIDs that should be provided to workloads making requests to
  this endpoint.

Example configuration:

```yaml
# services is a new section of the tbot configuration that controls long-lived
# services that tbot should run.
services:
  # type specified as spiffe to indicate a SPIFFE Workload API endpoint.
- type: spiffe
  # listen specifies where the listener should be opened. This is prefixed with
  # unix:// or tcp:// to indicate the type of listener.
  listen: unix:///var/run/my-spiffe-endpoint.sock
  # svids is a list of SVIDs that should be provided to workloads making
  # requests to this endpoint.
  svids:
    # path is the path segment of the SPIFFE ID that will be appended to the
    # trust domain.
  - path: /tenant/foo/service/bar
    # hint is an optional hint provided to the workload in the response from
    # the endpoint. This is useful in cases where a workload may receive
    # multiple SVIDs.
    hint: foo-bar-bizz
    # sans is a list of SANs that should be included in the x509 SVID.
    sans:
      # dns is a list of DNS SANs that should be included in the x509 SVID.
      dns: ["bar.foo.svc.cluster.local"]
      # ip is a list of IP SANs that should be included in the x509 SVID.
      ip: ["10.10.10.10"]
- type: spiffe
  listen: tcp://0.0.0.0:3000
  svids:
  - path: /bar/buzz
```

When `SpiffeWorkloadAPI.FetchX509SVID` is invoked on the `tbot` Workload API
service:

- `tbot` will start a goroutine to manage the connection. This will manage
  fetching the initial SVIDs, but also re-fetching SVIDs before they expire or
  if the `svid` CA is rotated.
- `tbot` will invoke `GenerateX509SVID` RPC on the Auth Server for each of the
  SVIDs that are configured for the endpoint.
- `tbot` will then send a response to the workload containing the SVIDs and
  the cached trust bundle for the Teleport cluster.
- `tbot` will then regularly re-issue the SVIDs before they expire or if the
  `svid` CA is rotated. As `FetchX509SVID` is a streamed RPC, these can then
  be pushed to the workload.
- When the workload closes the connection, `tbot` will close the goroutine
  managing the connection and no longer fetch SVIDs before they expire.

Outside the scope of the MVP is `tbot` pre-empting the generation of SVIDs
that are available over the SPIFFE Workload API endpoints. This would reduce the
time that it takes to return a SVID to a workload and reduce the dependency on
the availability of the Auth Server.

In addition, the `SpiffeWorkloadAPI.FetchX509Bundles` RPC will be implemented.
This will not return SVIDs and will instead return the trust bundle and any
updates to this(e.g if the `svid` CA is rotated). This will be implemented in a
similar way to `FetchX509SVID`.

### Implementation Stages

Implementation should be broken into roughly the following PRs:

- Write `WorkloadIdentityService` protobuf specification.
- Implement `GenerateX509SVID` RPC and access control on the Auth Server as
  part of a new `WorkloadIdentityService`.
- Implement `spiffe-x509-svid` output type in `tbot`.
- Preparatory work for long-lived services in `tbot`.
- Implement `spiffe` service in `tbot`.
- Document configuration and provide an example Go repository.

### Security Considerations

#### Auditing

An audit event should be added that is emitted when `GenerateX509SVID` is
invoked. This should include:

- The SPIFFE ID that was requested.
- The unique ID of the certificate that was generated.
- Any additional certificate fields that were requested (e.g DNS SANs).
- The name of the user that made the request.
- Whether the user that made this request was a Bot.
- The IP address and user agent from which this request was made.
- Whether the generation was rejected or approved.

In addition, `tbot` should emit a log message when it receives a request for
SVIDs from a workload. This should include:
- The Workload API endpoint that the request was made against.
- Any additional information that can be discerned of the caller
  (e.g IP address, user agent).
- The SPIFFE IDs that were returned.
- Any additional certificate fields that were retruned (e.g DNS SANs).

Hypothetically, this additional information with the `tbot` log message could
be provided to the Auth Server for inclusion in the audit event. However, this
information would not be verifiable (a malicious implementation of `tbot` could
provide falsified information). As such, this is left out of scope of the MVP.

#### Protecting the SPIFFE Workload API Endpoints

In the initial design the SPIFFE Workload API endpoints will offer SVIDs to
any workload which can make a request to them. This is not unusual, and is how
other implementations of the SPIFFE Workload API operate when workload
attestation is not available.

The primary mitigation to this is the introduction of workload attestation,
and requiring the workloads to present some kind of verifiable identity before
they can receive a SVID. However, this is not available in all environments and
is left out of scope of the MVP. Further information about this can be found in
the Future Improvements section.

The secondary mitigation is to encourage the use of unix sockets for the
listener. These can then be secured by filesystem permissions as are already
used for `tbot` destinations. Existing code which sets ACLs and checks the
permissions of destinations can be re-used for this.

The use of TCP listeners should be discouraged. Unfortunately, we cannot
entirely omit TCP listeners as they will be necessary in some environments. We
should emit a warning when these are used and when listening on a non-loopback
interface. In addition, we can require a flag to be provided which acknowledges
the security implications of this
(e.g `--insecure-tcp-listener-enabled`).

#### Security of x509 SVID Private Key

One notable element of the design of SPIFFE is that rather than a workload
submitting a public key to the SPIFFE Workload API endpoint, it instead receives
a private key from the endpoint.

This means that an attacker with a deep level of access to `tbot` would have the
ability to act as any workload that has used `tbot` to request SVIDs.

We should ensure that each workload receives a different private key from
`tbot`. We should also ensure that this key is not persisted to disk and is
not included in any logging.

#### Short Certificate TTL

As with most Teleport issued certificates, the x509 SVIDs should be short-lived.
This reduces the impact of a compromised certificate.

A degree of configuration of the TTL will be offered but a hard limit will be
enforced to guide users to best practices. The tooling around SPIFFE should
make this a non-problem.

#### Locking

Locking a Bot will prevent it from being able to request SVIDs. Existing SVIDs
will continue to be valid until they expire.

Supporting locking specifically for SVIDs is out of scope of the MVP. This
would leverage the Certificate Revocation List (CRL) functionality of the
SPIFFE Workload API. This is not yet implemented in competing implementations
like SPIRE, with the short TTL generally considered a sufficient mitigation.

### Future Improvements

#### Workload Attestation

**Reference material**:

- https://spiffe.io/docs/latest/spire-about/spire-concepts/#workload-attestation

One key element out of scope for the MVP design is workload attestation. This is
a level of authentication and authorization enforced on requests made by a
workload to the SPIFFE Workload API endpoints.

In the current design, any workload which can make a request to the SPIFFE
Workload API endpoint receives the SVIDs that are configured for that endpoint.
To ensure this is secure, `tbot` must therefore be running on the same host as
the workload or within a secure network.

Workload attestation requires that the workload submit some form of a 
verifiable identity to SPIFFE Workload API endpoint and the characteristics of
this identity control what SVIDs will be issued to it. For example, allowing
a specific SPIFFE ID to be issued to a workload running with a specific 
Kubernetes service account.

Typically, workload attestation is enforced by the Workload API endpoint server,
rather than the SVID CA itself. This creates a two layer access control model
where a bot's ability to request SVIDs is controlled by the Auth Server and the
issuing of these SVIDs to specific workloads is controlled by `tbot`. This is
beneficial in cases where the Auth Server does not have access to things
required to complete the attestation (e.g the public signing key of the
Kubernetes cluster).

Workload attestation is similar to delegated joining within Teleport and
therefore we could leverage existing code for the foundations of this.

A hypothetical configuration of workload attestation in `tbot` would look like:

```yaml
services:
- type: spiffe
  listen: unix:///var/run/my-spiffe-endpoint.sock
  svids:
  - path: /tenant/foo/service/bar
    hint: foo-bar-bizz
    sans:
      dns: ["bar.foo.svc.cluster.local"]
    attestation:
    - type: kubernetes
      service_account: bar
    - type: gcp
      service_account: service-bar
```

Whilst this is typically enforced by the Workload API endpoint server, we could
break from this design and move the workload attestation into the Auth Server.
This creates a simpler "single-layer" access control model but complicates
scenarios where no form of workload attestation is available (e.g onprem).

As workload attestation allows for easier deployments in more complex
environments, it is therefore one of the features that should follow the
release of the MVP.

#### Hosted SPIFFE Workload API Endpoints

Whilst the current design depends on `tbot` running in proximity to the workload
in order to secure the SPIFFE Workload API endpoints, once workload attestation
has been implemented, it will be possible to host these endpoints remotely in
some circumstances.

This could be useful in offering an "ops-free" experience on Teleport Cloud
where the user does not need to deploy `tbot` themselves. However, the workload
would be more reliant on a connection to the Auth Server.

#### JWT SVIDs

For the MVP, the only type of SVID that will be issued is x509. However, SPIFFE
specifies the structure of a second type, JWT SVIDs.

These encode a SPIFFE ID similar to x509 SVIDs. They also must include the
SPIFFE ID of the intended audience of the JWT SVID. This is useful in cases
where client certificate authentication is not possible (e.g TLS termination).

This has been omitted from the MVP as in most microservice architectures, we
expect that mTLS will be possible. The MVP Workload Identity endpoint will
return the gRPC NOT_IMPLEMENTED error for requests for JWT SVIDs.

In a future release, we may wish to add support for JWT SVIDs. These would
be signed by the same CA as the x509 SVIDs but additionally the signing key
should be made publicly accessible in JWKS format on an endpoint on the
Auth Server.

#### Federation

**Reference material**:

- https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md

In the MVP, we will not support federation. This is the ability for workloads
in one trust domain to trust workloads in another trust domain.

In a future iteration, we may wish to support federation via Teleports
Trusted Cluster mechanism. However, we should be aware that this limits cases
where the trust is reciprocal - the Trusted Cluster mechanism is based on a
root-leaf model which is potentially incompatible with this.

In addition, we may wish to consider supporting federation with non-Teleport
trust domains. In a first iteration, this could be one-way (e.g the non-Teleport
trust domain trusts Teleport) and in a future iteration, this could be
reciprocal. This enables cases where some of the workloads are managed by an
existing SPIFFE solution.

If a reciprocal solution is explored, it is important to ensure that we do not
create a confusing UX where Trusted Clusters are generally root to leaf but
the behaviour for SPIFFE is reciprocal.

#### Envoy SDS Support

Envoy is common tool used in the workload identity space. It is deployed as a
sidecar to a workload and manages making mTLS connections to other workloads.
This means that no changes need to be made to a workload implementation itself.

Envoy is not compatible with the SPIFFE Workload API and instead has its own
defined API for providing credentials, the Secret Discovery Service (SDS). The
SPIRE agent implements this API to bridge the gap between SPIFFE and Envoy,
providing the Envoy sidecar with the credentials to use when making and
receiving credentials.

This functionality is particularly useful in environments where the workload
implementations are varied (e.g multiple languages) or where workloads are
legacy and modification is avoided.

Offering an SDS API will be out of scope for an initial implementation. However,
it should be considered as one of the task that will likely follow the release.
We should monitor for interest in this feature to validate that it is desired 
by our target audience.
