---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 231 - New Oracle Join Method

## Required Approvers

* Engineering: @rosstimothy && @strideynet
* Security: @rob-picard-teleport
* Product: @klizhentas

## What

This RFD proposes a new join method for OCI (Oracle Cloud Infrastructure)
compute instances.

## Why

A major drawback of our
[current Oracle join method](https://goteleport.com/docs/enroll-resources/agents/oracle/)
is that it requires all joining instances to be added to a dynamic IAM group,
and granting the following policy to the group:

```
Allow dynamic-group '<identity domain>'/'join-teleport' to inspect authentication in tenancy
```

This policy is used to sign requests to the following API:

```
https://auth.<region>.oraclecloud.com/v1/authentication/authenticateClient
```

This API is undocumented by Oracle, and when Oracle learned we were using it for
this purpose and requiring this policy on all instances, they assisted us in
finding another method using supported APIs that will have public documentation
soon.

## Details

### UX

The new design will strictly improve UX as compared to the existing Oracle join
method.
Instances will not be required to have membership in any IAM groups and no IAM
policies will be required.
This will decrease the configuration burden and eliminate security questions of
why the policy is necessary.

Configuration on the Teleport side will not change.
The new design will continue to leverage existing Oracle join tokens that
Teleport admins may have configured.
No configuration options will need to change on the agent or bot.

### Design

The new method will leverage OCI instance identity certificates, which are made
available to all OCI compute instances via the IMDS (Instance Metadata Service).
The best docs available on this certificate are the FAQ entries
[here](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm#faq).
This certificate can be retrieved on any instance with the following `curl` command:

```
curl -H 'Authorization: Bearer Oracle' http://169.254.169.254/opc/v2/identity/cert.pem
```

An example instance identity certificate includes the following:

```
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            ca:49:16:9e:30:1f:5a:13:d3:09:2d:11:1b:fb:38:a4
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: OU=opc-device:36:9f:ed:ff:cc:b9:a4:a1:ea:71:76:ae:6a:05:d3:fb:c3:70:2a:9c:d9:77:05:ed:d3:3c:d5:50:f7:64:04:e0, CN=PKISVC Identity Intermediate r2
        Validity
            Not Before: Oct 24 21:39:42 2025 GMT
            Not After : Oct 24 23:40:42 2025 GMT
        Subject: CN=ocid1.instance.oc1.phx.<random-id>, OU=opc-certtype:instance, OU=opc-compartment:ocid1.compartment.oc1..<random-id>, OU=opc-instance:ocid1.instance.oc1.phx.<random-id>, OU=opc-tenant:ocid1.tenancy.oc1..<random-id>
```

The subject OU fields `opc-instance`, `opc-compartment`, and `opc-tenant`
describe the instance's ID, compartment, and tenancy.

In this design the instance provides the identity certificate, a chain of
trust for the certificate, and a challenge signature to the Teleport cluster.
After the Teleport cluster verifies the certificate was legitimately issued by
Oracle and the instance proves ownership of the private key corresponding to
the certificate, Teleport will allow the instance to join the cluster if the
identity matches an allow rule in a configured Teleport provision token.

#### Verifying the certificate chain

All instance identity certificates are issued and signed by an intermediate
CA, which is itself issued by one of Oracle's Instance
Identity Root authorities.

The instance cert and the intermediate CA chain are themselves available via
the instance IMDS at the following URLs:

```
http://169.254.169.254/opc/v2/identity/cert.pem
http://169.254.169.254/opc/v2/identity/intermediate.pem
```

The joining instance will fetch these and include them in the join request.

However, the Instance Identity Root CAs are only available via an authenticated endpoint:

```
https://auth.<region>.oraclecloud.com/v1/instancePrincipalRootCACertificates
```

Requests to this endpoint must include client authentication, they must be
signed by any authenticated OCI caller.
This prevents a Teleport Auth service (without its own OCI credentials) from
directly hitting this endpoint to get the root CAs.

Teleport needs some way to trust that the root CAs are legitimate, we can't
just let the joining instance fetch the CAs and include them in the request,
that would be trivially attacked by creating and sending fake root CAs.

The idea for fetching the root CAs in a trusted manner is the following:

1. The joining instance will create an HTTP request to `https://auth.<region>.oraclecloud.com/v1/instancePrincipalRootCACertificates`
1. The joining instance will sign the request with its own instance credentials using Oracle's Go SDK.
1. The joining instance will not send the request, but dump it to a byte stream.
1. The joining instance will send the raw signed request to Teleport in the join message.
1. The Teleport Auth service will parse the signed HTTP request and verify the host is a legitimate oracle API endpoint:
   * the region must be a valid OCI region
   * the host must the actual host for the OCI auth API in that region as determined by Oracle's Go SDK
   * the path must be exactly `/v1/instancePrincipalRootCACertificates`
1. The Teleport Auth service will send the pre-signed HTTP request to the Oracle API over HTTPS.
1. As the Teleport Auth service has verified the legitimacy of the API endpoint and established TLS, it can trust the returned root CAs.

The Auth service handling the join request will then build a certificate chain
including the intermediates sent by the client and the roots fetched from
Oracle and verify the signature on the instance identity certificate.

This process of signing an API request on the instance and having the Auth
service send the pre-signed request to the cloud API is not entirely novel,
it's quite similar to the way our existing Oracle and AWS IAM join methods
work.

Note: this `https://auth.<region>.oraclecloud.com/v1/instancePrincipalRootCACertificates`
endpoint is currently undocumented, we learned of it in direct discussion with
Oracle, they have said it will be documented soon.

#### Verifying ownership of the instance identity certificate

The joining instance must prove that it holds the private key associated with
the instance identity certificate, or at least that it can sign messages using
that private key.
This avoids treating the certificate as a password sent to the Auth service,
the instance proves ownership of the private key without sending it anywhere.

Teleport join methods use bidirectional-streaming RPCs, which allows the Auth
service to issue a "challenge" that the instance must sign with its private
key.
The Auth service can then verify the signature against the public key included in
the instance identity certificate.

The message flow will look like:

1. client->server: ClientInit
1. client<-server: ServerInit
1. client->server: OracleInit
1. client<-server: OracleChallenge
1. client->server: OracleChallengeSolution
1. client<-server: Result

The challenge will be a cryptographically random string generated by Go's
crypto/rand module with 256 bits of randomness.

The private key is made available via IMDS at `http://169.254.169.254/opc/v2/identity/key.pem`.
It is a 2048-bit RSA key.
The instance retrieves the key from the IMDS and generates an RSA-PSS
signature over the challenge, using SHA-256 as the message digest algorithm.

When verifying the signature, the Auth service will require the instance public
key is an RSA key of size >=2048 and <=4096 bits.

The Auth service will verify the challenge signature within the context of the
streaming RPC, which has a number of implicit benefits:

* the challenge is never stored or persisted
* the entire RPC has a timeout of 1 minute, meaning the challenge will be valid for <= 1 minute
* the challenge solution must be sent from the same host that initiated the
  request, to which the challenge was sent, as it's all in a single TCP stream.

### Security

* Explore DDoS and other outage-type attacks:

All join RPCs are rate-limited.
Requests for regional root CAs will be cached instead of repeated on each join attempt.

* Explore possible attack vectors, explain how to prevent them
* If introducing new attack surfaces (UI, CLI commands, API or gRPC endpoints),
* If introducing new auth{n,z}, explain their design and consequences
  consider how they may be abused and how to prevent it
* If using crypto, show that best practices were used to define it

These are covered in the design above.

* If frontend, explore common web vulnerabilities

N/A

### Privacy

This design does not include any additional privacy concerns in addition to our current Oracle join method.

OCI instance IDs, compartment IDs, and tenant IDs of joining instances will be logged.

### Proto Specification

```proto3
// OracleInit is sent from the client in response to the ServerInit message for
// the Oracle join method.
//
// The Oracle method join flow is:
// 1. client->server: ClientInit
// 2. client<-server: ServerInit
// 3. client->server: OracleInit
// 4. client<-server: OracleChallenge
// 5. client->server: OracleChallengeSolution
// 6. client<-server: Result
message OracleInit {
  // ClientParams holds parameters for the specific type of client trying to join.
  ClientParams client_params = 1;
}

// OracleChallenge is sent from the server in response to the OracleInit message from the client.
// The client is expected to respond with a OracleChallengeSolution.
message OracleChallenge {
  // Challenge is a a crypto-random string that should be included by the
  // client in the OracleChallengeSolution message.
  string challenge = 1;
}

// OracleChallengeSolution must be sent from the client in response to the
// OracleChallenge message.
message OracleChallengeSolution {
  // Cert is the OCI instance identity certificate, an X509 certificate in PEM format.
  bytes cert = 1;
  // Intermediate encodes the intermediate CAs that issued the instance
  // identity certificate, in PEM format.
  bytes intermediate = 2;
  // Signature is a signature over the challenge, signed by the private key
  // matching the instance identity certificate.
  bytes signature = 3;
  // SignedRootCaReq is a signed request to the Oracle API for retrieving the
  // root CAs that issued the instance identity certificate.
  bytes signed_root_ca_req = 4;
}
```

### Scale

Requests for regional root CAs will be cached instead of repeated on each join attempt.

### Backward Compatibility

The existing Oracle join method will continue to be supported on the server
side for backward compatibility with older joining agents, until v20 (all v19
agents will support the new join method, and v18 agents can't join v20 auth
servers).

Newer agents joining to older Auth servers without support for the new join
method will fall back to using the old Oracle join method, until v19 (all v19
Auth servers will support the new join method, and v19 agents can't join v18
Auth servers).

We will document that the IAM policy required by the old join method will no
longer be required only if both the Auth server and the agent are both on a
version >= whichever version this gets released in.

Luckily this is being implemented at the same time as the new join gRPC service
described in [RFD 27e](https://github.com/gravitational/teleport.e/blob/master/rfd/0027e-auth-assigned-uuids.md).
The new join service will only support the new Oracle join method, the legacy
join service will continue to support the existing Oracle join method, and we
get all the above backward compatibility guarantees for free.

### Audit Events

Existing Instance Join audit events will be emitted when an instance succeeds
or fails to join the cluster.
These audit events will contain the instance OCID, compartment, and tenancy ID.

### Observability

We can add tracing to the Oracle API requests made on the Auth service used to
fetch the root CAs.

### Product Usage

Describe how we can determine whether the feature is being adopted. Consider new
telemetry or usage events.

### Test Plan

The test plan will continue to cover Oracle joining.
