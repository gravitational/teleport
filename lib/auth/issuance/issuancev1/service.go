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
	v1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/issuance/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/internal/cert"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type authServer interface {
	AccessCheckerForScope(ctx context.Context, scope string, userState services.UserState, allowedResourceAccessIDs []types.ResourceAccessID) (*services.SplitAccessCheckerContext, error)
	GenerateUserCert(ctx context.Context, req cert.Request) (*proto.Certs, error)
}

type cache interface {
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
}

type Service struct {
	v1pb.UnimplementedIssuanceServiceServer
	scopedAuthorizer authz.ScopedAuthorizer
	cache            cache
	authServer       authServer
}

// ServiceConfig is the config for instantiating a Service
type ServiceConfig struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Cache            cache
	AuthServer       authServer
}

// NewService returns a new issuancev1 gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.ScopedAuthorizer != nil:
		return nil, trace.BadParameter("scoped authorizer is required")
	case cfg.Cache != nil:
		return nil, trace.BadParameter("cache is required")
	case cfg.AuthServer != nil:
		return nil, trace.BadParameter("auth server is required")
	}

	return &Service{
		scopedAuthorizer: cfg.ScopedAuthorizer,
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
	req *v1pb.IssueScopedBotCertsRequest,
) (*v1pb.IssueScopedBotCertsResponse, error) {
	// Temporarily, we need to check that Scopes + MWI Scopes is enabled to
	// derisk introduction of this RPC.
	if err := scopes.AssertMWIFeatureEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform basic RBAC checks to ensure the correct kind of identity is
	// invoking the endpoint
	currentIdentity := authCtx.Identity.GetIdentity()
	switch {
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

	// Fetch Bot User to ensure it still exists and is coherent to the current
	// identity
	user, err := s.cache.GetUser(ctx, currentIdentity.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Ensure that this actually is a Bot User and that this is scoped.
	// This really accounts for an awkward scenario where the Bot config has
	// changed since the issuance of the current identity.
	if !user.IsBot() {
		return nil, trace.BadParameter("user %q is not a bot", user.GetName())
	}
	botScope, _ := user.GetLabel(types.BotScopeLabel)
	if botScope == "" {
		return nil, trace.BadParameter("user %q has no bot scope label", user.GetName())
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
	// Sanity check that the scope is also descendent or equiv to the bot's scope
	rel := scopes.Compare(botScope, requestedScope)
	if !(rel == scopes.Equivalent || rel == scopes.Descendant) {
		return nil, trace.AccessDenied(
			"requested scope %q is not descendent or equivalent to bot's scope %q",
			requestedScope,
			botScope,
		)
	}

	checker, err := s.authServer.AccessCheckerForScope(
		ctx, requestedScope, user, []types.ResourceAccessID{},
	)
	if err != nil {
		return nil, trace.Wrap(err, "building access checker")
	}
	certReq := cert.Request{
		User:           user,
		CheckerContext: checker,
		// TODO-CRITICAL(strideynet): Validate TTL.
		TTL:          req.Ttl.AsDuration(),
		SSHPublicKey: req.SshPublicKey,
		TLSPublicKey: req.TlsPublicKey,

		// Explicitly set BotInternal to false as these certs are intended for
		// outputs/services and not use by the bot internally. This prevents
		// using them further issuance.
		BotInternal: false,
		// Explicitly reject use for further issuance
		DisallowReissue: true,

		// Propagate certain attributes from current identity to generated
		// certificates
		JoinAttributes: currentIdentity.JoinAttributes,
		LoginIP:        currentIdentity.LoginIP,
		BotName:        currentIdentity.BotName,
		BotInstanceID:  currentIdentity.BotInstanceID,
	}

	// nb(strideynet): One day, we'll want to pull more of the logic around
	// cert generation into this package rather than invoking this via the
	// auth server struct.
	certs, err := s.authServer.GenerateUserCert(ctx, certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Notably, we do not return any CAs. The Bot already has an internal
	// identity and the ability to fetch/watch CAs. Returning CAs here would
	// create confusion around where to correctly source CAs.
	return &v1pb.IssueScopedBotCertsResponse{
		Certs: &v1pb.Certs{
			Tls: certs.TLS,
			Ssh: certs.SSH,
		},
	}, nil
}
