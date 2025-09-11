---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 0027e - Auth Assigned Host UUIDs

## Required Approvers

* Engineering: @fspmarshall && @espadolini
* Security: @rosstimothy && @rob-picard-teleport

## What

When Teleport agents join a Teleport cluster, they should have a host UUID
assigned to them by the Auth service instead of chosen by the agent.
This host UUID should be included in the host certificates and the host should
only be able to heartbeat with the assigned UUID.
It should not be possible to change the host UUID except to completely re-join
the cluster and get a new UUID assigned by the Auth service.

## Why

Agents currently choose their own UUID when joining the cluster.
Agents report their chosen UUID to the Auth service when joining and the Auth
service issues certificates including that UUID.
The agent is then allowed to write "heartbeats" for resources under that UUID.

This can lead to security vulnerabilities if a malicious agent chooses a UUID
already assigned to another host or OpenSSH node.
This would allow the agent to overwrite resource heartbeats owned by the other
host, which means it could overwrite labels used for access control or other fields.

One consequence of this is that possession of a node join token allows the
holder to choose the UUID of an existing OpenSSH node and overwrite its `node` resource
to change its labels in order to grant unintended access
(https://github.com/gravitational/teleport-private/issues/1817).

While it's also possible to overwrite resource heartbeats for Teleport nodes,
apps, dbs, etc, in most cases the agent serving the resource ultimately makes
the access decisions.
An overwritten resource heartbeat would not affect the final access decision,
the agent only writes resource heartbeats, it does not read them.
However, overwriting a resource heartbeat can have other less-obvious impacts.
For example, a malicious agent could overwrite resource labels such that the
Auth service allows a user to create a resource access request for that
resource that the user would not otherwise be allowed to create.
If that resource access request is then approved (possibly automatically), the
user would be allowed to access the resource.

Forcing all agents to use a UUID assigned by the Auth service eliminates this
class of vulnerability.

## Details

### UX

This change should be an invisible security enhancement for most users.

The only user-facing change will be a new environment variable
`TELEPORT_UNSTABLE_NO_AGENT_ID_SELECTION`.

By default, auth-assigned UUIDs will only be required for agents connecting to
the new JoinService described in this RFD.
For backward compatibility to allow agents on slightly older versions to join a
cluster with a slightly newer Auth service, older agents will still be able to
connect to the legacy join endpoints and choose their own host UUID.
To disable the insecure legacy behavior, a new environment variable
`TELEPORT_UNSTABLE_NO_AGENT_ID_SELECTION`, if set to a truthy value, will
disable the legacy join endpoints to prevent any agent from selecting its own
host ID.
The legacy join endpoints will be removed after this feature has been released
for 2 major versions (in Teleport 20 if this is released in 18.x).

### Special Cases

#### EC2 Join Method

Nodes that join the cluster via the [EC2 join method](https://goteleport.com/docs/enroll-resources/agents/aws-ec2/)
do not use a UUID at all, they use a deterministic ID based on their EC2
instance identity document that looks like `<aws-account-id>-<ec2-instance-id>`.
This deterministic ID is used to reject the join request if another instance
with the same ID has already joined with the EC2 join method (it enables a fast
single-key lookup of node heartbeats).

Because the joining node proves that it is legitimately running on the matching
EC2 instance and we already explicitly check that the ID is not reused, this
deterministic ID is not vulnerable to the security concerns discussed in this
design, so nodes joining via the EC2 method will continue using this ID format.

In theory we could switch the ID format to a hash if the Instance Identity
Document using UUIDv5 to remove branches in e.g. node lookup where we have to
check if the host ID is in UUID or EC2 instance ID format, but existing
instances will continue with their existing host IDs so we can't actually
remove those branches.

#### EICE

EC2 Instance Connect Endpoint integration appears to use the same node ID
format as the EC2 join method
https://github.com/gravitational/teleport/blob/e5bb20815c21de0138e1f93063d04278530a7474/api/types/server.go#L554-L559

This feature is obsolete and deprecated, and permission to create the node
heartbeat with this ID format is not granted by a join token, so this feature
will not be considered in this design.

### Implementation

Rather than modifying the existing join endpoints and adding more branches, we
will implement a new gRPC JoinService for the new auth-assigned host ID flow.
There are a few justifications for this:

* If clients get a NotImplemented error attempting to use the new JoinService
  they can easily fall back to the legacy join RPCs.
* We avoid adding additional branching to the already complex join endpoints
* It gives us the opportunity to improve aspects of the existing, inconsistent
  join RPCs like including relevant details useful for logging in the initial
  message sent by the client so that agents failing to join are easier to track
  down.

The new JoinService will expose a single Join RPC implementing all join methods.
It will be a bidirectional streaming RPC in order to support our multiple
challenge-response based join methods currently using gRPC streaming.
A benefit of using a single RPC is that the client will no longer be required
to know the join method specified in the provision token, the cluster can
inform the client of the join method as part of the RPC, avoiding redundancy in
configuration where the join method currently has to be specified in both the
provision token and the agent configuration.

The JoinService will be implemented on the Proxy public gRPC listener address
to support unauthenticated clients connecting the proxy address for their
initial join.
This is either the web listen address or the reverse tunnel address in case the
minimal web service configuration is being used, it will be the same address
the current join service listens on.
It will also be implemented on the Auth service for clients that connect
directly to the Auth address rather than going through the proxy.
The server will include optional client authentication via mTLS for re-join
flows explained below.
Re-joining clients with existing credentials will always make an authenticated
request to the JoinService running on the Auth service, dialing Auth via the
proxy's mTLS routing if they connect to the proxy.
Only Instance and Bot certs will be allowed to make authenticated re-join
requests to maintain their assigned host ID.

The initial message sent from the joining client will include most parameters
currently included in `types.RegisterUsingTokenRequest`.
Notable omissions will include:

* `HostID`: removed from the request now that Auth will assign the host ID.
* `EC2IdentityDocument`: moved to a message specific to the EC2 join method.
* `IDToken`: moved to join-method specific messages
* `BotInstanceID`: currently extracted from client identity, but can be set in
  the request by the Proxy, and Auth only trusts it if set by the Proxy. Client
  should mTLS dial to the Auth service instead (possibly via Proxy TLS routing).
* `BotGeneration`: same as `BotInstanceID`
* `PreviousBotInstanceID`: it's never actually passed over gRPC, only internally

Currently each node goes through the join process for every local Teleport
service running on the node, plus the Instance service.
E.g. for a node running the SSH and App services, it goes through the token
join process 3 times (to get Instance, SSH, and App certs).
If each join process assigned a different host ID, this wouldn't work.
This logic will be changed so that it only goes through the join process once
to get the initial Instance certificates with a single unique host ID assigned
to the agent.
Each other service can then use the Instance certs to make an authenticated
connection to the GenerateHostCerts API regularly used for host cert refreshes
to get certs for the specific system role for that service.
The GenerateHostCerts API already allows Instance certs to get host certs for
any of the system roles present in the instance cert from when it first joined.

We may eventually be able to get rid of the service-specific certs entirely and
use the Instance cert for everything, we already reuse the Instance client for
all services in most cases.
However, in a few instances (proxy peering) we current require that the cert is
the actual Proxy cert and not the Instance cert, so this will be out of scope
for this design.

#### Re-joining with new system roles

A special case to consider is one where an additional service is added to the
agent configuration after its first start, where the new system role is
allowed by a new join token configured for the agent that was not allowed by
its original join token.
For example, if an agent first starts with only the SSH service enabled and a
token that only allows the Node role, it will first get two host certs: one for
the Instance service with the Instance and Node system roles, and another for
the SSH service with only the Node system role.
If the App service is later enabled and the agent is configured with a join
token that allows the App system role, here's what happens with the existing
logic:

1. The app service "joins" the cluster with the configured join token to get a
   new host cert with the App system role.
1. The process detects that the Instance cert does not contain the App system role.
1. The process uses the new App cert to call the AssertSystemRole RPC which
   creates a resource in the backend asserting that the local host UUID has
   the App system role.
1. The Instance cert is then used to call GenerateHostCerts to request a new
   Instance cert with the Instance, Node and App system roles. This is allowed
   because the cert already has the Instance and SSH roles, and a
   SystemRoleAssertion exists for the (host UUID, App system role) pair.

The issue is that at step 1 the Auth service trusts whatever host UUID the
agent requests for the new App cert when it "joins" with a token, and in the
new paradigm the token join method should only be used to request Instance
certs with a new host UUID assigned by Auth.

Allowing existing instances to extend their system role set by re-joining with
a valid join token is relied upon by multiple users and breaking it would be a
major breaking change.
The proposed solution is a new flow where the existing Instance cert is used to
make an authenticated request to the Join endpoint, and it receives an Instance
cert with its existing host UUID, all of its current system roles AND the
system roles allowed by the new join token.
This instance cert can then be used to call GenerateHostCerts and get a cert of
the App service with only the App system role.

Using the same example where an agent initially started with only the SSH
service restarts with the App service enabled:

1. The agent starts up and detects the app service is enabled but there is no
   App system role in the existing Instance cert.
1. The agent uses the existing Instance cert to call the Join RPC with its
   configured provision token.
1. The Auth service allows the join request to succeed with the existing host
   UUID because it is authenticated request, and grants all system roles
   authorized by the authenticated Instance identity AND the system roles allowed
   by the join token.
1. Host certs with the Instance, Node, and App roles are returned.
1. The Instance cert can be used to call GenerateHostCerts to get App service certs.

Once this new flow is in place, we should be able to deprecate and remove the
AssertSystemRole RPC.

### Security

This goal of this feature is purely to increase security by preventing joining
nodes from choosing their own UUID.

Special consideration should be given to the security model of the new flow for
extending Instance certs with additional system roles based on a new
authenticated token join flow.
This flow allows agents to complete the cluster joining process with an
existing host UUID and with system roles that the join token does not allow.
The existing host UUID is allowed to be used because the request is
authenticated with the existing Instance cert that encodes the host UUID and
the additional roles.

### Privacy

No new privacy-relevant data is collected or stored in this design.

### Proto Specification

The new JoinService will implement all existing join methods and should be
extended to implement all new join methods we add in the future.

```proto3
syntax = "proto3";

package teleport.join.v1;

import "google/protobuf/timestamp.proto";

option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1;joinv1";

// ClientInit is the first message sent from the client during the join process, it
// holds parameters common to all join methods.
message ClientInit {
  // JoinMethod is the name of the join method that the client is configured to use.
  // This parameter is optional, the client can leave it empty to allow the
  // server to determine the join method based on the provision token named by
  // TokenName, it will be sent to the client in the ServerInit message.
  optional string join_method = 1;
  // TokenName is the name of the join token.
  // This is a secret if using the token join method, otherwise it is a
  // non-secret name of a provision token resource.
  string token_name = 2;
  // SystemRole is the system role requested, e.g. Proxy, Node, Instance, Bot.
  string system_role = 3;
  // PublicTlsKey is the public key requested for the subject of the x509 certificate.
  // It must be encoded in PKIX, ASN.1 DER form.
  bytes public_tls_key = 4;
  // PublicSshKey is the public key requested for the subject of the SSH certificate.
  // It must be encoded in SSH wire format.
  bytes public_ssh_key = 5;
  // ForwardedByProxy will be set to true when the message is forwarded by the
  // Proxy service. When this is set the Auth service must ignore any
  // any credentials authenticating the request, except for the purpose of
  // accepting ProxySuppliedParams.
  bool forwarded_by_proxy = 6;

  // HostParams holds parameters that are specific to host joining and
  // irrelevant to bot joining.
  message HostParams {
    // HostName is the user-friendly node name for the host. This comes from
    // teleport.nodename in the service configuration and defaults to the
    // hostname. It is encoded as a valid principal in issued certificates.
    string host_name = 1;
    // AdditionalPrincipals is a list of additional principals requested.
    repeated string additional_principals = 2;
    // DnsNames is a list of DNS names requested for inclusion in the x509 certificate.
    repeated string dns_names = 3;
  }
  optional HostParams host_params = 7;

  // BotParams holds parameters that are specific to bot joining and irrelevant
  // to host joining.
  message BotParams {
    // Expires is a desired time of the expiry of the returned certificates.
    optional google.protobuf.Timestamp expires = 9;
  }
  optional BotParams bot_params = 8;

  // ProxySuppliedParams holds parameters set by the Proxy when nodes join
  // via the proxy address. They must only be trusted if the incoming join
  // request is authenticated as the Proxy.
  message ProxySuppliedParams {
    // RemoteAddr is the remote address of the host requesting a host certificate.
    // It replaces 0.0.0.0 in the list of additional principals.
    string remote_addr = 1;
    // ClientVersion is the Teleport version of the client attempting to join.
    string client_version = 2;
  }
  optional ProxySuppliedParams proxy_supplied_parameters = 9;
}

// EC2IdentityDocument is sent from the client in response to the
// ServerInit message for the EC2 join method.
//
// The EC2 method join flow is:
// 1. client->server: ClientInit
// 2. server->client: ServerInit
// 3. client->server: EC2IdentityDocument
// 4. server->client: Result
message EC2IdentityDocument {
  // Document is a signed EC2 Instance Identity Document used to prove the
  // identity of a joining EC2 instance.
  bytes document = 1;
}

// IAMChallenge is sent from the server immediately after the ServerInit
// message for the IAM join method.
// The client is expected to respond with a IAMChallengeSolution.
//
// The IAM method join flow is:
// 1. client->server: ClientInit
// 2. server->client: ServerInit
// 3. server->client: IAMChallenge
// 4. client->server: IAMChallengeSolution
// 5. server->client: Result
message IAMChallenge {
  // Challenge is a a crypto-random string that should be included by the
  // client in the IAMChallengeSolution message.
  string challenge = 1;
}

// IAMChallengeSolution must be sent from the client in response to the
// IAMChallenge message.
message IAMChallengeSolution {
  // sts:GetCallerIdentity API endpoint used to prove the AWS identity of a
  // joining node. It must include the challenge string as a signed header.
  bytes sts_identity_request = 1;
}

// AzureChallenge is sent from the server immediately after the ServerInit
// message for the Azure join method.
// The client is expected to respond with a AzureChallengeSolution.
//
// The Azure method join flow is:
// 1. client->server: ClientInit
// 2. server->client: ServerInit
// 3. server->client: AzureChallenge
// 4. client->server: AzureChallengeSolution
// 5. server->client: Result
message AzureChallenge {
  // Challenge is a a crypto-random string that should be included by the
  // client in the challenge response message.
  string challenge = 1;
}

// AzureChallengeSolution must be sent from the client in response to the
// AzureChallenge message.
message AzureChallengeSolution {
  // AttestedData is a signed JSON document from an Azure VM's attested data
  // metadata endpoint used to prove the identity of a joining node. It must
  // include the challenge string as the nonce.
  bytes attested_data = 1;
  // AccessToken is a JWT signed by Azure, used to prove the identity of a
  // joining node.
  string access_token = 2;
}

// OracleChallenge is the message type sent from the cluster in response to the
// ClientInit message when the provision token specifies the Oracle join method.
// The client is expected to respond with an OracleChallengeSolution.
//
// The Oracle method join flow is:
// 1. client->server: ClientInit
// 2. server->client: ServerInit
// 3. server->client: OracleChallenge
// 4. client->server: OracleChallengeSolution
// 5. server->client: Result
message OracleChallenge {
  // Challenge is a crypto-random string that should be included in the signed
  // headers.
  string challenge = 1;
}

// OracleChallengeSolution must be sent from the client in response to the
// OracleChallenge message.
message OracleChallengeSolution {
  // Headers is the signed headers for a request to the Oracle authorizeClient
  // endpoint.
  map<string, string> headers = 1;
  // PayloadHeaders is the signed headers that are the payload to the authorizeClient
  // request signified by Headers.
  map<string, string> payload_headers = 2;
}

// OIDCToken holds the OIDC identity token used for all OIDC-based join methods.
//
// The join flow for all OIDC-based join methods is:
// 1. client->server: ClientInit
// 2. server->client: ServerInit
// 3. client->server: OIDCToken
// 4. server->client: Result
message OIDCToken {
  // IdToken is the OIDC identity token.
  bytes id_token = 1;
}

// TPMAttestationParameters is the message sent from the client in response to
// the ServerInit message for the TPM join flow.
// The server is expected to respond with a TPMActiveCredential message.
//
// The TPM method join flow is:
// 1. client->server: ClientInit
// 2. server->client: ServerInit
// 3. client->server: TPMAttestationParameters
// 4. server->client: TPMEncryptedCredential
// 5. client->server: TPMSolution
// 6. server->client: Result
message TPMAttestationParameters {
  // The encoded TPMT_PUBLIC structure containing the attestation public key
  // and signing parameters.
  bytes public = 1;
  // The properties of the attestation key, encoded as a TPMS_CREATION_DATA
  // structure.
  bytes create_data = 2;
  // An assertion as to the details of the key, encoded as a TPMS_ATTEST
  // structure.
  bytes create_attestation = 3;
  // A signature of create_attestation, encoded as a TPMT_SIGNATURE structure.
  bytes create_signature = 4;
  oneof ek {
    // The device's endorsement certificate in X509, ASN.1 DER form. This
    // certificate contains the public key of the endorsement key. This is
    // preferred to ek_key.
    bytes ek_cert = 5;
    // The device's public endorsement key in PKIX, ASN.1 DER form. This is
    // used when a TPM does not contain any endorsement certificates.
    bytes ek_key = 6;
  }
}

// TPMEncryptedCredential is the message sent from the server in response to the
// TPMAttestationParameters message.
// The client is expected to respond with a TPMSolution message.
message TPMEncryptedCredential {
  // The `credential_blob` parameter to be used with the `ActivateCredential`
  // command. This is used with the decrypted value of `secret` in a
  // cryptographic process to decrypt the solution.
  bytes credential_blob = 1;
  // The `secret` parameter to be used with `ActivateCredential`. This is a
  // seed which can be decrypted with the EK. The decrypted seed is then used
  // when decrypting `credential_blob`.
  bytes secret = 2;
}

// TPMSolution is the message sent from the client in response to the
// TPMEncryptedCredential message. The server is expected to respond with a
// Result message.
message TPMSolution {
  // The client's solution to TPMEncryptedCredential using ActivateCredential.
  bytes solution = 1;
}

// BoundKeypairInit is sent from the client in response to the ServerInit
// message for the bound keypair join method.
// The server is expected to respond with a BoundKeypairChallenge.
//
// The bound keypair method join flow is:
// 1. client->server: ClientInit
// 2. server->client: ServerInit
// 3. client->server: BoundKeypairInit
// 4. server->client: BoundKeypairChallenge
// 5. client->server: BoundKeypairChallengeSolution
//   (optional additional steps if keypair rotation is required)
//   server->client: BoundKeypairRotationRequest
//   client->server: BoundKeypairRotationResponse
//   server->client: BoundKeypairChallenge
//   client->server: BoundKeypairChallengeSolution
// 6. server->client: Result containing BoundKeypairResult
message BoundKeypairInit {
  // If set, attempts to bind a new keypair using an initial join secret.
  // Any value set here will be ignored if a keypair is already bound.
  string initial_join_secret = 1;
  // A document signed by Auth containing join state parameters from the
  // previous join attempt. Not required on initial join; required on all
  // subsequent joins.
  bytes previous_join_state = 2;
}

// BoundKeypairChallenge is a challenge issued by the server that joining
// clients are expected to complete.
// The client is expected to respond with a BoundKeypairChallengeSolution.
message BoundKeypairChallenge {
  // The desired public key corresponding to the private key that should be used
  // to sign this challenge, in SSH authorized keys format.
  bytes public_key = 1;
  // A challenge to sign with the requested public key. During keypair rotation,
  // a second challenge will be provided to verify the new keypair before certs
  // are returned.
  string challenge = 2;
}

// BoundKeypairChallengeSolution is sent from the client in response to the
// BoundKeypairChallenge.
// The server is expected to respond with either a Result or a
// BoundKeypairRotationRequest.
message BoundKeypairChallengeSolution {
  // A solution to a challenge from the server. This generated by signing the
  // challenge as a JWT using the keypair associated with the requested public
  // key.
  bytes solution = 1;
}

// BoundKeypairRotationRequest is sent by the server in response to a
// BoundKeypairChallenge when a keypair rotation is required. It acts like an
// additional challenge, the client is expected to respond with a
// BoundKeypairRotationResponse.
message BoundKeypairRotationRequest {
  // The signature algorithm suite in use by the cluster.
  string signature_algorithm_suite = 1;
}

// BoundKeypairRotationResponse is sent by the client in response to a
// BoundKeypairRotationRequest from the server.
// The server is expected to respond with an additional BoundKeypairChallenge
// for the new key.
message BoundKeypairRotationResponse {
  // The public key to be registered with auth. Clients should expect a
  // subsequent challenge against this public key to be sent. This is encoded in
  // SSH authorized keys format.
  bytes public_key = 1;
}

// BoundKeypairResult holds additional result parameters relevant to the bound
// keypair join method.
message BoundKeypairResult {
  // A signed join state document to be provided on the next join attempt.
  bytes join_state = 2;
  // The public key registered with Auth at the end of the joining ceremony.
  // After a successful keypair rotation, this should reflect the newly
  // registered public key. This is encoded in SSH authorized keys format.
  bytes public_key = 3;
}

// ChallengeSolution holds a solution to a challenge issued by the server.
message ChallengeSolution {
  oneof payload {
    IAMChallengeSolution iam_challenge_solution = 1;
    AzureChallengeSolution azure_challenge_solution = 2;
    OracleChallengeSolution oracle_challenge_solution = 3;
    TPMSolution tpm_solution = 4;
    BoundKeypairChallengeSolution bound_keypair_challenge_solution = 5;
    BoundKeypairRotationResponse bound_keypair_rotation_response = 6;
  }
}

// JoinRequest is the message type sent from the joining client to the server.
message JoinRequest {
  oneof payload {
    ClientInit client_init = 1;
    ChallengeSolution solution = 2;
    EC2IdentityDocument ec2_identity_document = 3;
    OIDCToken oidc_token = 4;
    TPMAttestationParameters tpm_attestation_parameters = 5;
    BoundKeypairInit bound_keypair_init = 6;
  }
}

// ServerInit is the first message sent from the server in response to the
// ClientInit message. It contains the join method name, and it may include a
// challenge if the join method requires the server to issue a challenge that
// does not depend on information from the client.
message ServerInit {
  // JoinMethod is the name of the selected join method.
  string join_method = 1;
}

// Challenge is a challenge message sent from the server that the client must solve.
message Challenge {
  oneof payload {
    IAMChallenge iam_challenge = 1;
    AzureChallenge azure_challenge = 2;
    OracleChallenge oracle_challenge = 3;
    TPMEncryptedCredential tpm_encrypted_credential = 4;
    BoundKeypairChallenge bound_keypair_challenge = 5;
    BoundKeypairRotationRequest bound_keypair_rotation_request = 6;
  }
}

// Result is the final message sent from the cluster back to the client, it
// contains the result of the joining process including the assigned host ID
// and issued certificates.
message Result {
  // TlsCert is an X.509 certificate encoded in ASN.1 DER form.
  bytes tls_cert = 1;
  // TlsCaCerts is a list of TLS certificate authorities that the agent should trust.
  // Each certificate is encoding in ASN.1 DER form.
  repeated bytes tls_ca_certs = 2;
  // SshCert is an SSH certificate encoded in SSH wire format.
  bytes ssh_cert = 3;
  // SshCaKey is a list of SSH certificate authority public keys that the agent should trust.
  // Each CA key is encoded in SSH wire format.
  repeated bytes ssh_ca_keys = 4;
  // HostId is the unique ID assigned to the host. Unset for bot joining.
  optional string host_id = 5;
  // BoundKeypairResult holds extra result parameters relevant to the bound keypair join method.
  optional BoundKeypairResult bound_keypair_result = 6;
}

// JoinResponse is the message type sent from the server to the joining client.
message JoinResponse {
  oneof payload {
    // Init is the initial message sent from the server in response to the
    // ClientInit message. It specifies the join method used by the provision token.
    ServerInit init = 1;

    // Challenge is a challenge issued by the server that the client must solve
    // in order to complete the join flow. The challenge type depends on the join method.
    // Each method may issue zero or more challenges that the client must solve.
    Challenge challenge = 2;

    // Result is the result of the join flow, it is the final message sent from
    // the cluster when the join flow is successful.
    // For the token join method, it is sent immediately in response to the ClientInit request.
    Result result = 3;
  }
}

// JoinService provides methods which allow Teleport nodes, proxies, and other
// services to "join" the Teleport cluster by completing a supported join flow
// in order to receive signed certificates issued by the cluster.
//
// It may be used in multiple cases:
// * Teleport agents joining the cluster on their first start to receive their
//   initial certificates. These requests do not use mTLS and the client
//   authenticates itself using only the join flow and is assigned a new host
//   ID.
// * Teleport agents that need certificates authenticated for an additional
//   system role allowed by a new provision token. These requests must be
//   authenticated with mTLS using their existing certificates so that the
//   existing host ID can be maintained.
// * MachineID bots fetching their initial certificates.
// * MachineID bots refreshing their certificates.
//
// It is implemented on both the Auth and Proxy servers to serve the needs of
// * clients connecting to the proxy address for their initial join when they are
//   unauthenticated and unable to directly dial the auth service.
// * clients connecting to the auth address for their initial join.
// * clients refreshing existing certificates that are able to make an
//   authenticates dial to the auth service via proxy TLS routing.
service JoinService {
  // Join is a bidirectional streaming RPC that implements all join methods.
  // The client does not need to know the join method ahead of time, all it
  // needs is the token name.
  //
  // The client must send an ClientInit message on the JoinRequest stream to
  // initiate the join flow.
  //
  // The server will reply with a JoinResponse where the payload will vary
  // based on the join method specified in the provision token.
  rpc Join(stream JoinRequest) returns (stream JoinResponse);
}
```

The `AssertSystemRole` RPC in the legacy Auth gRPC service will be deprecated.
`RegisterUsingTokenRequest` will be deprecated.

### Backward Compatibility

Backward compatibility is discussed in the UX section.
Older agents will continue to be allowed to use the legacy join RPCs and select
their own host UUID until support for auth-assigned host UUIDs has been
released for 2 major versions or an environment variable is set on the Auth
service.
The existing AssertSystemRole RPC will continue to be supported for 2 major versions.

### Audit Events

A new audit event will be emitted when system roles assertions are created.

### Test Plan

We should test that additional services can be added to an existing agent with
a valid join token for the additional service.
