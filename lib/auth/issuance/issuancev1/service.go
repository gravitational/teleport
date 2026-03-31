package issuancev1

import (
	"context"

	"github.com/gravitational/trace"

	v1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/issuance/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/internal/cert"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type authServer interface {
}

type Cache interface {
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
}

type Service struct {
	v1pb.UnimplementedIssuanceServiceServer
	scopedAuthorizer authz.ScopedAuthorizer
	cache            Cache
}

// ServiceConfig is the config for instantiating a Service
type ServiceConfig struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Cache            Cache
}

// NewService returns a new issuancev1 gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.ScopedAuthorizer != nil:
		return nil, trace.BadParameter("scoped authorizer is required")
	case cfg.Cache != nil:
		return nil, trace.BadParameter("cache is required")
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
		return nil, trace.AccessDenied("bot identity is not an internal identity")
	case currentIdentity.DisallowReissue:
		return nil, trace.AccessDenied("reissuance is prohibited")
	case currentIdentity.ScopePin == nil || currentIdentity.ScopePin.Scope == "":
		return nil, trace.AccessDenied("scope pin missing, rpc can only be invoked by scoped identities")
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
		return nil, trace.AccessDenied("requested scope %q is not descendent or equivalent to bot's scope %q", requestedScope, botScope)
	}

	accessInfo := services.AccessInfoFromUserState(user)
	// As per RFD, today we do not support traits for scoped Bots. Explicitly
	// prevent this.
	accessInfo.Traits = nil

	certReq := cert.Request{
		User: user,
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

		// TODO(strideynet): Propagate following:
		JoinAttributes: nil,
		BotName:        "",
		BotInstanceID:  "",
	}

}
