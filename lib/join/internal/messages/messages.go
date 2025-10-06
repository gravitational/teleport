// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package messages

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
)

// Request is implemented by all join request messages.
type Request interface {
	isRequest()
}

// embedRequest is embedded in all join request message as a shorthand for
// implementing the [Request] interface on pointers to the message type.
type embedRequest struct{}

func (*embedRequest) isRequest() {}

// ClientInit is the first messages sent from the client during the join
// process, it contains parameters common to all join methods.
type ClientInit struct {
	embedRequest

	// JoinMethod is the name of the join method that the client is configured to use.
	// This parameter is optional, the client can leave it empty to allow the
	// server to determine the join method based on the provision token named by
	// TokenName, it will be sent to the client in the ServerInit message.
	JoinMethod *string
	// TokenName is the name of the join token.
	// This is a secret if using the token join method, otherwise it is a
	// non-secret name of a provision token resource.
	TokenName string
	// SystemRole is the system role requested, e.g. Proxy, Node, Instance, Bot.
	SystemRole string
	// ForwardedByProxy will be set to true when the message is forwarded by the
	// Proxy service. When this is set the Auth service must ignore any
	// any credentials authenticating the request, except for the purpose of
	// accepting ProxySuppliedParameters.
	ForwardedByProxy bool
	// ProxySuppliedParams holds parameters added by the Proxy when
	// nodes join via the proxy address. They must only be trusted if the
	// incoming join request is authenticated as the Proxy.
	ProxySuppliedParams *ProxySuppliedParams
}

func (i *ClientInit) Check() error {
	switch {
	case i.TokenName == "":
		return trace.BadParameter("TokenName is required")
	case i.SystemRole == "":
		return trace.BadParameter("SystemRole is required")
	}
	return nil
}

// ProxySuppliedParams contains parameters added by the proxy when the
// proxy terminates the initial TLS connection. These should only be trusted
// when the request authenticates as a valid proxy.
type ProxySuppliedParams struct {
	// RemoteAddr is the remote address of the host requesting a certificate.
	// It replaces 0.0.0.0 in the list of additional principals.
	RemoteAddr string
	// ClientVersion is the Teleport version of the client attempting to join.
	ClientVersion string
}

// TokenInit is sent by the client in response to the ServerInit message for
// the Token join method.
//
// The Token method join flow is:
// 1. client->server: ClientInit
// 2. server->client: ServerInit
// 3. client->server: TokenInit
// 4. server->client: Result
type TokenInit struct {
	embedRequest

	// ClientParams holds parameters for the specific type of client trying to join.
	ClientParams ClientParams
}

func (i *TokenInit) Check() error {
	return trace.Wrap(i.ClientParams.check(), "checking ClientParams")
}

// ClientParams holds either host or bot join parameters.
type ClientParams struct {
	HostParams *HostParams
	BotParams  *BotParams
}

func (p *ClientParams) check() error {
	switch {
	case p.HostParams == nil && p.BotParams == nil:
		return trace.BadParameter("HostParams or BotParams must be set")
	case p.HostParams != nil && p.BotParams != nil:
		return trace.BadParameter("HostParams and BotParams cannot both be set")
	}
	if err := p.HostParams.check(); err != nil {
		return trace.Wrap(err, "checking HostParams")
	}
	if err := p.BotParams.check(); err != nil {
		return trace.Wrap(err, "checking BotParams")
	}
	return nil
}

// HostParams holds parameters that are specific to host joining and
// irrelevant to bot joining.
type HostParams struct {
	// PublicKeys holds the host public keys.
	PublicKeys PublicKeys
	// HostName is the user-friendly node name for the host. This comes from
	// teleport.nodename in the service configuration and defaults to the
	// hostname. It is encoded as a valid principal in issued certificates.
	HostName string
	// AdditionalPrincipals is a list of additional principals requested.
	AdditionalPrincipals []string
	// DnsNames is a list of DNS names requested for inclusion in the x509 certificate.
	DNSNames []string
}

func (p *HostParams) check() error {
	switch {
	case p == nil:
		return nil
	case p.HostName == "":
		return trace.BadParameter("HostName is required")
	}
	return trace.Wrap(p.PublicKeys.check())
}

