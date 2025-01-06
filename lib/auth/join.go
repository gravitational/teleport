/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
)

// tokenJoinMethod returns the join method of the token with the given tokenName
func (a *Server) tokenJoinMethod(ctx context.Context, tokenName string) types.JoinMethod {
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		// could not find dynamic token, assume static token. If it does not
		// exist this will be caught later.
		return types.JoinMethodToken
	}
	return provisionToken.GetJoinMethod()
}

// checkTokenJoinRequestCommon checks all token join rules that are common to
// all join methods, including token existence, token TTL, and allowed roles.
func (a *Server) checkTokenJoinRequestCommon(ctx context.Context, req *types.RegisterUsingTokenRequest) (types.ProvisionToken, error) {
	// make sure the token is valid
	provisionToken, err := a.ValidateToken(ctx, req.Token)
	if err != nil {
		a.logger.WarnContext(ctx, "cannot join the cluster with invalid token",
			"node_name", req.NodeName,
			"role", req.Role,
			"error", err,
		)
		msg := "the token is not valid" // default to most generic message
		if strings.Contains(err.Error(), TokenExpiredOrNotFound) {
			// propagate ExpiredOrNotFound message so that clients can attempt
			// assertion-based fallback if appropriate.
			msg = TokenExpiredOrNotFound
		}
		return nil, trace.AccessDenied("%q can not join the cluster with role %q, %s", req.NodeName, req.Role, msg)
	}

	// instance certs can be requested by any agent that has at least one local service role (e.g. proxy, node, etc).
	if req.Role == types.RoleInstance {
		hasLocalServiceRole := false
		for _, role := range provisionToken.GetRoles() {
			if role.IsLocalService() {
				hasLocalServiceRole = true
				break
			}
		}
		if !hasLocalServiceRole {
			a.logger.WarnContext(ctx, "cannot requisition instance certs (token contains no local service roles)",
				"node_name", req.NodeName,
				"host_id", req.HostID,
			)
			return nil, trace.AccessDenied("%s [%v] cannot requisition instance certs (token contains no local service roles)", req.NodeName, req.HostID)
		}
	}

	// make sure the caller is requesting a role allowed by the token
	if !provisionToken.GetRoles().Include(req.Role) && req.Role != types.RoleInstance {
		a.logger.WarnContext(ctx, "token does not allow role to join the cluster",
			"node_name", req.NodeName,
			"host_id", req.HostID,
			"role", req.Role,
		)
		return nil, trace.BadParameter("node %q [%v] can not join the cluster, the token does not allow %q role", req.NodeName, req.HostID, req.Role)
	}

	return provisionToken, nil
}

type joinAttributeSourcer interface {
	// JoinAuditAttributes returns a series of attributes that can be inserted into
	// audit events related to a specific join.
	JoinAuditAttributes() (map[string]interface{}, error)
}

func setRemoteAddrFromContext(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	var addr string
	if clientIP, err := authz.ClientSrcAddrFromContext(ctx); err == nil {
		addr = clientIP.String()
	} else if p, ok := peer.FromContext(ctx); ok {
		addr = p.Addr.String()
	}
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return trace.Wrap(err)
	}
	req.RemoteAddr = ip

	return nil
}

