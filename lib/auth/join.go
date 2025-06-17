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
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
)

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

// handleJoinFailure logs and audits the failure of a join. It is intentionally
// designed to handle potential nullness of the input parameters.
func (a *Server) handleJoinFailure(
	ctx context.Context,
	origErr error,
	pt types.ProvisionToken,
	rawJoinAttrs any,
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

	// Fetch and encode rawJoinAttrs if they are available.
	attributesStruct, err := rawJoinAttrsToStruct(rawJoinAttrs)
	if err != nil {
		a.logger.WarnContext(ctx, "Unable to fetch join attributes from join method", "error", err)
	}
	if attributesStruct != nil {
		attrs = append(attrs, slog.Any("attributes", attributesStruct))
	}

	// Add log fields from token if available.
	if pt != nil {
		attrs = append(attrs, slog.String("join_method", string(pt.GetJoinMethod())))
		attrs = append(attrs, slog.String("token_name", pt.GetSafeName()))
	}
	a.logger.LogAttrs(ctx, slog.LevelWarn, "Failure to join cluster occurred", attrs...)

	errorMessage := origErr.Error()
	if errors.Is(origErr, context.Canceled) || status.Code(origErr) == codes.Canceled {
		errorMessage = "join attempt timed out or was aborted"
	}

	var evt apievents.AuditEvent
	status := apievents.Status{
		Success: false,
		Error:   errorMessage,
	}
	if req != nil && req.Role == types.RoleBot {
		botJoinEvent := &apievents.BotJoin{
			Metadata: apievents.Metadata{
				Type: events.BotJoinEvent,
				Code: events.BotJoinFailureCode,
			},
			Status:     status,
			Attributes: attributesStruct,
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
			Attributes: attributesStruct,
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
	attrs := &workloadidentityv1pb.JoinAttrs{}
	var rawClaims any
	var provisionToken types.ProvisionToken
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(ctx, err, provisionToken, rawClaims, req)
		}
	}()

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// perform common token checks
	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// nb: token returned by checkTokenJoinRequestCommon may be a
	// ProvisionTokenV2 derived from a static token. You cannot assume you will
	// be able to fetch this same token directly from the backend.
	method := provisionToken.GetJoinMethod()

	// Call join method-specific validation
	switch provisionToken.GetJoinMethod() {
	case types.JoinMethodEC2:
		if err := a.checkEC2JoinRequest(ctx, req); err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodIAM, types.JoinMethodAzure, types.JoinMethodTPM,
		types.JoinMethodOracle, types.JoinMethodBoundKeypair:
		// Some join methods require use of a specific RPC - reject those here.
		// This would generally be a developer error - but can be triggered if
		// the user has configured the wrong join method on the client-side.
		return nil, trace.AccessDenied("this token is only valid for the %s "+
			"join method but the node has connected to the wrong endpoint, make "+
			"sure your node is configured to use the %s join method", method, method)
	case types.JoinMethodGitHub:
		claims, err := a.checkGitHubJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims
			attrs.Github = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodGitLab:
		claims, err := a.checkGitLabJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims
			attrs.Gitlab = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodCircleCI:
		claims, err := a.checkCircleCIJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims
			attrs.Circleci = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodKubernetes:
		claims, err := a.checkKubernetesJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims
			attrs.Kubernetes = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodGCP:
		claims, err := a.checkGCPJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims
			attrs.Gcp = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodSpacelift:
		claims, err := a.checkSpaceliftJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims
			attrs.Spacelift = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodTerraformCloud:
		claims, err := a.checkTerraformCloudJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims
			attrs.TerraformCloud = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodBitbucket:
		claims, err := a.checkBitbucketJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims
			attrs.Bitbucket = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodAzureDevops:
		claims, err := a.checkAzureDevopsJoinRequest(ctx, req, provisionToken)
		if claims != nil {
			rawClaims = claims.ForAudit()
			attrs.AzureDevops = claims.JoinAttrs()
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodToken:
		// no additional validation to perform - the name is enough.
	default:
		// this is a logic error, all valid join methods should be captured
		// above (empty join method will be set to JoinMethodToken by
		// CheckAndSetDefaults)
		return nil, trace.BadParameter("unrecognized token join method")
	}

	// With all elements of the token validated, we can now generate & return
	// certificates.
	if req.Role == types.RoleBot {
		certs, _, err = a.generateCertsBot(
			ctx,
			provisionToken,
			req,
			rawClaims,
			attrs,
		)
		return certs, trace.Wrap(err)
	}
	certs, err = a.generateCerts(ctx, provisionToken, req, rawClaims)
	return certs, trace.Wrap(err)
}

func (a *Server) generateCertsBot(
	ctx context.Context,
	provisionToken types.ProvisionToken,
	req *types.RegisterUsingTokenRequest,
	rawJoinClaims any,
	attrs *workloadidentityv1pb.JoinAttrs,
) (*proto.Certs, string, error) {
	// bots use this endpoint but get a user cert
	// botResourceName must be set, enforced in CheckAndSetDefaults
	botName := provisionToken.GetBotName()
	joinMethod := provisionToken.GetJoinMethod()

	// Check this is a join method for bots we support.
	if !slices.Contains(machineidv1.SupportedJoinMethods, joinMethod) {
		return nil, "", trace.BadParameter(
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
	var err error
	joinEvent.Attributes, err = rawJoinAttrsToStruct(rawJoinClaims)
	if err != nil {
		a.logger.WarnContext(
			ctx,
			"Unable to encode join attributes for join audit event",
			"error", err,
		)
	}

	// Prepare join attributes for encoding into the X509 cert and for inclusion
	// in audit logs.
	if attrs == nil {
		attrs = &workloadidentityv1pb.JoinAttrs{}
	}
	attrs.Meta = &workloadidentityv1pb.JoinAttrsMeta{
		JoinMethod: string(joinMethod),
	}
	if joinMethod != types.JoinMethodToken {
		attrs.Meta.JoinTokenName = provisionToken.GetName()
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
		JoinAttrs: attrs,
	}

	// TODO(noah): In v19, we can drop writing to the deprecated Metadata field.
	auth.Metadata, err = rawJoinAttrsToGoogleStruct(rawJoinClaims)
	if err != nil {
		a.logger.WarnContext(ctx, "Unable to encode struct value for join metadata", "error", err)
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
		req.PreviousBotInstanceID,
		req.BotGeneration,
		attrs,
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
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
	return certs, botInstanceID, nil
}

func (a *Server) generateCerts(
	ctx context.Context,
	provisionToken types.ProvisionToken,
	req *types.RegisterUsingTokenRequest,
	rawJoinClaims any,
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
	joinEvent.Attributes, err = rawJoinAttrsToStruct(rawJoinClaims)
	if err != nil {
		a.logger.WarnContext(ctx, "Unable to fetch join attributes from join method", "error", err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, joinEvent); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit instance join event", "error", err)
	}
	return certs, nil
}

func rawJoinAttrsToStruct(in any) (*apievents.Struct, error) {
	if in == nil {
		return nil, nil
	}
	attrBytes, err := json.Marshal(in)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling join attributes")
	}
	out := &apievents.Struct{}
	if err := out.UnmarshalJSON(attrBytes); err != nil {
		return nil, trace.Wrap(err, "unmarshaling join attributes")
	}
	return out, nil
}

func rawJoinAttrsToGoogleStruct(in any) (*structpb.Struct, error) {
	if in == nil {
		return nil, nil
	}
	attrBytes, err := json.Marshal(in)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling join attributes")
	}
	out := &structpb.Struct{}
	if err := out.UnmarshalJSON(attrBytes); err != nil {
		return nil, trace.Wrap(err, "unmarshaling join attributes")
	}
	return out, nil
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