// BotParams holds parameters that are specific to bot joining and
// irrelevant to host joining.
type BotParams struct {
	// PublicKeys holds the bot public keys.
	PublicKeys PublicKeys
	// Expires is a desired time of the expiry of certificates returned by
	// registration.
	Expires *time.Time
}

func (p *BotParams) check() error {
	switch {
	case p == nil:
		return nil
	case p.Expires.IsZero():
		return trace.BadParameter("Expires is required")
	}
	return trace.Wrap(p.PublicKeys.check())
}

// PublicKeys holds public keys sent by the client requested subject keys for
// issued certificates.
type PublicKeys struct {
	// PublicTlsKey is the public key requested for the subject of the x509 certificate.
	// It must be encoded in PKIX, ASN.1 DER form.
	PublicTLSKey []byte
	// PublicSshKey is the public key requested for the subject of the SSH certificate.
	// It must be encoded in SSH wire format.
	PublicSSHKey []byte
}

func (k *PublicKeys) check() error {
	switch {
	case len(k.PublicTLSKey) == 0:
		return trace.BadParameter("PublicTLSKey is required")
	case len(k.PublicSSHKey) == 0:
		return trace.BadParameter("PublicSSHKey is required")
	}
	return nil
}

// BoundKeypairInit is sent from the client in response to the ServerInit
// message for the bound keypair join method.
// The server is expected to respond with a BoundKeypairChallenge.
//
// The bound keypair method join flow is:
//  1. client->server: ClientInit
//  2. server->client: ServerInit
//  3. client->server: BoundKeypairInit
//  4. server->client: BoundKeypairChallenge
//  5. client->server: BoundKeypairChallengeSolution
//     (optional additional steps if keypair rotation is required)
//     server->client: BoundKeypairRotationRequest
//     client->server: BoundKeypairRotationResponse
//     server->client: BoundKeypairChallenge
//     client->server: BoundKeypairChallengeSolution
//  6. server->client: Result containing BoundKeypairResult
type BoundKeypairInit struct {
	embedRequest

	// ClientParams holds parameters for the specific type of client trying to join.
	ClientParams ClientParams
	// If set, attempts to bind a new keypair using an initial join secret. Any
	// value set here will be ignored if a keypair is already bound.
	InitialJoinSecret string
	// A document signed by Auth containing join state parameters from the
	// previous join attempt. Not required on initial join; required on all
	// subsequent joins.
	PreviousJoinState []byte
}

// BoundKeypairChallenge is a challenge issued by the server that joining
// clients are expected to complete.
// The client is expected to respond with a BoundKeypairChallengeSolution.
type BoundKeypairChallenge struct {
	embedResponse

	// The desired public key corresponding to the private key that should be
	// used to sign this challenge, in SSH authorized keys format.
	PublicKey []byte
	// A challenge to sign with the requested public key. During keypair
	// rotation, a second challenge will be provided to verify the new keypair
	// before certs are returned.
	Challenge string
}

// BoundKeypairChallengeSolution is sent from the client in response to the
// BoundKeypairChallenge.
// The server is expected to respond with either a Result or a
// BoundKeypairRotationRequest.
type BoundKeypairChallengeSolution struct {
	embedRequest

	// A solution to a challenge from the server. This generated by signing the
	// challenge as a JWT using the keypair associated with the requested public
	// key.
	Solution []byte
}

// BoundKeypairRotationRequest is sent by the server in response to a
// BoundKeypairChallenge when a keypair rotation is required. It acts like an
// additional challenge, the client is expected to respond with a
// BoundKeypairRotationResponse.
type BoundKeypairRotationRequest struct {
	embedResponse

	// The signature algorithm suite in use by the cluster.
	SignatureAlgorithmSuite string
}

// BoundKeypairRotationResponse is sent by the client in response to a
// BoundKeypairRotationRequest from the server. The server is expected to
// respond with an additional BoundKeypairChallenge for the new key.
type BoundKeypairRotationResponse struct {
	embedRequest

	// The public key to be registered with auth. Clients should expect a
	// subsequent challenge against this public key to be sent. This is encoded
	// in SSH authorized keys format.
	PublicKey []byte
}

