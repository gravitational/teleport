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

package join

import (
	"cmp"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	joinauthz "github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/utils/hostid"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "join")

// AuthService is the subset of the Auth service interface required by the
// JoinServer to implement joining.
type AuthService interface {
	ValidateToken(ctx context.Context, tokenName string) (types.ProvisionToken, error)
	GenerateHostCertsForJoin(ctx context.Context, provisionToken types.ProvisionToken, req *HostCertsParams) (*proto.Certs, error)
	GenerateBotCertsForJoin(ctx context.Context, provisionToken types.ProvisionToken, req *BotCertsParams) (*proto.Certs, string, error)
	EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error
}

// ServerConfig holds configuration parameters for [Server].
type ServerConfig struct {
	AuthService AuthService
	Authorizer  authz.Authorizer
	Clock       clockwork.Clock
}

// Server implements cluster joining for nodes and bots.
type Server struct {
	cfg   *ServerConfig
	clock clockwork.Clock
}

// NewServer returns a new [Server] instance.
func NewServer(cfg *ServerConfig) *Server {
	return &Server{
		cfg:   cfg,
		clock: cmp.Or(cfg.Clock, clockwork.NewRealClock()),
	}
}

// Join implements cluster joining for nodes and bots.
//
// It returns credentials for a node or bot to join the Teleport cluster using
// a provision token.
//
// The client must request a specific role (and the role must match one of the
// roles the token was generated for).
//
// If a token was generated with a TTL, it gets enforced (can't join after the
// token expires).
//
// Only secret tokens are currently supported.
// TODO(nklaassen): support all join methods.
func (s *Server) Join(stream messages.ServerStream) (err error) {
	ctx := stream.Context()
	diag := stream.Diagnostic()
	defer func() {
		if err != nil {
			s.handleJoinFailure(ctx, diag, err)
		}
	}()

	// Receive the first message from the client, which must always be ClientInit.
	clientInit, err := messages.RecvRequest[*messages.ClientInit](stream)
	if err != nil {
		return trace.Wrap(err)
	}
	// Set any diagnostic info we can get from the ClientInit message.
	diag.Set(func(i *diagnostic.Info) {
		i.Role = clientInit.SystemRole
		if clientInit.JoinMethod != nil {
			i.RequestedJoinMethod = *clientInit.JoinMethod
		}
		if clientInit.HostParams != nil {
			i.NodeName = clientInit.HostParams.HostName
		}
	})
	if err := clientInit.Check(); err != nil {
		return trace.Wrap(err, "validating ClientInit message")
	}

	// Authenticate the request in case the node/bot is rejoining with previous
	// credentials.
	authCtx, err := s.authenticate(ctx, clientInit)
	if err != nil {
		return trace.Wrap(err)
	}
	// Set any diagnostic info we can get from the authenticated identity.
	diag.Set(func(i *diagnostic.Info) {
		i.HostID = authCtx.HostID
		i.SystemRoles = authCtx.SystemRoles.StringSlice()
		i.BotInstanceID = authCtx.BotInstanceID
		i.BotGeneration = authCtx.BotGeneration
	})

	// Fetch the provision token and validate that it is not expired.
	provisionToken, err := s.cfg.AuthService.ValidateToken(ctx, clientInit.TokenName)
	if err != nil {
		return trace.Wrap(err)
	}
	// Set any diagnostic info we can get from the token.
	diag.Set(func(i *diagnostic.Info) {
		i.SafeTokenName = provisionToken.GetSafeName()
		i.TokenJoinMethod = string(provisionToken.GetJoinMethod())
		i.TokenExpires = provisionToken.Expiry()
		i.BotName = provisionToken.GetBotName()
	})

	// Validate that the requested join method matches the join method
	// configured on the token, or that the client did not specify a specific
	// join method and allow the server to choose it from the token.
	joinMethod, err := checkJoinMethod(provisionToken, clientInit.JoinMethod)
	if err != nil {
		return trace.Wrap(err)
	}

	// Assert that the provision token allows the requested system role.
	if err := ProvisionTokenAllowsRole(provisionToken, types.SystemRole(clientInit.SystemRole)); err != nil {
		return trace.Wrap(err)
	}

	// TODO(nklaassen): implement checks for all join methods.
	switch joinMethod {
	case types.JoinMethodToken:
		// No additional checks necessary for the token join method.
	default:
		return trace.NotImplemented("join method %s is not yet implemented by the new join service", joinMethod)
	}

	var (
		certs  *proto.Certs
		hostID *string
	)
	if types.SystemRole(clientInit.SystemRole) == types.RoleBot {
		params, err := makeBotCertsParams(diag, authCtx, clientInit)
		if err != nil {
			return trace.Wrap(err)
		}
		certs, _, err = s.cfg.AuthService.GenerateBotCertsForJoin(ctx, provisionToken, params)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		params, err := makeHostCertsParams(ctx, diag, authCtx, clientInit, joinMethod)
		if err != nil {
			return trace.Wrap(err)
		}
		hostID = &params.HostID
		certs, err = s.cfg.AuthService.GenerateHostCertsForJoin(ctx, provisionToken, params)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Convert the result from GenerateCertsForJoin to a Result message and
	// send it back to the client.
	result, err := makeResultMessage(certs)
	if err != nil {
		return trace.Wrap(err)
	}
	result.HostID = hostID
	return trace.Wrap(stream.Send(result))
}

func (s *Server) authenticate(ctx context.Context, clientInit *messages.ClientInit) (*joinauthz.Context, error) {
	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil && !trace.IsAccessDenied(err) {
		return nil, trace.Wrap(err, "unexpected error authorizing request")
	}
	if trace.IsAccessDenied(err) || authCtx == nil {
		// No authentication or AccessDenied is okay, this is not normally an
		// authenticated endpoint unless the client is re-joining or the
		// request was forwarded by a proxy, just return an empty Context.
		return &joinauthz.Context{}, nil
	}
	isProxy := authz.HasBuiltinRole(*authCtx, types.RoleProxy.String())
	if !isProxy && clientInit.ProxySuppliedParams != nil {
		return nil, trace.AccessDenied("client set ProxySuppliedParameters but did not authenticate as a proxy")
	}
	if clientInit.ForwardedByProxy {
		if !isProxy {
			return nil, trace.BadParameter("client claims to be a proxy forwarding the request but did not authenticate as a proxy (this is a bug)")
		}
		// Must ignore any authentication if the request was forwarded by a
		// proxy to avoid forgery of a host ID or system role via the proxy
		// credentials.
		return &joinauthz.Context{
			IsForwardedByProxy: true,
		}, nil
	}

	id := authCtx.Identity.GetIdentity()

	isInstance := slices.Equal(id.Groups, []string{types.RoleInstance.String()})
	var systemRoles types.SystemRoles
	if isInstance {
		systemRoles, err = types.NewTeleportRoles(id.SystemRoles)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	hostID := ""
	botInstanceID := ""
	botGeneration := uint64(0)
	if id.IsBot() {
		botInstanceID = id.BotInstanceID
		botGeneration = id.Generation
	} else {
		hostID = strings.SplitN(id.Username, ".", 2)[0]
	}

	return &joinauthz.Context{
		IsInstance:    isInstance,
		IsBot:         id.IsBot(),
		SystemRoles:   systemRoles,
		HostID:        hostID,
		BotInstanceID: botInstanceID,
		BotGeneration: botGeneration,
	}, nil
}

func checkJoinMethod(provisionToken types.ProvisionToken, requestedJoinMethod *string) (types.JoinMethod, error) {
	tokenJoinMethod := provisionToken.GetJoinMethod()
	if requestedJoinMethod == nil {
		// Auto join method mode, the client didn't specify so use whatever is on the token.
		return tokenJoinMethod, nil
	}
	if types.JoinMethod(*requestedJoinMethod) != tokenJoinMethod {
		return "", trace.BadParameter(
			"client requested join method %s, provision token only supports method %s",
			*requestedJoinMethod, tokenJoinMethod)
	}
	return tokenJoinMethod, nil
}

// ProvisionTokenAllowsRole asserts that the given provision token allows the
// requested role, or else it returns an error.
func ProvisionTokenAllowsRole(provisionToken types.ProvisionToken, role types.SystemRole) error {
	// Instance certs can be requested if the provision token allows at least
	// one local service role (e.g. proxy, node, etc).
	if role == types.RoleInstance {
		hasLocalServiceRole := false
		for _, role := range provisionToken.GetRoles() {
			if role.IsLocalService() {
				hasLocalServiceRole = true
				break
			}
		}
		if !hasLocalServiceRole {
			return trace.AccessDenied("cannot requisition instance certs (token contains no local service roles)")
		}
	}

	// Make sure the caller is requesting a role allowed by the token.
	if !provisionToken.GetRoles().Include(role) && role != types.RoleInstance {
		return trace.BadParameter("can not join the cluster, the token does not allow role %s", role)
	}

	return nil
}

// MakeHostCertsParams returns [HostCertsParams] populated by the ClientInit
// message and context of the request.
func makeHostCertsParams(
	ctx context.Context,
	diag *diagnostic.Diagnostic,
	authCtx *joinauthz.Context,
	clientInit *messages.ClientInit,
	joinMethod types.JoinMethod,
) (*HostCertsParams, error) {
	// GenerateHostCertsForJoin requires the TLS key to be PEM-encoded.
	tlsPub, err := x509.ParsePKIXPublicKey(clientInit.PublicTLSKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPubPEM, err := keys.MarshalPublicKey(crypto.PublicKey(tlsPub))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// GenerateHostCertsForJoin requires the SSH key to be in authorized keys format.
	sshPub, err := ssh.ParsePublicKey(clientInit.PublicSSHKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshAuthorizedKey := ssh.MarshalAuthorizedKey(sshPub)

	params := &HostCertsParams{
		SystemRole:   types.SystemRole(clientInit.SystemRole),
		PublicTLSKey: tlsPubPEM,
		PublicSSHKey: sshAuthorizedKey,
	}

	if hostParams := clientInit.HostParams; hostParams != nil {
		params.HostName = hostParams.HostName
		params.AdditionalPrincipals = hostParams.AdditionalPrincipals
		params.DNSNames = hostParams.DNSNames
	}

	if authCtx.IsInstance {
		// Only authenticated Instance certs are allowed to re-join and
		// maintain their existing host ID and authenticate additional system
		// roles.
		params.HostID = authCtx.HostID
		params.AuthenticatedSystemRoles = authCtx.SystemRoles
	} else {
		// Generate a new host ID to assign to the client.
		hostID, err := hostid.Generate(ctx, joinMethod)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		params.HostID = hostID
	}

	// Trust the remote address as forwarded by the proxy, or else use the one
	// we get from the connection context.
	if authCtx.IsForwardedByProxy && clientInit.ProxySuppliedParams != nil {
		params.RemoteAddr = clientInit.ProxySuppliedParams.RemoteAddr
	} else {
		// This gets set on the diagnostic by the gRPC layer.
		params.RemoteAddr = diag.Get().RemoteAddr
	}

	return params, nil
}

// makeBotCertsParams returns [BotCertsParams] populated by the
// ClientInit message and context of the request.
func makeBotCertsParams(
	diag *diagnostic.Diagnostic,
	authCtx *joinauthz.Context,
	clientInit *messages.ClientInit,
) (*BotCertsParams, error) {
	// GenerateBotCertsForJoin requires the TLS key to be PEM-encoded.
	tlsPub, err := x509.ParsePKIXPublicKey(clientInit.PublicTLSKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPubPEM, err := keys.MarshalPublicKey(crypto.PublicKey(tlsPub))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// GenerateBotCertsForJoin requires the SSH key to be in authorized keys format.
	sshPub, err := ssh.ParsePublicKey(clientInit.PublicSSHKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshAuthorizedKey := ssh.MarshalAuthorizedKey(sshPub)

	params := &BotCertsParams{
		PublicTLSKey:  tlsPubPEM,
		PublicSSHKey:  sshAuthorizedKey,
		BotInstanceID: authCtx.BotInstanceID,
	}

	if botParams := clientInit.BotParams; botParams != nil {
		params.BotGeneration = int32(authCtx.BotGeneration)
		params.BotInstanceID = authCtx.BotInstanceID
		params.Expires = botParams.Expires
	}

	// Trust the remote address as forwarded by the proxy, or else use the one
	// we get from the connection context.
	if authCtx.IsForwardedByProxy && clientInit.ProxySuppliedParams != nil {
		params.RemoteAddr = clientInit.ProxySuppliedParams.RemoteAddr
	} else {
		// This gets set on the diagnostic by the gRPC layer.
		params.RemoteAddr = diag.Get().RemoteAddr
	}

	return params, nil
}

// makeResultMessage returns a [*messages.Result] populated from [*proto.Certs]
// with the certs converted into the proper wire format.
func makeResultMessage(certs *proto.Certs) (*messages.Result, error) {
	sshCert, err := rawSSHCert(certs.SSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// certs.SSHCACerts is a misnomer, SSH CAs are just public keys, not certificates.
	sshCAKeys, err := rawSSHPublicKeys(certs.SSHCACerts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.Result{
		TLSCert:    rawTLSCert(certs.TLS),
		TLSCACerts: rawTLSCerts(certs.TLSCACerts),
		SSHCert:    sshCert,
		SSHCAKeys:  sshCAKeys,
	}, nil
}

// rawTLSCerts converts a slice of PEM-encoded TLS certificates to the raw ASN.1
// DER form as required by [Result].
func rawTLSCerts(pemBytes [][]byte) [][]byte {
	out := make([][]byte, len(pemBytes))
	for i, bytes := range pemBytes {
		out[i] = rawTLSCert(bytes)
	}
	return out
}

// rawTLSCert converts a PEM-encoded TLS certificate to the raw ASN.1 DER form
// as required by [Result].
func rawTLSCert(pemBytes []byte) []byte {
	pemBlock, _ := pem.Decode(pemBytes)
	return pemBlock.Bytes
}

// rawSSHCert converts an SSH certificate or public key in SSH authorized_keys
// format to the SSH wire format as required by [messages.Result].
func rawSSHCert(authorizedKey []byte) ([]byte, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(authorizedKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pub.Marshal(), nil
}

// rawSSHPublicKeys converts a slices of SSH public keys in SSH authorized_keys
// format to the SSH wire format as required by [messages.Result].
func rawSSHPublicKeys(authorizedKeys [][]byte) ([][]byte, error) {
	out := make([][]byte, len(authorizedKeys))
	for i, authorizedKey := range authorizedKeys {
		var err error
		out[i], err = rawSSHCert(authorizedKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return out, nil
}

func (s *Server) handleJoinFailure(ctx context.Context, diag *diagnostic.Diagnostic, err error) {
	diag.Set(func(i *diagnostic.Info) { i.Error = err })
	log.LogAttrs(ctx, slog.LevelWarn, "Failure to join cluster occurred", diag.SlogAttrs()...)
	if err := s.cfg.AuthService.EmitAuditEvent(context.WithoutCancel(ctx), makeAuditEvent(diag)); err != nil {
		log.WarnContext(ctx, "Failed to emit failed join event", "error", err)
	}
}

func makeAuditEvent(d *diagnostic.Diagnostic) apievents.AuditEvent {
	info := d.Get()
	errorMessage := info.Error.Error()
	if errors.Is(info.Error, context.Canceled) || status.Code(info.Error) == codes.Canceled {
		errorMessage = "join attempt timed out or was aborted"
	}
	status := apievents.Status{
		Success: false,
		Error:   errorMessage,
	}
	if info.Role == types.RoleBot.String() {
		return &apievents.BotJoin{
			Metadata: apievents.Metadata{
				Type: events.BotJoinEvent,
				Code: events.BotJoinFailureCode,
				Time: time.Now(),
			},
			Status: status,
			ConnectionMetadata: apievents.ConnectionMetadata{
				RemoteAddr: info.RemoteAddr,
			},
			Method:        cmp.Or(info.TokenJoinMethod, info.RequestedJoinMethod),
			TokenName:     info.SafeTokenName,
			BotName:       info.BotName,
			BotInstanceID: info.BotInstanceID,
		}
	}
	return &apievents.InstanceJoin{
		Metadata: apievents.Metadata{
			Type: events.InstanceJoinEvent,
			Code: events.InstanceJoinFailureCode,
			Time: time.Now(),
		},
		Status: status,
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: info.RemoteAddr,
		},
		Method:       cmp.Or(info.TokenJoinMethod, info.RequestedJoinMethod),
		TokenName:    info.SafeTokenName,
		TokenExpires: info.TokenExpires,
		Role:         info.Role,
		NodeName:     info.NodeName,
	}
}
