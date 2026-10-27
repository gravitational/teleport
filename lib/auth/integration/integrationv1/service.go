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

package integrationv1

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/integrations"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// Cache is the subset of the cached resources that the Service queries.
type Cache interface {
	// GetClusterName returns local cluster name of the current auth server
	GetClusterName(ctx context.Context) (types.ClusterName, error)

	// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
	// controls if signing keys are loaded
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error)

	// GetProxies returns a list of registered proxies.
	//
	// Deprecated: Prefer paginated variant [ListProxyServers].
	//
	// TODO(kiosion): DELETE IN 21.0.0
	GetProxies() ([]types.Server, error)

	// ListProxyServers returns a paginated list of registered proxies.
	ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)

	// IntegrationsGetter defines methods to access Integration resources.
	services.IntegrationsGetter

	// DiscoveryConfigsGetter defines methods to access DiscoveryConfig resources.
	services.DiscoveryConfigsGetter

	// AppServersGetter defines methods to access application servers.
	services.AppServersGetter

	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}

// KeyStoreManager defines methods to get signers using the server's keystore.
type KeyStoreManager interface {
	// GetJWTSigner selects a usable JWT keypair from the given keySet and returns a [crypto.Signer].
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
	// NewSSHKeyPair generates a new SSH keypair in the keystore backend and returns it.
	NewSSHKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.SSHKeyPair, error)
	// GetSSHSignerFromKeySet selects a usable SSH keypair from the provided key set.
	GetSSHSignerFromKeySet(ctx context.Context, keySet types.CAKeySet) (ssh.Signer, error)
	// GetTLSCertAndSigner selects a usable TLS keypair from the given CA
	// and returns the PEM-encoded TLS certificate and a [crypto.Signer].
	GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error)
}

// Backend defines the interface for all the backend services that the
// integration service needs.
type Backend interface {
	services.IntegrationsInternal
	services.PluginStaticCredentialsInternal
	services.GitServers
	services.DiscoveryConfigs
	services.PresenceInternal
	services.OAuthProxyAuthRequests
}

// ServiceConfig holds configuration options for
// the Integration gRPC service.
type ServiceConfig struct {
	Authorizer      authz.Authorizer
	Backend         Backend
	StorageBackend  StorageBackend
	Cache           Cache
	KeyStoreManager KeyStoreManager
	Logger          *slog.Logger
	Clock           clockwork.Clock
	Emitter         apievents.Emitter
	// NotifyGitHubCallbackMigration is called after integration changes to
	// update the GitHub OAuth callback migration notification.
	NotifyGitHubCallbackMigration func()
	// ProxyPublicAddrGetter gets the public proxy address for validation.
	ProxyPublicAddrGetter func(context.Context) string
	// CheckGitHubIntegrationLicense returns an error if the GitHub integration
	// requires a Teleport Enterprise license.
	CheckGitHubIntegrationLicense func(ctx context.Context, spec *types.GitHubIntegrationSpecV1) error
	// FIPS means FedRAMP/FIPS compliant configuration was requested.
	FIPS bool

	// awsRolesAnywhereCreateSessionFn is a function that creates an AWS Roles Anywhere session.
	// This is used to allow mocking in tests, because the real implementation does
	// If not set, the default implementation is used.
	awsRolesAnywhereCreateSessionFn func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error)
}

// StorageBackend is used to atomically persist an integration, app server, and credential.
// It must be the same backend as Backend.
type StorageBackend interface {
	AtomicWrite(ctx context.Context, actions []backend.ConditionalAction) (revision string, err error)
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Cache == nil {
		return trace.BadParameter("cache is required")
	}

	if s.KeyStoreManager == nil {
		return trace.BadParameter("keystore manager is required")
	}

	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}

	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}

	if s.Emitter == nil {
		return trace.BadParameter("emitter is required")
	}

	if s.Logger == nil {
		s.Logger = slog.With(teleport.ComponentKey, "integrations.service")
	}

	if s.StorageBackend == nil {
		return trace.BadParameter("storage backend is required")
	}

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	if s.NotifyGitHubCallbackMigration == nil {
		return trace.BadParameter("NotifyGitHubCallbackMigration is required")
	}
	if s.ProxyPublicAddrGetter == nil {
		return trace.BadParameter("ProxyPublicAddrGetter is required")
	}
	if s.CheckGitHubIntegrationLicense == nil {
		return trace.BadParameter("CheckGitHubIntegrationLicense is required")
	}

	return nil
}