// BoundKeypairResult holds additional result parameters relevant to the bound
// keypair join method.
type BoundKeypairResult struct {
	// A signed join state document to be provided on the next join attempt.
	JoinState []byte
	// The public key registered with Auth at the end of the joining ceremony.
	// After a successful keypair rotation, this should reflect the newly
	// registered public key. This is encoded in SSH authorized keys format.
	PublicKey []byte
}

// Response is implemented by all join response messages.
type Response interface {
	isResponse()
}

// embedResponse is embedded in all join response messages as a shorthand for
// implementing the [Response] interface on pointers to the message type.
type embedResponse struct{}

func (*embedResponse) isResponse() {}

// ServerInit is the first message sent from the server in response to the
// ClientInit message.
type ServerInit struct {
	embedResponse

	// JoinMethod is the name of the selected join method.
	JoinMethod string
	// SignatureAlgorithmSuite is the name of the signature algorithm suite
	// currently configured for the cluster.
	SignatureAlgorithmSuite types.SignatureAlgorithmSuite
}

// HostResult holds results for host joining.
type HostResult struct {
	embedResponse

	// Certificates holds issued certificates and cluster CAs.
	Certificates Certificates
	// HostId is the unique ID assigned to the host.
	HostID string
}

// BotResult holds results for bot joining.
type BotResult struct {
	embedResponse

	// Certificates holds issued certificates and cluster CAs.
	Certificates Certificates
	// BoundKeypairResult holds extra result parameters relevant to the bound keypair join method.
	BoundKeypairResult *BoundKeypairResult
}

// Certificates holds issued certificates and cluster CAs.
type Certificates struct {
	// TLSCert is an X.509 certificate encoded in ASN.1 DER form.
	TLSCert []byte
	// TLSCACerts is a list of TLS certificate authorities that the client should trust.
	// Each certificate is encoding in ASN.1 DER form.
	TLSCACerts [][]byte
	// SSHCert is an SSH certificate encoded in SSH wire format.
	SSHCert []byte
	// SSHCAKeys is a list of SSH certificate authority public keys that the client should trust.
	// Each CA key is encoded in SSH wire format.
	SSHCAKeys [][]byte
}

// ClientStream represents the client side of a join request stream.
// It can send [Request]s and receive [Response]s.
//
// To cancel a blocked Send or Recv, cancel the parent context used to create
// the stream. This models the underlying gRPC stream.
type ClientStream interface {
	Send(Request) error
	Recv() (Response, error)
	CloseSend() error
}

// ServerStream represents the server side of a join request stream.
// It can send [Response]s and receive [Request]s.
//
// To cancel a blocked Send or Recv you must return from the handler. This
// models the underlying gRPC stream.
type ServerStream interface {
	Context() context.Context
	Diagnostic() *diagnostic.Diagnostic
	Send(Response) error
	Recv() (Request, error)
}

// RecvRequest calls [ServerStream.Recv] and asserts the expected type of
// the received message, returning an appropriate error if the client sent a
// message with an unexpected type.
func RecvRequest[T Request](ss ServerStream) (T, error) {
	req, err := ss.Recv()
	if err != nil {
		var nul T
		return nul, trace.Wrap(err)
	}
	return AssertRequestType[T](req)
}

// AssertRequestType performs a type assertion on a request and returns an
// appropriate error if the request has an unexpected type.
func AssertRequestType[T Request](req Request) (T, error) {
	// TODO(nklaassen): add ClientGivingUp request type and return an
	// appropriate error here.
	switch typedRequest := req.(type) {
	case T:
		return typedRequest, nil
	default:
		var nul T
		return nul, trace.BadParameter("expected client to send message of type %T, got %T", nul, req)
	}
}

// RecvResponse calls [ClientStream.Recv] and asserts the expected type of
// the received message, returning an appropriate error if the server sent a
// message with an unexpected type.
func RecvResponse[T Response](cs ClientStream) (T, error) {
	var nul T
	resp, err := cs.Recv()
	if err != nil {
		return nul, trace.Wrap(err)
	}
	typedResp, ok := resp.(T)
	if !ok {
		return nul, trace.BadParameter("expected server to send message of type %T, got %T", nul, resp)
	}
	return typedResp, nil
}