// handleJoinFailure logs and audits the failure of a join. It is intentionally
// designed to handle potential nullness of the input parameters.
func (a *Server) handleJoinFailure(
	ctx context.Context,
	origErr error,
	pt types.ProvisionToken,
	attributeSource joinAttributeSourcer,
	req *types.RegisterUsingTokenRequest,
) {
	attrs := []slog.Attr{slog.Any("error", origErr)}
	if req != nil {
		attrs = append(attrs, []slog.Attr{
			slog.String("role", string(req.Role)),
			slog.String("host_id", req.HostID),
			slog.String("node_name", req.NodeName),
			slog.String("remote_addr", req.RemoteAddr),
		}...)
	}

	// Fetch and encode attributes if they are available.
	var attributesProto *apievents.Struct
	if attributeSource != nil {
		var err error
		attributes, err := attributeSource.JoinAuditAttributes()
		if err != nil {
			a.logger.WarnContext(ctx, "Unable to fetch join attributes from join method", "error", err)
		}
		attrs = append(attrs, slog.Any("attributes", attributes))
		attributesProto, err = apievents.EncodeMap(attributes)
		if err != nil {
			a.logger.WarnContext(ctx, "Unable to encode join attributes for audit event", "error", err)
		}
	}

	// Add log fields from token if available.
	if pt != nil {
		attrs = append(attrs, slog.String("join_method", string(pt.GetJoinMethod())))
		attrs = append(attrs, slog.String("token_name", pt.GetSafeName()))
	}
	a.logger.LogAttrs(ctx, slog.LevelWarn, "Failure to join cluster occurred", attrs...)

	var evt apievents.AuditEvent
	status := apievents.Status{
		Success: false,
		Error:   origErr.Error(),
	}
	if req != nil && req.Role == types.RoleBot {
		botJoinEvent := &apievents.BotJoin{
			Metadata: apievents.Metadata{
				Type: events.BotJoinEvent,
				Code: events.BotJoinFailureCode,
			},
			Status:     status,
			Attributes: attributesProto,
			ConnectionMetadata: apievents.ConnectionMetadata{
				RemoteAddr: req.RemoteAddr,
			},
		}
		if pt != nil {
			botJoinEvent.Method = string(pt.GetJoinMethod())
			botJoinEvent.TokenName = pt.GetSafeName()
			botJoinEvent.BotName = pt.GetBotName()
		}
		evt = botJoinEvent
	} else {
		instanceJoinEvent := &apievents.InstanceJoin{
			Metadata: apievents.Metadata{
				Type: events.InstanceJoinEvent,
				Code: events.InstanceJoinFailureCode,
			},
			Status:     status,
			Attributes: attributesProto,
		}
		if pt != nil {
			instanceJoinEvent.Method = string(pt.GetJoinMethod())
			instanceJoinEvent.TokenName = pt.GetSafeName()
		}
		if req != nil {
			instanceJoinEvent.Role = string(req.Role)
			instanceJoinEvent.NodeName = req.NodeName
			instanceJoinEvent.HostID = req.HostID
			instanceJoinEvent.RemoteAddr = req.RemoteAddr
		}
		evt = instanceJoinEvent
	}
	if err := a.emitter.EmitAuditEvent(a.closeCtx, evt); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit failed join event", "error", err)
	}
}