// Service implements the teleport.integration.v1.IntegrationService RPC service.
type Service struct {
	integrationpb.UnimplementedIntegrationServiceServer
	authorizer      authz.Authorizer
	cache           Cache
	keyStoreManager KeyStoreManager
	backend         Backend
	logger          *slog.Logger
	clock           clockwork.Clock
	emitter         apievents.Emitter

	awsRolesAnywhereCreateSessionFn func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error)
	notifyGitHubCallbackMigration   func()
	proxyPublicAddrGetter           func(context.Context) string
	checkGitHubIntegrationLicense   func(ctx context.Context, spec *types.GitHubIntegrationSpecV1) error
	fips                            bool
	storageBackend                  StorageBackend
}

// NewService returns a new Integrations gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		logger:                          cfg.Logger,
		authorizer:                      cfg.Authorizer,
		cache:                           cfg.Cache,
		keyStoreManager:                 cfg.KeyStoreManager,
		backend:                         cfg.Backend,
		clock:                           cfg.Clock,
		emitter:                         cfg.Emitter,
		awsRolesAnywhereCreateSessionFn: cfg.awsRolesAnywhereCreateSessionFn,
		notifyGitHubCallbackMigration:   cfg.NotifyGitHubCallbackMigration,
		proxyPublicAddrGetter:           cfg.ProxyPublicAddrGetter,
		checkGitHubIntegrationLicense:   cfg.CheckGitHubIntegrationLicense,
		fips:                            cfg.FIPS,
		storageBackend:                  cfg.StorageBackend,
	}, nil
}

var _ integrationpb.IntegrationServiceServer = (*Service)(nil)

