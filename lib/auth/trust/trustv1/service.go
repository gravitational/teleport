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

package trustv1

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TODO(codingllama): DELETE IN 20. All valid clients support WindowsCA by then.
var minGetUserCASemver = semver.New("18.7.0")

type authServer interface {
	// GetClusterName returns cluster name
	GetClusterName(ctx context.Context) (types.ClusterName, error)

	// GenerateHostCert uses the private key of the CA to sign the public key of
	// the host (along with metadata like host ID, node name, roles, and ttl)
	// to generate a host certificate.
	GenerateHostCert(ctx context.Context, hostPublicKey []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error)

	// RotateCertAuthority starts or restarts certificate authority rotation process.
	RotateCertAuthority(ctx context.Context, req types.RotateRequest) error

	// UpsertTrustedClusterV2 upserts a Trusted Cluster.
	UpsertTrustedClusterV2(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error)
	// CreateTrustedCluster creates a Trusted Cluster.
	CreateTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error)
	// UpdateTrustedCluster updates a Trusted Cluster.
	UpdateTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error)
	// ListTrustedClusters returns a page of Trusted Cluster resources.
	ListTrustedClusters(ctx context.Context, limit int, startKey string) ([]types.TrustedCluster, string, error)
}

// ServiceConfig holds configuration options for
// the trust gRPC service.
type ServiceConfig struct {
	Authorizer       authz.Authorizer
	ScopedAuthorizer authz.ScopedAuthorizer
	Cache            services.AuthorityGetter
	Backend          services.TrustInternal
	AuthServer       authServer
}

// Service implements the teleport.trust.v1.TrustService RPC service.
type Service struct {
	trustpb.UnimplementedTrustServiceServer
	authorizer       authz.Authorizer
	scopedAuthorizer authz.ScopedAuthorizer
	cache            services.AuthorityGetter
	backend          services.TrustInternal
	authServer       authServer
}

// NewService returns a new trust gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.ScopedAuthorizer == nil:
		return nil, trace.BadParameter("scoped authorizer is required")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("authServer is required")
	}

	return &Service{
		authorizer:       cfg.Authorizer,
		scopedAuthorizer: cfg.ScopedAuthorizer,
		cache:            cfg.Cache,
		backend:          cfg.Backend,
		authServer:       cfg.AuthServer,
	}, nil
}