// RegisterUsingToken returns credentials for a new node to join the Teleport
// cluster using a previously issued token.
//
// A node must also request a specific role (and the role must match one of the roles
// the token was generated for.)
//
// If a token was generated with a TTL, it gets enforced (can't register new
// nodes after TTL expires.)
//
// If the token includes a specific join method, the rules for that join method
// will be checked.
func (a *Server) RegisterUsingToken(ctx context.Context, req *types.RegisterUsingTokenRequest) (certs *proto.Certs, err error) {
	var joinAttributeSrc joinAttributeSourcer
	var provisionToken types.ProvisionToken
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(ctx, err, provisionToken, joinAttributeSrc, req)
		}
	}()

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	method := a.tokenJoinMethod(ctx, req.Token)
	switch method {
	case types.JoinMethodEC2:
		if err := a.checkEC2JoinRequest(ctx, req); err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodIAM, types.JoinMethodAzure, types.JoinMethodTPM:
		// IAM and Azure join methods must use gRPC register methods
		return nil, trace.AccessDenied("this token is only valid for the %s "+
			"join method but the node has connected to the wrong endpoint, make "+
			"sure your node is configured to use the %s join method", method, method)
	case types.JoinMethodGitHub:
		claims, err := a.checkGitHubJoinRequest(ctx, req)
		if claims != nil {
			joinAttributeSrc = claims
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodGitLab:
		claims, err := a.checkGitLabJoinRequest(ctx, req)
		if claims != nil {
			joinAttributeSrc = claims
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodCircleCI:
		claims, err := a.checkCircleCIJoinRequest(ctx, req)
		if claims != nil {
			joinAttributeSrc = claims
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodKubernetes:
		claims, err := a.checkKubernetesJoinRequest(ctx, req)
		if claims != nil {
			joinAttributeSrc = claims
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodGCP:
		claims, err := a.checkGCPJoinRequest(ctx, req)
		if claims != nil {
			joinAttributeSrc = claims
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodSpacelift:
		claims, err := a.checkSpaceliftJoinRequest(ctx, req)
		if claims != nil {
			joinAttributeSrc = claims
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodTerraformCloud:
		claims, err := a.checkTerraformCloudJoinRequest(ctx, req)
		if claims != nil {
			joinAttributeSrc = claims
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodBitbucket:
		claims, err := a.checkBitbucketJoinRequest(ctx, req)
		if claims != nil {
			joinAttributeSrc = claims
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodToken:
		// carry on to common token checking logic
	default:
		// this is a logic error, all valid join methods should be captured
		// above (empty join method will be set to JoinMethodToken by
		// CheckAndSetDefaults)
		return nil, trace.BadParameter("unrecognized token join method")
	}

	// perform common token checks
	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// With all elements of the token validated, we can now generate & return
	// certificates.
	if req.Role == types.RoleBot {
		certs, err = a.generateCertsBot(ctx, provisionToken, req, joinAttributeSrc)
		return certs, trace.Wrap(err)
	}
	certs, err = a.generateCerts(ctx, provisionToken, req, joinAttributeSrc)
	return certs, trace.Wrap(err)
}

func (a *Server) generateCertsBot(
	ctx context.Context,
	provisionToken types.ProvisionToken,
	req *types.RegisterUsingTokenRequest,
	joinAttributeSrc joinAttributeSourcer,
) (*proto.Certs, error) {
	// bots use this endpoint but get a user cert
	// botResourceName must be set, enforced in CheckAndSetDefaults
	botName := provisionToken.GetBotName()
	joinMethod := provisionToken.GetJoinMethod()

	// Check this is a join method for bots we support.
	if !slices.Contains(machineidv1.SupportedJoinMethods, joinMethod) {
		return nil, trace.BadParameter(
			"unsupported join method %q for bot", joinMethod,
		)
	}

	// Most join methods produce non-renewable certificates and join must be
	// called again to fetch fresh certificates with a longer lifetime. These
	// join methods do not delete the token after use.
	renewable := false
	shouldDeleteToken := false
	if joinMethod == types.JoinMethodToken {
		// The token join method is special and produces renewable certificates
		// but the token is deleted after use.
		shouldDeleteToken = true
		renewable = true
	}

	expires := a.GetClock().Now().Add(defaults.DefaultRenewableCertTTL)
	if req.Expires != nil {
		expires = *req.Expires
	}

	// Construct a Join event to be sent later.
	joinEvent := &apievents.BotJoin{
		Metadata: apievents.Metadata{
			Type: events.BotJoinEvent,
			Code: events.BotJoinCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		BotName:   botName,
		Method:    string(joinMethod),
		TokenName: provisionToken.GetSafeName(),
		UserName:  machineidv1.BotResourceName(botName),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: req.RemoteAddr,
		},
	}

	auth := &machineidv1pb.BotInstanceStatusAuthentication{
		AuthenticatedAt: timestamppb.New(a.GetClock().Now()),
		// TODO: GetSafeName may not return an appropriate value for later
		// comparison / locking purposes, and this also shouldn't contain
		// secrets. Should we hash it?
		JoinToken:  provisionToken.GetSafeName(),
		JoinMethod: string(provisionToken.GetJoinMethod()),
		// TODO(nklaassen): consider logging the SSH public key as well, for now
		// the SSH and TLS public keys are still identical for tbot.
		PublicKey: req.PublicTLSKey,
	}

	if joinAttributeSrc != nil {
		attributes, err := joinAttributeSrc.JoinAuditAttributes()
		if err != nil {
			a.logger.WarnContext(ctx, "Unable to fetch join attributes from join method", "error", err)
		}
		joinEvent.Attributes, err = apievents.EncodeMap(attributes)
		if err != nil {
			a.logger.WarnContext(ctx, "Unable to encode join attributes for audit event", "error", err)
		}

		auth.Metadata, err = structpb.NewStruct(attributes)
		if err != nil {
			a.logger.WarnContext(ctx, "Unable to encode struct value for join metadata", "error", err)
		}
	}

	certs, botInstanceID, err := a.generateInitialBotCerts(
		ctx,
		botName,
		machineidv1.BotResourceName(botName),
		req.RemoteAddr,
		req.PublicSSHKey,
		req.PublicTLSKey,
		expires,
		renewable,
		auth,
		req.BotInstanceID,
		req.BotGeneration,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	joinEvent.BotInstanceID = botInstanceID

	if shouldDeleteToken {
		// delete ephemeral bot join tokens so they can't be re-used
		if err := a.DeleteToken(ctx, provisionToken.GetName()); err != nil {
			a.logger.WarnContext(ctx, "Could not delete bot provision token after generating certs",
				"provision_token", provisionToken.GetSafeName(),
				"error", err,
			)
		}
	}

	// Emit audit event for bot join.
	a.logger.InfoContext(ctx, "Bot has joined the cluster",
		"bot_name", botName,
		"bot_instance", botInstanceID,
	)
	if err := a.emitter.EmitAuditEvent(ctx, joinEvent); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit bot join event", "error", err)
	}
	return certs, nil
}

func (a *Server) generateCerts(
	ctx context.Context,
	provisionToken types.ProvisionToken,
	req *types.RegisterUsingTokenRequest,
	joinAttributeSrc joinAttributeSourcer,
) (*proto.Certs, error) {
	if req.Expires != nil {
		return nil, trace.BadParameter("'expires' cannot be set on join for non-bot certificates")
	}

	// instance certs include an additional field that specifies the list of
	// all services authorized by the token.
	var systemRoles []types.SystemRole
	if req.Role == types.RoleInstance {
		for _, r := range provisionToken.GetRoles() {
			if r.IsLocalService() {
				systemRoles = append(systemRoles, r)
			} else {
				a.logger.WarnContext(ctx, "Omitting non-service system role from instance cert", "system_role", string(r))
			}
		}
	}

	// generate and return host certificate and keys
	certs, err := a.GenerateHostCerts(ctx,
		&proto.HostCertsRequest{
			HostID:               req.HostID,
			NodeName:             req.NodeName,
			Role:                 req.Role,
			AdditionalPrincipals: req.AdditionalPrincipals,
			PublicTLSKey:         req.PublicTLSKey,
			PublicSSHKey:         req.PublicSSHKey,
			RemoteAddr:           req.RemoteAddr,
			DNSNames:             req.DNSNames,
			SystemRoles:          systemRoles,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Emit audit event
	if req.Role == types.RoleInstance {
		a.logger.InfoContext(ctx, "Instance has joined the cluster",
			"node_name", req.NodeName,
			"host_id", req.HostID,
			"role", req.Role,
			"system_roles", systemRoles,
		)
	} else {
		a.logger.InfoContext(ctx, "Instance has joined the cluster",
			"node_name", req.NodeName,
			"host_id", req.HostID,
			"role", req.Role,
		)
	}
	joinEvent := &apievents.InstanceJoin{
		Metadata: apievents.Metadata{
			Type: events.InstanceJoinEvent,
			Code: events.InstanceJoinCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		NodeName:     req.NodeName,
		Role:         string(req.Role),
		Method:       string(provisionToken.GetJoinMethod()),
		TokenName:    provisionToken.GetSafeName(),
		TokenExpires: provisionToken.Expiry(),
		HostID:       req.HostID,
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: req.RemoteAddr,
		},
	}
	if joinAttributeSrc != nil {
		attributes, err := joinAttributeSrc.JoinAuditAttributes()
		if err != nil {
			a.logger.WarnContext(ctx, "Unable to fetch join attributes from join method", "error", err)
		}
		joinEvent.Attributes, err = apievents.EncodeMap(attributes)
		if err != nil {
			a.logger.WarnContext(ctx, "Unable to encode join attributes for audit event", "error", err)
		}
	}
	if err := a.emitter.EmitAuditEvent(ctx, joinEvent); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit instance join event", "error", err)
	}
	return certs, nil
}

func generateChallenge(encoding *base64.Encoding, length int) (string, error) {
	// read crypto-random bytes to generate the challenge
	challengeRawBytes := make([]byte, length)
	if _, err := rand.Read(challengeRawBytes); err != nil {
		return "", trace.Wrap(err)
	}

	// encode the challenge to base64 so it can be sent over HTTP
	return encoding.EncodeToString(challengeRawBytes), nil
}
