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
	// PublicTlsKey is the public key requested for the subject of the x509 certificate.
	// It must be encoded in PKIX, ASN.1 DER form.
	PublicTLSKey []byte
	// PublicSshKey is the public key requested for the subject of the SSH certificate.
	// It must be encoded in SSH wire format.
	PublicSSHKey []byte
	// ForwardedByProxy will be set to true when the message is forwarded by the
	// Proxy service. When this is set the Auth service must ignore any
	// any credentials authenticating the request, except for the purpose of
	// accepting ProxySuppliedParameters.
	ForwardedByProxy bool
	// HostParams holds parameters that are specific to host joining and
	// irrelevant to bot joining.
	HostParams *HostParams
	// BotParams holds parameters that are specific to bot joining and
	// irrelevant to host joining.
	BotParams *BotParams
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
	case len(i.PublicTLSKey) == 0:
		return trace.BadParameter("PublicTLSKey is required")
	case len(i.PublicSSHKey) == 0:
		return trace.BadParameter("PublicSSHKey is required")
	case i.HostParams == nil && i.BotParams == nil:
		return trace.BadParameter("HostParams or BotParams must be set")
	case i.HostParams != nil && i.BotParams != nil:
		return trace.BadParameter("HostParams and BotParams cannot both be set")
	}
	if err := i.HostParams.check(); err != nil {
		return trace.Wrap(err, "checking HostParams")
	}
	if err := i.BotParams.check(); err != nil {
		return trace.Wrap(err, "checking BotParams")
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

// HostParams holds parameters that are specific to host joining and
// irrelevant to bot joining.
type HostParams struct {
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
	return nil
}

// BotParams holds parameters that are specific to bot joining and
// irrelevant to host joining.
type BotParams struct {
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
	return nil
}

// Response is implemented by all join response messages.
type Response interface {
	isResponse()
}

// embedResponse is embedded in all join response messages as a shorthand for
// implementing the [Response] interface on pointers to the message type.
type embedResponse struct{}

func (*embedResponse) isResponse() {}

// Result is the final message sent from the cluster back to the client, it
// contains the result of the joining process including the assigned host ID
// and issued certificates.
type Result struct {
	embedResponse

	// TlsCert is an X.509 certificate encoded in ASN.1 DER form.
	TLSCert []byte
	// TlsCaCerts is a list of TLS certificate authorities that the agent should trust.
	// Each certificate is encoding in ASN.1 DER form.
	TLSCACerts [][]byte
	// SshCert is an SSH certificate encoded in SSH wire format.
	SSHCert []byte
	// SshCaKey is a list of SSH certificate authority public keys that the agent should trust.
	// Each CA key is encoded in SSH wire format.
	SSHCAKeys [][]byte
	// HostId is the unique ID assigned to the host.
	HostID *string
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