// GetCertAuthority retrieves the matching certificate authority.
func (s *Service) GetCertAuthority(ctx context.Context, req *trustpb.GetCertAuthorityRequest) (*types.CertAuthorityV2, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	readVerb := types.VerbReadNoSecrets
	if req.IncludeKey {
		readVerb = types.VerbRead
	}

	// Before looking up the requested CA perform RBAC on a dummy CA to
	// determine if the user has access to CAs in general. This helps prevent
	// leaking information about the requested CA if the call to GetCertAuthority
	// fails.
	contextCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.CertAuthType(req.Type),
		ClusterName: req.Domain,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err = authCtx.CheckAccessToResource(contextCA, readVerb); err != nil {
		return nil, trace.Wrap(err)
	}

	// Require admin MFA to read secrets.
	if req.IncludeKey {
		if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Retrieve the requested CA and perform RBAC on it to ensure that
	// the user has access to this particular CA.
	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{Type: types.CertAuthType(req.Type), DomainName: req.Domain}, req.IncludeKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err = authCtx.CheckAccessToResource(ca, readVerb); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := failPreWindowsCATctlUserCAQuery(ctx, req.Type); err != nil {
		return nil, trace.Wrap(err)
	}
	authority, ok := ca.(*types.CertAuthorityV2)
	if !ok {
		return nil, trace.BadParameter("unexpected ca type %T", ca)
	}

	return authority, nil
}

// failPreWindowsCATctlUserCAQuery checks if the client requesting the UserCA is
// a version of tctl that does not know about the WindowsCA. Older Auth servers
// used the UserCA for Windows desktop sessions. Newer Auth servers use
// WindowsCA. If an old tctl is requesting the UserCA, we do not know if it is
// meant for Windows desktop purposes, so deny the request and require the user
// upgrade the tctl client so that the intent is clear.
// See RFD 0239 for details:
// https://github.com/gravitational/teleport/blob/master/rfd/0239-windows-ca-split.md
// TODO(codingllama): DELETE IN 20. All valid clients support WindowsCA by then.
func failPreWindowsCATctlUserCAQuery(ctx context.Context, caType string) error {
	// Query for UserCA?
	if caType != string(types.UserCA) {
		return nil
	}

	// Client metadata present?
	clientVersion, ok := metadata.ClientVersionFromContext(ctx)
	ua := metadata.UserAgentFromContext(ctx)
	if !ok || ua == "" {
		slog.WarnContext(ctx,
			"Client context lacks version or user agent. Unable to make GetCertAuthority compatibility decision, letting the request through.",
			"client_version", clientVersion,
			"user_agent", ua,
		)
		return nil
	}

	// Client is tctl?
	if !strings.HasPrefix(ua, teleport.ComponentTCTL+"/") {
		return nil
	}

	// tctl pre WindowsCA?
	clientSemver, err := semver.NewVersion(clientVersion)
	if err != nil {
		slog.WarnContext(ctx,
			"Client version invalid. Unable to make GetCertAuthority compatibility decision, letting the request through.",
			"error", err,
			"client_version", clientVersion,
		)
		return nil
	}
	if clientSemver.Compare(*minGetUserCASemver) >= 0 {
		return nil
	}

	return trace.BadParameter(
		"in order to query the User CA, tctl must be upgraded to version %v or newer", minGetUserCASemver)
}

// GetCertAuthorities retrieves the cert authorities with the specified type.
func (s *Service) GetCertAuthorities(ctx context.Context, req *trustpb.GetCertAuthoritiesRequest) (*trustpb.GetCertAuthoritiesResponse, error) {
	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verbs := []string{types.VerbList, types.VerbReadNoSecrets}
	// for standard reads we do not enforce scope pinning. this ensures that CAs are readable for
	// all scoped identities regardless of their current scope pinning. this pattern should not
	// be used for any checks save essential global configuration reads that are necessary for basic
	// teleport functionality.
	decisionFn := authCtx.CheckerContext.RiskyUnpinnedDecision

	if req.IncludeKey {
		verbs = append(verbs, types.VerbRead)

		// for queries that include secrets we must enforce standard scope pinning rules.
		// NOTE: technically we have no plans to introduce scoped CA secrets. as of the time of writing,
		// attempts to read CA secrets by scoped identities are always denied by virtue of the scoped
		// role verb limits enforced in scopes/access. this, however, would be the correct pattern should
		// we choose to introduce scoped CA secrets in the future.
		decisionFn = authCtx.CheckerContext.Decision

		// Require admin MFA to read secrets (admin MFA is currently only supported for unscoped identities).
		if unscopedCtx, ok := authCtx.UnscopedContext(); ok {
			if err := unscopedCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
				return nil, trace.Wrap(err)
			}
		} else {
			return nil, trace.AccessDenied("cannot perform admin action %s:%s as scoped identity", types.KindCertAuthority, types.VerbRead)
		}
	}

	// build rule context for where-clause evaluation once to avoid re-creation
	// on each decision invocation.
	ruleCtx := authCtx.RuleContext()

	// perform access-control decision. all cert authorities can be considered as being "root" resources from
	// the perspective of scoped RBAC, so we just hard-code root as the resource scope for the decision.
	if err := decisionFn(ctx, scopes.Root, func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, types.KindCertAuthority, verbs...)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	cas, err := s.cache.GetCertAuthorities(ctx, types.CertAuthType(req.Type), req.IncludeKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &trustpb.GetCertAuthoritiesResponse{CertAuthoritiesV2: make([]*types.CertAuthorityV2, 0, len(cas))}

	for _, ca := range cas {
		cav2, ok := ca.(*types.CertAuthorityV2)
		if !ok {
			return nil, trace.BadParameter("cert authority has invalid type %T", ca)
		}

		resp.CertAuthoritiesV2 = append(resp.CertAuthoritiesV2, cav2)
	}

	return resp, nil
}

// DeleteCertAuthority deletes the matching cert authority.
func (s *Service) DeleteCertAuthority(ctx context.Context, req *trustpb.DeleteCertAuthorityRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCertAuthority, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteCertAuthority(ctx, types.CertAuthID{DomainName: req.Domain, Type: types.CertAuthType(req.Type)}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// UpsertCertAuthority creates or updates the provided cert authority.
func (s *Service) UpsertCertAuthority(ctx context.Context, req *trustpb.UpsertCertAuthorityRequest) (*types.CertAuthorityV2, error) {
	if req.CertAuthority == nil {
		return nil, trace.BadParameter("missing certificate authority")
	}

	if err := services.ValidateCertAuthority(req.CertAuthority); err != nil {
		return nil, trace.Wrap(err)
	}

	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToResource(req.CertAuthority, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.UpsertCertAuthority(ctx, req.CertAuthority); err != nil {
		return nil, trace.Wrap(err)
	}

	return req.CertAuthority, nil
}

// RotateCertAuthority rotates a cert authority.
func (s *Service) RotateCertAuthority(ctx context.Context, req *trustpb.RotateCertAuthorityRequest) (*trustpb.RotateCertAuthorityResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCertAuthority, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	rotateRequest := types.RotateRequest{
		Type:        types.CertAuthType(req.Type),
		TargetPhase: req.TargetPhase,
		Mode:        req.Mode,
	}

	if req.GracePeriod != nil {
		duration := req.GracePeriod.AsDuration()
		rotateRequest.GracePeriod = &duration
	}

	if req.Schedule != nil {
		rotateRequest.Schedule = &types.RotationSchedule{
			UpdateClients: req.Schedule.UpdateClients.AsTime(),
			UpdateServers: req.Schedule.UpdateServers.AsTime(),
			Standby:       req.Schedule.Standby.AsTime(),
		}
	}

	if err := s.authServer.RotateCertAuthority(ctx, rotateRequest); err != nil {
		return nil, trace.Wrap(err)
	}

	return &trustpb.RotateCertAuthorityResponse{}, nil
}

// RotateExternalCertAuthority rotates external certificate authority,
// this method is called by remote trusted cluster and is used to update
// only public keys and certificates of the certificate authority.
func (s *Service) RotateExternalCertAuthority(ctx context.Context, req *trustpb.RotateExternalCertAuthorityRequest) (*trustpb.RotateExternalCertAuthorityResponse, error) {
	if req.CertAuthority == nil {
		return nil, trace.BadParameter("missing certificate authority")
	}

	if err := services.ValidateCertAuthority(req.CertAuthority); err != nil {
		return nil, trace.Wrap(err)
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToResource(req.CertAuthority, types.VerbRotate); err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalOrRemoteService(*authCtx) {
		return nil, trace.AccessDenied("this request can be only executed by an internal Teleport service")
	}

	// ensure that the caller is rotating a CA from the same cluster
	caClusterName := req.CertAuthority.GetClusterName()
	if caClusterName != authCtx.Identity.GetIdentity().TeleportCluster {
		return nil, trace.BadParameter("can not rotate local certificate authority")
	}
	// ensure the subjects and issuers of the CA certs match what the
	// cluster name of this CA is supposed to be
	for _, keyPair := range req.CertAuthority.GetTrustedTLSKeyPairs() {
		cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certClusterName, err := tlsca.ClusterName(cert.Subject)
		if err != nil {
			return nil, trace.AccessDenied("CA certificate subject organization is invalid")
		}
		if certClusterName != caClusterName {
			return nil, trace.AccessDenied("the subject organization of a CA certificate does not match the cluster name of the CA")
		}
	}

	clusterName, err := s.authServer.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// this is just an extra precaution against local admins,
	// because this is additionally enforced by RBAC as well
	if req.CertAuthority.GetClusterName() == clusterName.GetClusterName() {
		return nil, trace.BadParameter("can not rotate local certificate authority")
	}

	existing, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       req.CertAuthority.GetType(),
		DomainName: req.CertAuthority.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated := existing.Clone()
	if err := updated.SetActiveKeys(req.CertAuthority.GetActiveKeys().Clone()); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := updated.SetAdditionalTrustedKeys(req.CertAuthority.GetAdditionalTrustedKeys().Clone()); err != nil {
		return nil, trace.Wrap(err)
	}

	// a rotation state of "" gets stored as "standby" after
	// CheckAndSetDefaults, so if `ca` came in with a zeroed rotation we must do
	// this before checking if `updated` is the same as `existing` or the check
	// will fail for no reason (CheckAndSetDefaults is idempotent, so it's fine
	// to call it both here and in CompareAndSwapCertAuthority)
	updated.SetRotation(req.CertAuthority.GetRotation())
	if err := services.CheckAndSetDefaults(updated); err != nil {
		return nil, trace.Wrap(err)
	}

	// CASing `updated` over `existing` if they're equivalent will only cause
	// backend and watcher spam for no gain, so we exit early if that's the case
	if existing.IsEqual(updated) {
		return &trustpb.RotateExternalCertAuthorityResponse{}, nil
	}

	// use compare and swap to protect from concurrent updates
	// by trusted cluster API
	if _, err := s.backend.UpdateCertAuthority(ctx, updated); err != nil {
		return nil, trace.Wrap(err)
	}

	return &trustpb.RotateExternalCertAuthorityResponse{}, nil
}

// GenerateHostCert takes a public key in the OpenSSH `authorized_keys` format
// and returns a SSH certificate signed by the Host CA.
func (s *Service) GenerateHostCert(
	ctx context.Context, req *trustpb.GenerateHostCertRequest,
) (*trustpb.GenerateHostCertResponse, error) {
	// Perform special authz as we allow for `where` rules on the `host_cert`
	// resource type.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ruleCtx := &services.Context{
		User: authCtx.User,
		HostCert: &services.HostCertContext{
			HostID:      req.HostId,
			NodeName:    req.NodeName,
			Principals:  req.Principals,
			ClusterName: req.ClusterName,
			Role:        types.SystemRole(req.Role),
			TTL:         req.Ttl.AsDuration(),
		},
	}
	if err = authCtx.CheckAccessToRule(
		ruleCtx,
		types.KindHostCert,
		types.VerbCreate,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Call through to the underlying implementation on auth.Server. At some
	// point in the future, we may wish to pull more of that implementation
	// up to here.
	cert, err := s.authServer.GenerateHostCert(
		ctx,
		req.Key,
		req.HostId,
		req.NodeName,
		req.Principals,
		req.ClusterName,
		types.SystemRole(req.Role),
		req.Ttl.AsDuration(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &trustpb.GenerateHostCertResponse{
		SshCertificate: cert,
	}, nil
}
