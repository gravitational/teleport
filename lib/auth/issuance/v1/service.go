/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package issuancev1

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	issuancev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/issuance/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/internal/cert"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type authServer interface {
	AccessCheckerForScope(ctx context.Context, scope string, userState services.UserState, allowedResourceAccessIDs []types.ResourceAccessID) (*services.ScopedAccessCheckerContext, error)
	GenerateUserCert(ctx context.Context, req cert.Request) (*proto.Certs, error)
}

type cache interface{}

// Service implements teleport.issuance.v1.IssuanceService
//
// It provides methods for issuing certificates and other types of credential.
// It's designed to be called by already authenticated users and not to provide
// authentication (i.e. login).
//
// Today, it only supports methods for Bots to issue certificates to themselves,
// but may be extended in the future to support other types of certificate
// issuance.
//
// Some deviations from historical issuance methods in Teleport:
//
//  1. Rather than accept a desired certificate expiry time, accept a desired
//     TTL. This avoids issues caused by client clock skew.
//  2. Avoid returning certificate authority information. The client is already
//     authenticated and has access to this information. If we ever must return
//     CA information, we MUST ensure that different CA types are not
//     intermingled - it must be explicit what CA is being returned.
type Service struct {
	issuancev1pb.UnimplementedIssuanceServiceServer
	scopedAuthorizer authz.ScopedAuthorizer
	cache            cache
	authServer       authServer
}

// ServiceConfig is the config for instantiating a [Service].
type ServiceConfig struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Cache            cache
	AuthServer       authServer
}

// NewService returns a new [Service].
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.ScopedAuthorizer == nil:
		return nil, trace.BadParameter("scoped authorizer is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("auth server is required")
	}

	return &Service{
		scopedAuthorizer: cfg.ScopedAuthorizer,
		cache:            cfg.Cache,
		authServer:       cfg.AuthServer,
	}, nil
}

// IssueScopedBotCerts issues scoped certificates to an already scoped bot.
// These scoped certificates are intended to be used in tbot outputs/services.
// It can only be invoked by a scoped bot.
//
// This RPC explicitly permits the generation of certificates that may outlive
// the current TTL of the identity.
func (s *Service) IssueScopedBotCerts(
	ctx context.Context,
	req *issuancev1pb.IssueScopedBotCertsRequest,
) (*issuancev1pb.IssueScopedBotCertsResponse, error) {
	// Temporarily, we need to check that Scopes + MWI Scopes is enabled to
	// derisk introduction of this RPC.
	if err := scopes.AssertMWIFeatureEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform any basic validity checks
	ttl := req.Ttl.AsDuration()
	if ttl <= 0 {
		return nil, trace.BadParameter(
			"ttl: must be provided and positive",
		)
	}
	if ttl > defaults.MaxRenewableCertTTL {
		return nil, trace.BadParameter(
			"ttl: value (%s) exceeds maximum permitted value (%s)",
			ttl,
			defaults.MaxRenewableCertTTL,
		)
	}

	// Perform basic RBAC checks to ensure the correct kind of identity is
	// invoking the endpoint
	currentIdentity := authCtx.Identity.GetIdentity()
	switch {
	case !currentIdentity.IsBot():
		return nil, trace.BadParameter(
			"IssueScopedBotCerts can only be invoked by bots",
		)
	case !currentIdentity.BotInternal:
		return nil, trace.AccessDenied(
			"bot identity is not an internal identity",
		)
	case currentIdentity.DisallowReissue:
		return nil, trace.AccessDenied("reissuance is prohibited")
	case currentIdentity.ScopePin == nil || currentIdentity.ScopePin.Scope == "":
		return nil, trace.AccessDenied(
			"scope pin missing, rpc can only be invoked by scoped identities",
		)
	}

	// Perform some consistency checks against the user to catch cases where
	// the user may have changed substantially since certificate issuance.
	user := authCtx.User
	if !user.IsBot() {
		return nil, trace.BadParameter(
			"user %q is not a bot", user.GetName(),
		)
	}
	botScope, _ := user.GetLabel(types.BotScopeLabel)
	if botScope == "" {
		return nil, trace.BadParameter(
			"user %q has no bot scope label", user.GetName(),
		)
	}
	if err := scopes.StrongValidate(botScope); err != nil {
		return nil, trace.Wrap(err, "validating bot user scope")
	}

	// nb(strideynet): Today, this endpoint will only generate certs with the
	// same scope as the current tlsidentity, and the same scope as the Bot
	// resource itself. In the future, we will allow "sub-pinning" where the
	// resulting certs may be a descendent scope of the current scope and
	// bot scope.
	requestedScope := currentIdentity.ScopePin.Scope
	// Sanity check that the requested scope is still descendent or equiv to
	// botScope - in case bot scope has changed.
	rel := scopes.Compare(botScope, requestedScope)
	if !(rel == scopes.Equivalent || rel == scopes.Descendant) {
		return nil, trace.AccessDenied(
			"requested scope %q is not descendent or equivalent to bot's scope %q",
			requestedScope,
			botScope,
		)
	}

	// Now we've performed

	checker, err := s.authServer.AccessCheckerForScope(
		ctx, requestedScope, user, []types.ResourceAccessID{},
	)
	if err != nil {
		return nil, trace.Wrap(err, "building access checker")
	}
	certReq := cert.Request{
		User:           user,
		CheckerContext: checker,
		TTL:            ttl,
		SSHPublicKey:   req.SshPublicKey,
		TLSPublicKey:   req.TlsPublicKey,

		// Explicitly set BotInternal to false as these certs are intended for
		// outputs/services and not use by the bot internally. This prevents
		// using them further issuance.
		BotInternal: false,
		// Explicitly reject use for further issuance
		DisallowReissue: true,

		// We do not pass traits from the Bot user here. This is because we do
		// not currently support traits for scoped Bots. Users shouldn't be able
		// to set these due to validation but this acts as a back-stop.

		// Propagate certain attributes from current identity to generated
		// certificates
		JoinAttributes: currentIdentity.JoinAttributes,
		LoginIP:        currentIdentity.LoginIP,
		BotName:        currentIdentity.BotName,
		BotInstanceID:  currentIdentity.BotInstanceID,
		JoinToken:      currentIdentity.JoinToken,
	}

	// nb(strideynet): One day, we'll want to pull more of the logic around
	// cert generation into this package rather than invoking this via the
	// auth server struct.
	certs, err := s.authServer.GenerateUserCert(ctx, certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We do not return any CAs. The Bot already has an internal identity and
	// the ability to fetch/watch CAs. Returning CAs here would create confusion
	// around where to correctly source CAs.
	return &issuancev1pb.IssueScopedBotCertsResponse{
		Certs: &issuancev1pb.Certs{
			Tls: certs.TLS,
			Ssh: certs.SSH,
		},
	}, nil
}