// ListIntegrations returns a paginated list of all Integration resources.
func (s *Service) ListIntegrations(ctx context.Context, req *integrationpb.ListIntegrationsRequest) (*integrationpb.ListIntegrationsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// identity based defaulting for the filter is not yet implemented, so
	// default to the unscoped mode.
	if req.GetScopeFilter() == nil {
		req.SetScopeFilter(scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build())
	}
	results, nextKey, err := s.cache.ListIntegrations(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igs := make([]*types.IntegrationV1, len(results))
	for i, r := range results {
		r = r.WithoutCredentials()
		v1, ok := r.(*types.IntegrationV1)
		if !ok {
			return nil, trace.BadParameter("unexpected Integration type %T", r)
		}
		igs[i] = v1
	}

	return integrationpb.ListIntegrationsResponse_builder{
		Integrations: igs,
		NextKey:      nextKey,
	}.Build(), nil
}

// GetIntegration returns the specified Integration resource.
func (s *Service) GetIntegration(ctx context.Context, req *integrationpb.GetIntegrationRequest) (*types.IntegrationV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	integration, err := s.cache.GetIntegration(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Credentials are not used outside of Auth service.
	integration = integration.WithoutCredentials()
	igV1, ok := integration.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", integration)
	}

	return igV1, nil
}

// CreateIntegration creates a new Integration resource.
func (s *Service) CreateIntegration(ctx context.Context, req *integrationpb.CreateIntegrationRequest) (*types.IntegrationV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	switch req.GetIntegration().GetSubKind() {
	case types.IntegrationSubKindGitHub:
		spec := req.GetIntegration().GetGitHubIntegrationSpec()
		if spec == nil {
			return nil, trace.BadParameter("missing GitHub integration spec")
		}
		if err := s.checkGitHubIntegrationLicense(ctx, spec); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := s.validateGitHubOAuthCallbackURL(ctx, spec.OAuthCallbackURL); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := validateGitHubAllowProtocols(spec.AllowProtocols, s.fips); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := s.createGitHubCredentials(ctx, req.GetIntegration()); err != nil {
			return nil, trace.Wrap(err)
		}
	case types.IntegrationSubKindAWSOIDC, types.IntegrationSubKindAWSRolesAnywhere:
		if err := integrations.ValidName(req.GetIntegration().GetName()); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := validateAWSRolesAnywhereProfileFilters(req.GetIntegration()); err != nil {
			return nil, trace.Wrap(err)
		}
	case types.IntegrationSubKindOAuthProxy:
		if req.GetIntegration().GetOAuthProxyIntegrationSpec() == nil {
			return nil, trace.BadParameter("missing OAuth Proxy integration spec")
		}

		if err := validateOAuthProxyFIPS(s.fips); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := integrations.ValidName(req.GetIntegration().GetName()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var ig types.Integration
	switch req.GetIntegration().GetSubKind() {
	case types.IntegrationSubKindOAuthProxy:
		ig, err = s.createOAuthProxyIntegration(ctx, req.GetIntegration())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		ig, err = s.backend.CreateIntegration(ctx, req.GetIntegration())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	igMeta, err := getIntegrationMetadata(ig)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to build all integration metadata for audit event.", "error", err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.IntegrationCreate{
		Metadata: apievents.Metadata{
			Type: events.IntegrationCreateEvent,
			Code: events.IntegrationCreateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    ig.GetName(),
			Expires: ig.Expiry(),
			Scope:   ig.GetScope(),
		},
		IntegrationMetadata: igMeta,
		ConnectionMetadata:  authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit integration create event.", "error", err)
	}

	ig = ig.WithoutCredentials()
	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", ig)
	}

	s.postWriteOp(ig)
	return igV1, nil
}

// UpdateIntegration updates an existing integration.
func (s *Service) UpdateIntegration(ctx context.Context, req *integrationpb.UpdateIntegrationRequest) (*types.IntegrationV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	switch req.GetIntegration().GetSubKind() {
	case types.IntegrationSubKindGitHub:
		spec := req.GetIntegration().GetGitHubIntegrationSpec()
		if spec == nil {
			return nil, trace.BadParameter("missing GitHub integration spec")
		}
		if err := s.checkGitHubIntegrationLicense(ctx, spec); err != nil {
			return nil, trace.Wrap(err)
		}
		// Allow empty oauth_callback_url for backwards compatibility or
		// temporary rollback.
		if spec.OAuthCallbackURL != "" {
			if err := s.validateGitHubOAuthCallbackURL(ctx, spec.OAuthCallbackURL); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		if err := validateGitHubAllowProtocols(spec.AllowProtocols, s.fips); err != nil {
			return nil, trace.Wrap(err)
		}
	case types.IntegrationSubKindOAuthProxy:
		if err := validateOAuthProxyFIPS(s.fips); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := services.CheckAndSetDefaults(req.GetIntegration()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	oldIg, err := s.backend.GetIntegration(ctx, integrationpb.GetIntegrationRequest_builder{
		Name:  req.GetIntegration().GetName(),
		Scope: req.GetIntegration().GetScope(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Prevent changing subkind to/from oauth-proxy.
	if oldIg.IsOAuthProxy() != req.GetIntegration().IsOAuthProxy() {
		// TODO(nixpig): Expand this to _all_ integration kinds in a separate PR (see: https://github.com/gravitational/core/pull/2583#discussion_r4300544913)
		return nil, oldIg.CanChangeStateTo(req.GetIntegration())
	}

	var ig types.Integration
	switch req.GetIntegration().GetSubKind() {
	case types.IntegrationSubKindOAuthProxy:
		ig, err = s.updateOAuthProxyIntegration(ctx, req.GetIntegration(), oldIg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		if err := validateAWSRolesAnywhereProfileFilters(req.GetIntegration()); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := s.maybeUpdateStaticCredentials(ctx, req.GetIntegration(), oldIg); err != nil {
			return nil, trace.Wrap(err)
		}
		ig, err = s.backend.UpdateIntegration(ctx, req.GetIntegration())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	ig = ig.WithoutCredentials()
	igMeta, err := getIntegrationMetadata(ig)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to build all integration metadata for audit event.", "error", err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.IntegrationUpdate{
		Metadata: apievents.Metadata{
			Type: events.IntegrationUpdateEvent,
			Code: events.IntegrationUpdateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    ig.GetName(),
			Expires: ig.Expiry(),
			Scope:   ig.GetScope(),
		},
		IntegrationMetadata: igMeta,
		ConnectionMetadata:  authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit integration update event.", "error", err)
	}

	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", ig)
	}

	s.postWriteOp(ig)
	return igV1, nil
}

func (s *Service) validateGitHubOAuthCallbackURL(ctx context.Context, callbackURL string) error {
	proxyAddr, err := utils.ParseAddr(s.proxyPublicAddrGetter(ctx))
	if err != nil {
		return trace.Wrap(err, "parsing proxy addr")
	}

	// Accept with or without the default HTTPS port.
	allowed := []string{fmt.Sprintf("https://%s%s", proxyAddr.String(), types.IntegrationGitHubOAuthCallbackPath)}
	if proxyAddr.Port(0) == 443 {
		allowed = append(allowed, fmt.Sprintf("https://%s%s", proxyAddr.Host(), types.IntegrationGitHubOAuthCallbackPath))
	}

	if !slices.Contains(allowed, callbackURL) {
		return trace.BadParameter("oauth_callback_url %q does not match expected %q", callbackURL, allowed[len(allowed)-1])
	}
	return nil
}

// validateOAuthProxyFIPS returns an error when FIPS enabled.
func validateOAuthProxyFIPS(fips bool) error {
	if fips {
		return trace.BadParameter("OAuth Proxy integration not supported in FIPS mode")
	}

	return nil
}

func validateGitHubAllowProtocols(protocols []string, fips bool) error {
	// Credentials for HTTPS are encrypted with age, which is not FIPS approved.
	if fips && types.IsGitProtocolAllowed(protocols, types.GitProtocolHTTPS) {
		return trace.BadParameter("GitHub integration with HTTPS protocol is not supported in FIPS mode")
	}
	for _, protocol := range protocols {
		if protocol != types.GitProtocolSSH && protocol != types.GitProtocolHTTPS {
			return trace.BadParameter("unsupported protocol %q in allow_protocols, must be %q or %q",
				protocol, types.GitProtocolSSH, types.GitProtocolHTTPS)
		}
	}
	return nil
}

func validateAWSRolesAnywhereProfileFilters(ig types.Integration) error {
	rolesAnywhereSpec := ig.GetAWSRolesAnywhereIntegrationSpec()
	if rolesAnywhereSpec == nil {
		return nil
	}

	if rolesAnywhereSpec.ProfileSyncConfig == nil {
		return nil
	}

	for _, profileNameFilter := range rolesAnywhereSpec.ProfileSyncConfig.ProfileNameFilters {
		if _, err := utils.CompileExpression(profileNameFilter); err != nil {
			return trace.BadParameter("invalid filter %q, use glob-like matching or regex by adding the anchors (eg, ^regex$): %v", profileNameFilter, err)
		}
	}
	return nil
}

// DeleteIntegration removes the specified Integration resource.
//
// This RPC may remove multiple resources in the backend:
// - Associated resources like Git servers if DeleteAssociatedResources is set
// - Associated plugin credentials
// - The integration resource itself
//
// Note that there is no rollback if some error happens in the middle of the
// process.
func (s *Service) DeleteIntegration(ctx context.Context, req *integrationpb.DeleteIntegrationRequest) (*emptypb.Empty, error) {
	s.logger.DebugContext(ctx, "Deleting integration", "integration", req.GetName())

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := s.cache.GetIntegration(ctx, integrationpb.GetIntegrationRequest_builder{
		Name:  req.GetName(),
		Scope: req.GetScope(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We currently only allow deleting associated resources when the integration is unscoped.
	if req.GetDeleteAssociatedResources() && req.GetScope() == "" {
		if err := s.deleteAssociatedResources(ctx, authCtx, ig); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := s.ensureNoAssociatedResources(ctx, ig); err != nil {
		return nil, trace.Wrap(err)
	}

	s.logger.DebugContext(ctx, "Deleted integration", "integration", ig, "credentials", ig.GetCredentials())
	if err := s.removeStaticCredentials(ctx, ig); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteIntegration(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	if ig.IsOAuthProxy() {
		if err := s.deleteOAuthProxyAppServer(ctx, ig); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	igMeta, err := getIntegrationMetadata(ig)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to build all integration metadata for audit event.", "error", err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.IntegrationDelete{
		Metadata: apievents.Metadata{
			Type: events.IntegrationDeleteEvent,
			Code: events.IntegrationDeleteCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:  ig.GetName(),
			Scope: ig.GetScope(),
		},
		IntegrationMetadata: igMeta,
		ConnectionMetadata:  authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit integration delete event.", "error", err)
	}

	s.postWriteOp(ig)
	return &emptypb.Empty{}, nil
}

func getIntegrationMetadata(ig types.Integration) (apievents.IntegrationMetadata, error) {
	igMeta := apievents.IntegrationMetadata{
		SubKind: ig.GetSubKind(),
	}
	switch igMeta.SubKind {
	case types.IntegrationSubKindAWSOIDC:
		igMeta.AWSOIDC = &apievents.AWSOIDCIntegrationMetadata{
			RoleARN:     ig.GetAWSOIDCIntegrationSpec().RoleARN,
			IssuerS3URI: ig.GetAWSOIDCIntegrationSpec().IssuerS3URI,
		}
	case types.IntegrationSubKindAzureOIDC:
		igMeta.AzureOIDC = &apievents.AzureOIDCIntegrationMetadata{
			TenantID: ig.GetAzureOIDCIntegrationSpec().TenantID,
			ClientID: ig.GetAzureOIDCIntegrationSpec().ClientID,
		}
	case types.IntegrationSubKindGitHub:
		igMeta.GitHub = &apievents.GitHubIntegrationMetadata{
			Organization: ig.GetGitHubIntegrationSpec().Organization,
		}
	case types.IntegrationSubKindAWSRolesAnywhere:
		igMeta.AWSRA = &apievents.AWSRAIntegrationMetadata{
			TrustAnchorARN: ig.GetAWSRolesAnywhereIntegrationSpec().TrustAnchorARN,
		}
	case types.IntegrationSubKindOAuthProxy:
		// TODO(nixpig): Add metadata for OAuth Proxy integration audit events.
	default:
		return apievents.IntegrationMetadata{}, fmt.Errorf("unknown integration subkind: %s", igMeta.SubKind)
	}

	return igMeta, nil
}

// DeleteAllIntegrations removes all Integration resources.
// DEPRECATED: can't delete all integrations over gRPC.
func (s *Service) DeleteAllIntegrations(ctx context.Context, _ *integrationpb.DeleteAllIntegrationsRequest) (*emptypb.Empty, error) {
	return nil, trace.BadParameter("DeleteAllIntegrations is deprecated")
}

func (s *Service) ensureNoAssociatedResources(ctx context.Context, ig types.Integration) error {
	switch ig.GetSubKind() {
	case types.IntegrationSubKindGitHub:
		return trace.Wrap(s.ensureNoGitHubAssociatedResources(ctx, ig))
	default:
		// TODO support this check for other types.
		return nil
	}
}

func (s *Service) ensureNoGitHubAssociatedResources(ctx context.Context, ig types.Integration) error {
	s.logger.DebugContext(ctx, "Checking GitHub integration associated resources", "integration", ig.GetName())
	for server, err := range clientutils.Resources(ctx, s.backend.ListGitServers) {
		if err != nil {
			return trace.Wrap(err)
		}

		if server.GetGitHub() != nil && server.GetGitHub().Integration == ig.GetName() {
			return trace.BadParameter("git servers associated with integration %s must be deleted first", ig.GetName())
		}
	}

	return nil
}

// newOAuthProxyAppServer returns an app server for an OAuth Proxy integration.
func (s *Service) newOAuthProxyAppServer(ctx context.Context, ig types.Integration) (types.AppServer, error) {
	spec := ig.GetOAuthProxyIntegrationSpec()
	if spec == nil {
		return nil, trace.BadParameter("missing OAuth Proxy integration spec")
	}

	proxyPublicAddr := s.proxyPublicAddrGetter(ctx)
	if proxyPublicAddr == "" {
		return nil, trace.NotFound("no proxy public addr found")
	}
	appPublicAddr := utils.DefaultAppPublicAddr(ig.GetName(), proxyPublicAddr)

	hostID := oauthProxyHostID(ig.GetName())

	appServer, err := types.NewAppServerForOAuthProxyIntegration(ig, hostID, appPublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateAppServer(appServer, s.cache); err != nil {
		return nil, trace.Wrap(err)
	}

	return appServer, nil
}

// createOAuthProxyIntegration atomically creates the OAuth Proxy integration, credentials, and app server.
func (s *Service) createOAuthProxyIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	appServer, err := s.newOAuthProxyAppServer(ctx, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions, err := s.appendCreateOAuthProxyCredentialActions(nil, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions, err = s.backend.AppendPutIntegrationActions(actions, ig, backend.NotExists())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions, err = s.backend.AppendPutAppServerActions(actions, appServer, backend.NotExists())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	revision, err := s.storageBackend.AtomicWrite(ctx, actions)
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.AlreadyExists("integration could not be created due to naming conflict")
		}
		return nil, trace.Wrap(err)
	}

	ig.SetRevision(revision)

	return ig, nil
}

// updateOAuthProxyIntegration atomically updates the OAuth Proxy integration, credentials, and app server.
func (s *Service) updateOAuthProxyIntegration(ctx context.Context, newIg, oldIg types.Integration) (types.Integration, error) {
	appServer, err := s.newOAuthProxyAppServer(ctx, newIg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions, err := s.appendUpdateOAuthProxyCredentialActions(ctx, nil, newIg, oldIg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions, err = s.backend.AppendPutIntegrationActions(actions, newIg, backend.Revision(oldIg.GetRevision()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions, err = s.backend.AppendPutAppServerActions(actions, appServer, backend.Whatever())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	revision, err := s.storageBackend.AtomicWrite(ctx, actions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newIg.SetRevision(revision)

	return newIg, nil
}

// deleteOAuthProxyAppServer removes the app server for the specified OAuth Proxy integration.
func (s *Service) deleteOAuthProxyAppServer(ctx context.Context, ig types.Integration) error {
	if err := s.backend.DeleteAppServer(ctx, presencev1.DeleteAppServerRequest_builder{
		HostId: oauthProxyHostID(ig.GetName()),
		Name:   ig.GetName(),
	}.Build()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Service) deleteAssociatedResources(ctx context.Context, authCtx *authz.Context, ig types.Integration) error {
	switch ig.GetSubKind() {
	case types.IntegrationSubKindAWSOIDC:
		return trace.Wrap(s.deleteAWSOIDCAssociatedResources(ctx, authCtx, ig))
	case types.IntegrationSubKindGitHub:
		return trace.Wrap(s.deleteGitHubAssociatedResources(ctx, authCtx, ig))
	default:
		return trace.NotImplemented("DeleteAssociatedResources not supported for integration kind %q", ig.GetKind())
	}
}

func (s *Service) deleteGitHubAssociatedResources(ctx context.Context, authCtx *authz.Context, ig types.Integration) error {
	s.logger.DebugContext(ctx, "Deleting git servers associated with integration", "integration", ig.GetName())

	// This RPC only attempts to delete the git server (it's not returning the
	// git server for the caller to use), so check for types.VerbDelete and
	// types.VerbList but skip types.Read and authCtx.Checker.CheckAccess on the
	// resource.
	if err := authCtx.CheckAccessToKind(types.KindGitServer, types.VerbDelete, types.VerbList); err != nil {
		return trace.Wrap(err)
	}

	for server, err := range clientutils.Resources(ctx, s.backend.ListGitServers) {
		if err != nil {
			return trace.Wrap(err)
		}

		if server.GetGitHub() != nil && server.GetGitHub().Integration == ig.GetName() {
			return trace.Wrap(s.backend.DeleteGitServer(ctx, server.GetName()))
		}
	}

	return nil
}

func (s *Service) postWriteOp(ig types.Integration) {
	if ig.GetSubKind() == types.IntegrationSubKindGitHub {
		go s.notifyGitHubCallbackMigration()
	}
}
