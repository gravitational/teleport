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
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

// Cache is the subset of the cached resources that the Service queries.
type Cache interface {
	// GetClusterName returns local cluster name of the current auth server
	GetClusterName(ctx context.Context) (types.ClusterName, error)

	// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
	// controls if signing keys are loaded
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error)

	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)

	// IntegrationsGetter defines methods to access Integration resources.
	services.IntegrationsGetter

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
	services.Integrations
	services.PluginStaticCredentials
	services.GitServers
}

// ServiceConfig holds configuration options for
// the Integration gRPC service.
type ServiceConfig struct {
	Authorizer      authz.Authorizer
	Backend         Backend
	Cache           Cache
	KeyStoreManager KeyStoreManager
	Logger          *slog.Logger
	Clock           clockwork.Clock
	Emitter         apievents.Emitter

	// awsRolesAnywhereCreateSessionFn is a function that creates an AWS Roles Anywhere session.
	// This is used to allow mocking in tests, because the real implementation does
	// If not set, the default implementation is used.
	awsRolesAnywhereCreateSessionFn func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error)
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

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	if s.awsRolesAnywhereCreateSessionFn == nil {
		s.awsRolesAnywhereCreateSessionFn = createsession.CreateSession
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
}

// NewService returns a new Integrations gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		logger:          cfg.Logger,
		authorizer:      cfg.Authorizer,
		cache:           cfg.Cache,
		keyStoreManager: cfg.KeyStoreManager,
		backend:         cfg.Backend,
		clock:           cfg.Clock,
		emitter:         cfg.Emitter,

		awsRolesAnywhereCreateSessionFn: cfg.awsRolesAnywhereCreateSessionFn,
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

	results, nextKey, err := s.cache.ListIntegrations(ctx, int(req.GetLimit()), req.GetNextKey())
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

	return &integrationpb.ListIntegrationsResponse{
		Integrations: igs,
		NextKey:      nextKey,
	}, nil
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
	integration, err := s.cache.GetIntegration(ctx, req.GetName())
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

	switch req.Integration.GetSubKind() {
	case types.IntegrationSubKindGitHub:
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return nil, trace.AccessDenied("GitHub integration requires a Teleport Enterprise license")
		}
		if err := s.createGitHubCredentials(ctx, req.Integration); err != nil {
			return nil, trace.Wrap(err)
		}
	case types.IntegrationSubKindAWSOIDC, types.IntegrationSubKindAWSRolesAnywhere:
		// AWS OIDC and Roles Anywhere Integrations can be used as source of credentials to access AWS Web/CLI.
		// For OIDC, this creates a new AppServer whose endpoint is <integrationName>.<proxyURL>, which can fail if integrationName is not a valid DNS Label.
		// For Roles Anywhere, this creates a AppServers for each Roles Anywhere Profile whose endpoint is <profileName>-<integrationName>.<proxyURL>, which can fail if integrationName is not a valid DNS Label.
		// Instead of failing when the integration is already created, it fails at creation time.
		if errs := validation.IsDNS1035Label(req.GetIntegration().GetName()); len(errs) > 0 {
			return nil, trace.BadParameter("integration name %q must be a lower case valid DNS subdomain so that it can be used to allow Web/CLI access", req.GetIntegration().GetName())
		}
	}

	ig, err := s.backend.CreateIntegration(ctx, req.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
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

	if err := s.maybeUpdateStaticCredentials(ctx, req.Integration); err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := s.backend.UpdateIntegration(ctx, req.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
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

	return igV1, nil
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

	ig, err := s.cache.GetIntegration(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.DeleteAssociatedResources {
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

	if err := s.backend.DeleteIntegration(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
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
			Name: ig.GetName(),
		},
		IntegrationMetadata: igMeta,
		ConnectionMetadata:  authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit integration delete event.", "error", err)
	}

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
	return trace.Wrap(clientutils.IterateResources(ctx, s.backend.ListGitServers, func(server types.Server) error {
		if server.GetGitHub() != nil && server.GetGitHub().Integration == ig.GetName() {
			return trace.BadParameter("git servers associated with integration %s must be deleted first", ig.GetName())
		}
		return nil
	}))
}

func (s *Service) deleteAssociatedResources(ctx context.Context, authCtx *authz.Context, ig types.Integration) error {
	switch ig.GetSubKind() {
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

	return trace.Wrap(clientutils.IterateResources(ctx, s.backend.ListGitServers, func(server types.Server) error {
		if server.GetGitHub() == nil || server.GetGitHub().Integration != ig.GetName() {
			return nil
		}
		return trace.Wrap(s.backend.DeleteGitServer(ctx, server.GetName()))
	}))
}
