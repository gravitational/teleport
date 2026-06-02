// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package workloadidentityv1

import (
	"cmp"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/url"
	"strings"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	apiproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	traitv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trait/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	apiworkloadidentity "github.com/gravitational/teleport/api/workloadidentity"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1")

// KeyStorer is an interface that provides methods to retrieve keys and
// certificates from the backend.
type KeyStorer interface {
	GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error)
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
}

// SigstorePolicyEvaluator implements the actual Sigstore verification logic.
type SigstorePolicyEvaluator interface {
	// Evaluate the Sigstore policies against the given workload attributes.
	Evaluate(ctx context.Context, policyNames []string, attrs *workloadidentityv1pb.Attrs) (map[string]error, error)
}

// OSSSigstorePolicyEvaluator is the Community Edition implementation of the
// SigstorePolicyEvaluator interface. It simply returns a licensing error if
// any policy names or sigstore payloads are given.
type OSSSigstorePolicyEvaluator struct{}

// Evaluate satisfies the SigstorePolicyEvaluator interface.
func (OSSSigstorePolicyEvaluator) Evaluate(_ context.Context, policyNames []string, attrs *workloadidentityv1pb.Attrs) (map[string]error, error) {
	if len(policyNames) != 0 || len(attrs.GetWorkload().GetSigstore().GetPayloads()) != 0 {
		return nil, trace.AccessDenied("Sigstore workload attestation is only available with an enterprise license")
	}
	return make(map[string]error), nil
}

type issuerCache interface {
	workloadIdentityCache
	// Deprecated: Prefer paginated variant [ListProxyServers].
	//
	// TODO(kiosion): DELETE IN 21.0.0
	GetProxies() ([]types.Server, error)
	ListProxyServers(context.Context, int, string) ([]types.Server, string, error)
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	ListResources(ctx context.Context, req apiproto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// IssuanceServiceConfig holds configuration options for the IssuanceService.
type IssuanceServiceConfig struct {
	Authorizer                 authz.Authorizer
	ScopedAuthorizer           authz.ScopedAuthorizer
	Cache                      issuerCache
	Clock                      clockwork.Clock
	Emitter                    apievents.Emitter
	Logger                     *slog.Logger
	KeyStore                   KeyStorer
	OverrideGetter             services.WorkloadIdentityX509CAOverrideGetter
	GetSigstorePolicyEvaluator func() SigstorePolicyEvaluator

	ClusterName string
}

// IssuanceService is the gRPC service for managing workload identity resources.
// It implements the workloadidentityv1pb.WorkloadIdentityIssuanceServiceServer.
type IssuanceService struct {
	workloadidentityv1pb.UnimplementedWorkloadIdentityIssuanceServiceServer

	authorizer                 authz.Authorizer
	scopedAuthorizer           authz.ScopedAuthorizer
	cache                      issuerCache
	clock                      clockwork.Clock
	emitter                    apievents.Emitter
	logger                     *slog.Logger
	keyStore                   KeyStorer
	overrideGetter             services.WorkloadIdentityX509CAOverrideGetter
	getSigstorePolicyEvaluator func() SigstorePolicyEvaluator

	clusterName string
}

// NewIssuanceService returns a new instance of the IssuanceService.
func NewIssuanceService(cfg *IssuanceServiceConfig) (*IssuanceService, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.ScopedAuthorizer == nil:
		return nil, trace.BadParameter("scoped authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.KeyStore == nil:
		return nil, trace.BadParameter("key store is required")
	case cfg.OverrideGetter == nil:
		return nil, trace.BadParameter("override getter is required")
	case cfg.ClusterName == "":
		return nil, trace.BadParameter("cluster name is required")
	case cfg.GetSigstorePolicyEvaluator == nil:
		return nil, trace.BadParameter("sigstore policy evaluator is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "workload_identity_issuance.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &IssuanceService{
		authorizer:                 cfg.Authorizer,
		scopedAuthorizer:           cfg.ScopedAuthorizer,
		cache:                      cfg.Cache,
		clock:                      cfg.Clock,
		emitter:                    cfg.Emitter,
		logger:                     cfg.Logger,
		keyStore:                   cfg.KeyStore,
		overrideGetter:             cfg.OverrideGetter,
		getSigstorePolicyEvaluator: cfg.GetSigstorePolicyEvaluator,

		clusterName: cfg.ClusterName,
	}, nil
}

func (s *IssuanceService) deriveAttrs(
	identity authz.IdentityGetter,
	user types.User,
	workloadAttrs *workloadidentityv1pb.WorkloadAttrs,
) (*workloadidentityv1pb.Attrs, error) {
	attrs := &workloadidentityv1pb.Attrs{
		Workload: workloadAttrs,
		User: &workloadidentityv1pb.UserAttrs{
			Name:    identity.GetIdentity().Username,
			IsBot:   identity.GetIdentity().BotName != "",
			BotName: identity.GetIdentity().BotName,
			Labels:  user.GetAllLabels(),
		},
		Join: identity.GetIdentity().JoinAttributes,
	}

	for key, values := range identity.GetIdentity().Traits {
		attrs.User.Traits = append(attrs.User.Traits, &traitv1.Trait{
			Key:    key,
			Values: values,
		})
	}

	return attrs, nil
}

const (
	// defaultMaxTTL defines the max requestable TTL for SVIDs where the
	// workload identity resource does not specify a maximum TTL.
	defaultMaxTTL = 24 * time.Hour
	// defaultTTL defines the TTL when a client has not requested a specific
	// TTL.
	defaultTTL = 1 * time.Hour
)

func (s *IssuanceService) IssueWorkloadIdentity(
	ctx context.Context,
	req *workloadidentityv1pb.IssueWorkloadIdentityRequest,
) (*workloadidentityv1pb.IssueWorkloadIdentityResponse, error) {
	switch {
	case req.GetName() == "":
		return nil, trace.BadParameter("name: is required")
	case req.GetCredential() == nil:
		return nil, trace.BadParameter("at least one credential type must be requested")
	}

	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attrs, err := s.deriveAttrs(authCtx.Identity, authCtx.User, req.GetWorkloadAttrs())
	if err != nil {
		return nil, trace.Wrap(err, "deriving attributes")
	}

	wi, err := s.cache.GetWorkloadIdentity(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Check the caller is authorized to issue using this WorkloadIdentity: in the
	// resource's scope they must hold the read rule for the workload_identity kind
	// AND match it via a workload_identity label selector.
	if err := s.authorizeIssuance(ctx, authCtx, wi, types.VerbReadNoSecrets); err != nil {
		return nil, trace.Wrap(err)
	}

	decision := decide(ctx, wi, attrs, s.getSigstorePolicyEvaluator())
	if !decision.shouldIssue {
		return nil, trace.Wrap(decision.reason, "workload identity failed evaluation")
	}
	// Defense-in-depth: ensure templating did not produce a SPIFFE ID that
	// escapes the WorkloadIdentity's scope.
	if err := validateRenderedSpiffeIDScope(decision.templatedWorkloadIdentity); err != nil {
		return nil, trace.Wrap(err)
	}

	var cred *workloadidentityv1pb.Credential
	switch v := req.GetCredential().(type) {
	case *workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams:
		ca, chain, err := s.getX509CA(ctx, types.SPIFFECA, v.X509SvidParams.GetUseIssuerOverrides())
		if err != nil {
			return nil, trace.Wrap(err, "fetching X509 SPIFFE CA")
		}
		cred, err = s.issueX509SVID(
			ctx,
			issueX509SVIDParams{
				ca:                    ca,
				chain:                 chain,
				workloadIdentity:      decision.templatedWorkloadIdentity,
				x509Params:            v.X509SvidParams,
				requestedTTL:          req.RequestedTtl.AsDuration(),
				attrs:                 attrs,
				sigstorePolicyResults: decision.sigstorePolicyResults,
				nameSelector:          req.GetName(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err, "issuing X509 SVID")
		}
	case *workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams:
		key, issuer, err := s.getJWTIssuerKey(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "getting JWT issuer key")
		}
		cred, err = s.issueJWTSVID(
			ctx,
			issueJWTSVIDParams{
				issuerKey:             key,
				issuerURI:             issuer,
				workloadIdentity:      decision.templatedWorkloadIdentity,
				jwtParams:             v.JwtSvidParams,
				requestedTTL:          req.RequestedTtl.AsDuration(),
				attrs:                 attrs,
				sigstorePolicyResults: decision.sigstorePolicyResults,
				nameSelector:          req.GetName(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err, "issuing JWT SVID")
		}
	default:
		return nil, trace.BadParameter("credential: unknown type %T", req.GetCredential())
	}

	return &workloadidentityv1pb.IssueWorkloadIdentityResponse{
		Credential: cred,
	}, nil
}

// maxWorkloadIdentitiesIssued is the maximum number of workload identities that
// can be issued in a single request.
// TODO(noah): We'll want to make this tunable via env var or similar to make
// sure we can adjust it as needed.
var maxWorkloadIdentitiesIssued = 10

func (s *IssuanceService) IssueWorkloadIdentities(
	ctx context.Context,
	req *workloadidentityv1pb.IssueWorkloadIdentitiesRequest,
) (*workloadidentityv1pb.IssueWorkloadIdentitiesResponse, error) {
	switch {
	case len(req.LabelSelectors) == 0:
		return nil, trace.BadParameter("label_selectors: at least one label selector must be specified")
	case req.GetCredential() == nil:
		return nil, trace.BadParameter("at least one credential type must be requested")
	}

	authCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Cheap feasibility check before iterating cluster state: can the caller list
	// WorkloadIdentities at all (in any scope)?
	ruleCtx := authCtx.RuleContext()
	if err := authCtx.CheckerContext.CheckMaybeHasAccessToRules(
		&ruleCtx, types.KindWorkloadIdentity, types.VerbList,
	); err != nil {
		return nil, trace.Wrap(err)
	}
	attrs, err := s.deriveAttrs(authCtx.Identity, authCtx.User, req.GetWorkloadAttrs())
	if err != nil {
		return nil, trace.Wrap(err, "deriving attributes")
	}

	// Iterate all WorkloadIdentities, keeping those that (a) match the requested
	// label selectors, (b) the caller is authorized to issue using (read+list
	// rules AND a label-selector grant, scoped to the resource's scope), and (c)
	// pass rule/template evaluation.
	labels := convertLabels(req.LabelSelectors)
	shouldIssue := []*workloadidentityv1pb.WorkloadIdentity{}
	for wi, err := range s.cache.RangeWorkloadIdentities(ctx, "", "", "", false) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Filter to the caller's requested label selectors.
		match, _, err := services.MatchLabelGetter(
			labels, types.Resource153ToResourceWithLabels(wi),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !match {
			continue
		}

		// Silently skip WorkloadIdentities the caller is not authorized to issue
		// using.
		if err := s.authorizeIssuance(ctx, authCtx, wi, types.VerbReadNoSecrets, types.VerbList); err != nil {
			continue
		}

		decision := decide(ctx, wi, attrs, s.getSigstorePolicyEvaluator())
		if !decision.shouldIssue {
			continue
		}
		// Defense-in-depth: ensure templating did not produce a SPIFFE ID that
		// escapes the WorkloadIdentity's scope.
		if err := validateRenderedSpiffeIDScope(decision.templatedWorkloadIdentity); err != nil {
			return nil, trace.Wrap(err)
		}
		shouldIssue = append(shouldIssue, decision.templatedWorkloadIdentity)
		if len(shouldIssue) > maxWorkloadIdentitiesIssued {
			// If we're now above the limit, then we want to exit out...
			return nil, trace.BadParameter(
				"number of identities that would be issued exceeds maximum permitted (max = %d), use more specific labels",
				maxWorkloadIdentitiesIssued,
			)
		}
	}

	creds := make([]*workloadidentityv1pb.Credential, 0, len(shouldIssue))
	switch v := req.GetCredential().(type) {
	case *workloadidentityv1pb.IssueWorkloadIdentitiesRequest_X509SvidParams:
		ca, chain, err := s.getX509CA(ctx, types.SPIFFECA, v.X509SvidParams.GetUseIssuerOverrides())
		if err != nil {
			return nil, trace.Wrap(err, "fetching CA to sign X509 SVID")
		}
		for _, wi := range shouldIssue {
			cred, err := s.issueX509SVID(
				ctx,
				issueX509SVIDParams{
					ca:               ca,
					chain:            chain,
					workloadIdentity: wi,
					x509Params:       v.X509SvidParams,
					requestedTTL:     req.RequestedTtl.AsDuration(),
					attrs:            attrs,
					labelSelectors:   req.LabelSelectors,
				},
			)
			if err != nil {
				return nil, trace.Wrap(
					err,
					"issuing X509 SVID for workload identity %q",
					wi.GetMetadata().GetName(),
				)
			}
			creds = append(creds, cred)
		}
	case *workloadidentityv1pb.IssueWorkloadIdentitiesRequest_JwtSvidParams:
		key, issuer, err := s.getJWTIssuerKey(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "getting JWT issuer key")
		}
		for _, wi := range shouldIssue {
			cred, err := s.issueJWTSVID(
				ctx,
				issueJWTSVIDParams{
					issuerKey:        key,
					issuerURI:        issuer,
					workloadIdentity: wi,
					jwtParams:        v.JwtSvidParams,
					requestedTTL:     req.RequestedTtl.AsDuration(),
					attrs:            attrs,
					labelSelectors:   req.LabelSelectors,
				},
			)
			if err != nil {
				return nil, trace.Wrap(
					err,
					"issuing JWT SVID for workload identity %q",
					wi.GetMetadata().GetName(),
				)
			}
			creds = append(creds, cred)
		}
	default:
		return nil, trace.BadParameter("credential: unknown type %T", req.GetCredential())
	}

	return &workloadidentityv1pb.IssueWorkloadIdentitiesResponse{
		Credentials: creds,
	}, nil
}

// IssueTeleportWorkloadIdentity issues a workload identity credential for the
// requested Teleport usage. This request cannot be performed by users, only
// by Teleport services.
//
// Optional future work: define the audit semantics for the workload identity
// issued by this RPC. This path currently does not emit per-issuance audit
// events because the app-access usage issues short-lived credentials
// periodically, up to once every 5 minutes per app session.
//
// If explicit audit is needed here, prefer a dedicated lower-volume Teleport
// workload event over reusing SPIFFESVIDIssued, and review event frequency
// before enabling it.
func (s *IssuanceService) IssueTeleportWorkloadIdentity(
	ctx context.Context,
	req *workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest,
) (*workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	builtin, ok := authCtx.Identity.(authz.BuiltinRole)
	if !ok {
		return nil, trace.AccessDenied("only Teleport services can execute this request")
	}

	switch usage := req.Usage.(type) {
	case *workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest_AppAccess:
		if !authz.HasBuiltinRole(*authCtx, string(types.RoleApp)) {
			return nil, trace.AccessDenied("only app services can issue workload identity for app access")
		}
		switch credParams := req.Credential.(type) {
		case *workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest_X509SvidParams:
			return s.issueAppAccessX509Identity(ctx, builtin.GetServerID(), req, usage.AppAccess, credParams)
		default:
			return nil, trace.BadParameter("app access usage only supports issuing x509 credentials")
		}
	default:
		return nil, trace.BadParameter("invalid identity usage")
	}
}

func (s *IssuanceService) issueAppAccessX509Identity(
	ctx context.Context,
	hostID string,
	req *workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest,
	appUsage *workloadidentityv1pb.AppAccessUsage,
	credParams *workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest_X509SvidParams,
) (_ *workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/issueAppAccessX509Identity")
	defer func() { tracing.EndSpan(span, err) }()

	route, userIdentity, err := s.routeToAppFromCert(ctx, appUsage.GetUserCertificate())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We cannot rely on RouteToApp.Name because:
	//   1. It might not be available.
	//   2. The value is not guarateed to be from the correct app, and in some
	//      flows requestors can provide arbitrary values.
	app, err := s.getApp(ctx, hostID, route)
	if err != nil {
		return nil, trace.Wrap(err, "unable to locate app")
	}

	pubKey, err := x509.ParsePKIXPublicKey(credParams.X509SvidParams.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err, "parsing public key")
	}

	// AppClient CA doesn't support workload identity CA override.
	ca, chain, err := s.getX509CA(ctx, types.AppClientCA, false /* useIssuerOverrides */)
	if err != nil {
		return nil, trace.Wrap(err, "fetching X509 SPIFFE CA")
	}

	_, notBefore, notAfter, ttl := calculateTTL(
		ctx,
		s.logger,
		s.clock,
		req.RequestedTtl.AsDuration(),
		// Cap the cert TTL at the session expiry, plus an alloweance for clock
		// drift that mirrors [veirfyCertValidityWithSkew]. Without the
		// allowance session that are within the skew window would yield app
		// certs with a near-zero (or negative) TTL.
		s.clock.Until(userIdentity.Expires)+certVerifyClockSkewAllowance,
	)

	// This intentionally uses cluster name as part of the the SPIFFE trust
	// domain, matching regular workload identity issuance behavior. Clusters
	// with names that are not valid SPIFFE trust domain are expected to fail
	// issuance.
	td, err := spiffeid.TrustDomainFromString(
		apiworkloadidentity.NewInternalAppTrustDomain(s.clusterName),
	)
	if err != nil {
		return nil, trace.Wrap(err, "cluster name cannot be used as SPIFFE ID trust domain")
	}

	spiffeID, err := spiffeid.FromSegments(td, "app", app.GetName())
	if err != nil {
		return nil, trace.Wrap(err, "app name contains invalid format and cannot be used as SPIFFE ID")
	}

	certSerial, err := generateCertSerial()
	if err != nil {
		return nil, trace.Wrap(err, "generating certificate serial")
	}

	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		x509Template(
			certSerial,
			notBefore,
			notAfter,
			spiffeID,
			nil, /* dnsSANs */
			&workloadidentityv1pb.X509DistinguishedNameTemplate{
				CommonName: userIdentity.Username,
			},
		),
		ca.Cert,
		pubKey,
		ca.Signer,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse{
		Credential: &workloadidentityv1pb.Credential{
			SpiffeId: spiffeID.String(),

			ExpiresAt: timestamppb.New(notAfter),
			Ttl:       durationpb.New(ttl),

			Credential: &workloadidentityv1pb.Credential_X509Svid{
				X509Svid: &workloadidentityv1pb.X509SVIDCredential{
					Cert:         certBytes,
					SerialNumber: serialString(certSerial),
					Chain:        chain,
				},
			},
		},
	}, nil
}

// routeToAppFromCert validates the certificate and extracts the RouteToApp info.
//
// We intentionally validate only the user certificate signature and expiration,
// and app routing metadata here. The app service is expected to call this RPC
// only after it has already authenticated the app request and verified that the
// referenced AppSession still exists.
//
// Because it won't look up the AppSession, callers must not use this as the
// AppSession validity check.
func (s *IssuanceService) routeToAppFromCert(ctx context.Context, rawCert []byte) (tlsca.RouteToApp, *tlsca.Identity, error) {
	cert, err := x509.ParseCertificate(rawCert)
	if err != nil {
		return tlsca.RouteToApp{}, nil, trace.Wrap(err)
	}

	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: s.clusterName,
	}, false)
	if err != nil {
		return tlsca.RouteToApp{}, nil, trace.Wrap(err)
	}

	roots := x509.NewCertPool()
	for _, key := range ca.GetTrustedTLSKeyPairs() {
		if ok := roots.AppendCertsFromPEM(key.Cert); !ok {
			return tlsca.RouteToApp{}, nil, trace.BadParameter("unable to build UserCA pool to validate certificate")
		}
	}

	// cert.Verify applies CurrentTime to every certificate in the chain and
	// does not support separate NotBefore/NotAfter leeway for the leaf
	// certificate.
	//
	// For this reason we perform the certificate verify in two steps:
	//   1. Verify certificate validity using custom clock-skew policy.
	//   2. Perform the remaining verification on the cert using a shallow copy
	//      of the certificate that will pass the expiry verification.
	//
	// The shallow copy usage still checks the original TBSCertificate bytes,
	// the only effective difference is the Verify's leaf time check.
	now := s.clock.Now()
	if err := verifyCertValidityWithSkew(cert, now); err != nil {
		return tlsca.RouteToApp{}, nil, trace.Wrap(err)
	}

	verifyCert := *cert
	verifyCert.NotAfter = now.Add(time.Second)
	verifyCert.NotBefore = now.Add(-time.Second)

	if _, err := verifyCert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: now,
		KeyUsages: []x509.ExtKeyUsage{
			// Extensions added by tlsca.
			// See https://github.com/gravitational/teleport/blob/master/lib/tlsca/ca.go
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}); err != nil {
		return tlsca.RouteToApp{}, nil, trace.Wrap(err, "requestor provided an invalid certificate")
	}

	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return tlsca.RouteToApp{}, nil, trace.Wrap(err, "requestor provided a certificate that doesn't contain a Teleport identity")
	}

	route, err := identity.GetRouteToApp()
	if err != nil {
		return tlsca.RouteToApp{}, nil, trace.Wrap(err, "identity must be from an app access session")
	}

	// Ensure the required information is available on the identity.
	//
	// This acts as a guardrail for any inconsistent or malformed identity and
	// shouldn't happen. This validation just prevents generating workload
	// identity that is inconsistent as well.
	if route.ClusterName != s.clusterName {
		return tlsca.RouteToApp{}, nil, trace.BadParameter("cannot request workload identity for a different cluster")
	}

	return route, identity, nil
}

// getApp retrieves the app definition from an AppServer registered by the
// requesting host. Requiring the requestor (identified by hostID) to be one
// of the app's serving AppServers ensures the app definition we sign comes from
// a service authoritative for the app, not from an arbitrary cached entry.
//
// This is not a strong authorization check: any app service that advertises the
// target app can satisfy this lookup, even if it is not actually serving the
// specific session referenced by the app session certificate.
func (s *IssuanceService) getApp(ctx context.Context, hostID string, route tlsca.RouteToApp) (types.Application, error) {
	appServersIter := clientutils.Resources(ctx, func(ctx context.Context, limit int, startKey string) ([]types.ResourceWithLabels, string, error) {
		resp, err := s.cache.ListResources(ctx, apiproto.ListResourcesRequest{
			Namespace:           apidefaults.Namespace,
			ResourceType:        types.KindAppServer,
			Limit:               int32(limit),
			StartKey:            startKey,
			PredicateExpression: fmt.Sprintf(`resource.spec.public_addr == %q`, route.PublicAddr),
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return resp.Resources, resp.NextKey, nil
	})

	for resource, err := range appServersIter {
		if err != nil {
			return nil, trace.Wrap(err, "unabled to complete app servers list")
		}

		appServer, ok := resource.(types.AppServer)
		if !ok {
			return nil, trace.BadParameter("expected types.AppServer, got: %T", resource)
		}

		// Ensure the requesting app service can serve the app.
		if appServer.GetHostID() == hostID {
			return appServer.GetApp(), nil
		}
	}

	return nil, trace.NotFound("application at %q not found for app service %q", route.PublicAddr, hostID)
}

func generateCertSerial() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}

func x509Template(
	serialNumber *big.Int,
	notBefore time.Time,
	notAfter time.Time,
	spiffeID spiffeid.ID,
	dnsSANs []string,
	subjectTemplate *workloadidentityv1pb.X509DistinguishedNameTemplate,
) *x509.Certificate {
	c := &x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		// SPEC(X509-SVID) 4.3. Key Usage:
		// - Leaf SVIDs MUST NOT set keyCertSign or cRLSign.
		// - Leaf SVIDs MUST set digitalSignature
		// - They MAY set keyEncipherment and/or keyAgreement;
		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment |
			x509.KeyUsageKeyAgreement,
		// SPEC(X509-SVID) 4.4. Extended Key Usage:
		// - Leaf SVIDs SHOULD include this extension, and it MAY be marked as critical.
		// - When included, fields id-kp-serverAuth and id-kp-clientAuth MUST be set.
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
		},
		// SPEC(X509-SVID) 4.1. Basic Constraints:
		// - leaf certificates MUST set the cA field to false
		BasicConstraintsValid: true,
		IsCA:                  false,

		// SPEC(X509-SVID) 2. SPIFFE ID:
		// - The corresponding SPIFFE ID is set as a URI type in the Subject Alternative Name extension
		// - An X.509 SVID MUST contain exactly one URI SAN, and by extension, exactly one SPIFFE ID.
		// - An X.509 SVID MAY contain any number of other SAN field types, including DNS SANs.
		URIs:     []*url.URL{spiffeID.URL()},
		DNSNames: dnsSANs,
	}
	if subjectTemplate != nil {
		c.Subject.CommonName = subjectTemplate.CommonName
		if subjectTemplate.Organization != "" {
			c.Subject.Organization = []string{
				subjectTemplate.Organization,
			}
		}
		if subjectTemplate.OrganizationalUnit != "" {
			c.Subject.OrganizationalUnit = []string{subjectTemplate.OrganizationalUnit}
		}
	}

	return c
}

func (s *IssuanceService) getX509CA(
	ctx context.Context,
	caType types.CertAuthType,
	useIssuerOverrides bool,
) (_ *tlsca.CertAuthority, _ [][]byte, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/getX509CA")
	defer func() { tracing.EndSpan(span, err) }()

	const loadKeysTrue = true
	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: s.clusterName,
	}, loadKeysTrue)

	tlsCert, tlsSigner, err := s.keyStore.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, nil, trace.Wrap(err, "getting CA cert and key")
	}
	tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if !useIssuerOverrides {
		return tlsCA, nil, nil
	}
	// TODO(espadolini): support alternate overrides depending on the trust
	// domain, once that's fleshed out
	newCA, chain, err := s.overrideGetter.GetWorkloadIdentityX509CAOverride(ctx, "", tlsCA)
	if err != nil {
		return nil, nil, trace.Wrap(err, "getting CA override")
	}

	return newCA, chain, nil
}

func rawAttrsToStruct(in *workloadidentityv1pb.Attrs) (*apievents.Struct, error) {
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

func baseEvent(
	ctx context.Context,
	wi *workloadidentityv1pb.WorkloadIdentity,
	spiffeID spiffeid.ID,
	attrs *workloadidentityv1pb.Attrs,
	sigstorePolicyResults map[string]error,
	nameSelector string,
	labelSelectors []*workloadidentityv1pb.LabelSelector,
) (*apievents.SPIFFESVIDIssued, error) {
	structAttrs, err := rawAttrsToStruct(attrs)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling attributes")
	}

	workloadFields := structAttrs.GetFields()["workload"].GetStructValue().GetFields()
	if workloadFields != nil {
		delete(workloadFields, "sigstore")
		if len(sigstorePolicyResults) != 0 {
			policyResults := make(map[string]any)
			for name, err := range sigstorePolicyResults {
				result := map[string]any{
					"satisfied": err == nil,
				}
				if err != nil {
					result["reason"] = err.Error()
				}
				policyResults[name] = result
			}
			field, err := apievents.EncodeMap(map[string]any{
				"payload_count":      len(attrs.GetWorkload().GetSigstore().GetPayloads()),
				"evaluated_policies": policyResults,
			})
			if err != nil {
				return nil, trace.Wrap(err, "marshaling sigstore attributes")
			}
			workloadFields["sigstore"] = &gogotypes.Value{
				Kind: &gogotypes.Value_StructValue{
					StructValue: &field.Struct,
				},
			}
		}
	}

	return &apievents.SPIFFESVIDIssued{
		Metadata: apievents.Metadata{
			Type: events.SPIFFESVIDIssuedEvent,
			Code: events.SPIFFESVIDIssuedSuccessCode,
		},
		UserMetadata:             authz.ClientUserMetadata(ctx),
		ConnectionMetadata:       authz.ConnectionMetadata(ctx),
		SPIFFEID:                 spiffeID.String(),
		Hint:                     wi.GetSpec().GetSpiffe().GetHint(),
		WorkloadIdentity:         wi.GetMetadata().GetName(),
		WorkloadIdentityRevision: wi.GetMetadata().GetRevision(),
		Attributes:               structAttrs,
		NameSelector:             nameSelector,
		LabelSelectors:           labelSelectorsToAudit(labelSelectors),
	}, nil
}

func labelSelectorsToAudit(
	in []*workloadidentityv1pb.LabelSelector,
) []*apievents.LabelSelector {
	if len(in) == 0 {
		return nil
	}
	out := make([]*apievents.LabelSelector, 0, len(in))
	for _, ls := range in {
		out = append(out, &apievents.LabelSelector{
			Key:    ls.Key,
			Values: ls.Values,
		})
	}
	return out
}

func calculateTTL(
	ctx context.Context,
	log *slog.Logger,
	clock clockwork.Clock,
	requestedTTL time.Duration,
	configuredMaxTTL time.Duration,
) (time.Time, time.Time, time.Time, time.Duration) {
	ttl := cmp.Or(requestedTTL, defaultTTL)
	maxTTL := cmp.Or(configuredMaxTTL, defaultMaxTTL)

	if ttl > maxTTL {
		log.InfoContext(
			ctx,
			"Requested SVID TTL exceeds maximum, using maximum instead",
			"requested_ttl", ttl,
			"max_ttl", maxTTL)
		ttl = maxTTL
	}

	now := clock.Now()
	notBefore := now.Add(-1 * time.Minute)
	notAfter := now.Add(ttl)
	return now, notBefore, notAfter, ttl
}

type issueX509SVIDParams struct {
	ca                    *tlsca.CertAuthority
	chain                 [][]byte
	workloadIdentity      *workloadidentityv1pb.WorkloadIdentity
	x509Params            *workloadidentityv1pb.X509SVIDParams
	requestedTTL          time.Duration
	attrs                 *workloadidentityv1pb.Attrs
	sigstorePolicyResults map[string]error
	nameSelector          string
	labelSelectors        []*workloadidentityv1pb.LabelSelector
}

func (s *IssuanceService) issueX509SVID(ctx context.Context, params issueX509SVIDParams) (_ *workloadidentityv1pb.Credential, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/issueX509SVID")
	defer func() { tracing.EndSpan(span, err) }()

	switch {
	case params.x509Params == nil:
		return nil, trace.BadParameter("x509_svid_params: is required")
	case len(params.x509Params.PublicKey) == 0:
		return nil, trace.BadParameter("x509_svid_params.public_key: is required")
	}

	spiffeID, err := spiffeid.FromURI(&url.URL{
		Scheme: "spiffe",
		Host:   s.clusterName,
		Path:   params.workloadIdentity.GetSpec().GetSpiffe().GetId(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "parsing SPIFFE ID")
	}
	_, notBefore, notAfter, ttl := calculateTTL(
		ctx,
		s.logger,
		s.clock,
		params.requestedTTL,
		params.workloadIdentity.GetSpec().GetSpiffe().GetX509().GetMaximumTtl().AsDuration(),
	)

	pubKey, err := x509.ParsePKIXPublicKey(params.x509Params.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err, "parsing public key")
	}

	certSerial, err := generateCertSerial()
	if err != nil {
		return nil, trace.Wrap(err, "generating certificate serial")
	}
	serialString := serialString(certSerial)

	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		x509Template(
			certSerial,
			notBefore,
			notAfter,
			spiffeID,
			params.workloadIdentity.GetSpec().GetSpiffe().GetX509().GetDnsSans(),
			params.workloadIdentity.GetSpec().GetSpiffe().GetX509().GetSubjectTemplate(),
		),
		params.ca.Cert,
		pubKey,
		params.ca.Signer,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt, err := baseEvent(
		ctx,
		params.workloadIdentity,
		spiffeID,
		params.attrs,
		params.sigstorePolicyResults,
		params.nameSelector,
		params.labelSelectors,
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating base event")
	}
	evt.SVIDType = "x509"
	evt.SerialNumber = serialString
	evt.DNSSANs = params.workloadIdentity.GetSpec().GetSpiffe().GetX509().GetDnsSans()
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.WarnContext(
			ctx,
			"failed to emit audit event for SVID issuance",
			"error", err,
			"event", evt,
		)
	}

	return &workloadidentityv1pb.Credential{
		WorkloadIdentityName:     params.workloadIdentity.GetMetadata().GetName(),
		WorkloadIdentityRevision: params.workloadIdentity.GetMetadata().GetRevision(),

		SpiffeId: spiffeID.String(),
		Hint:     params.workloadIdentity.GetSpec().GetSpiffe().GetHint(),

		ExpiresAt: timestamppb.New(notAfter),
		Ttl:       durationpb.New(ttl),

		Credential: &workloadidentityv1pb.Credential_X509Svid{
			X509Svid: &workloadidentityv1pb.X509SVIDCredential{
				Cert:         certBytes,
				SerialNumber: serialString,
				Chain:        params.chain,
			},
		},
	}, nil
}

const jtiLength = 16

func (s *IssuanceService) getJWTIssuerKey(
	ctx context.Context,
) (_ *jwt.Key, _ string, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/getJWTIssuerKey")
	defer func() { tracing.EndSpan(span, err) }()

	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: s.clusterName,
	}, true)
	if err != nil {
		return nil, "", trace.Wrap(err, "getting SPIFFE CA")
	}

	jwtSigner, err := s.keyStore.GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, "", trace.Wrap(err, "getting JWT signer")
	}

	jwtKey, err := services.GetJWTSigner(
		jwtSigner, s.clusterName, s.clock,
	)
	if err != nil {
		return nil, "", trace.Wrap(err, "creating JWT signer")
	}

	// Determine the public address of the proxy for inclusion in the JWT as
	// the issuer for purposes of OIDC compatibility.
	issuer, err := oidc.IssuerForCluster(ctx, s.cache, "/workload-identity")
	if err != nil {
		return nil, "", trace.Wrap(err, "determining issuer URI")
	}

	return jwtKey, issuer, nil
}

type issueJWTSVIDParams struct {
	issuerKey             *jwt.Key
	issuerURI             string
	workloadIdentity      *workloadidentityv1pb.WorkloadIdentity
	jwtParams             *workloadidentityv1pb.JWTSVIDParams
	requestedTTL          time.Duration
	attrs                 *workloadidentityv1pb.Attrs
	sigstorePolicyResults map[string]error
	nameSelector          string
	labelSelectors        []*workloadidentityv1pb.LabelSelector
}

func (s *IssuanceService) issueJWTSVID(ctx context.Context, params issueJWTSVIDParams) (_ *workloadidentityv1pb.Credential, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/issueJWTSVID")
	defer func() { tracing.EndSpan(span, err) }()

	switch {
	case params.jwtParams == nil:
		return nil, trace.BadParameter("jwt_svid_params: is required")
	case len(params.jwtParams.Audiences) == 0:
		return nil, trace.BadParameter("jwt_svid_params.audiences: at least one audience should be specified")
	}

	spiffeID, err := spiffeid.FromURI(&url.URL{
		Scheme: "spiffe",
		Host:   s.clusterName,
		Path:   params.workloadIdentity.GetSpec().GetSpiffe().GetId(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "parsing SPIFFE ID")
	}
	now, _, notAfter, ttl := calculateTTL(
		ctx,
		s.logger,
		s.clock,
		params.requestedTTL,
		params.workloadIdentity.GetSpec().GetSpiffe().GetJwt().GetMaximumTtl().AsDuration(),
	)

	jti, err := utils.CryptoRandomHex(jtiLength)
	if err != nil {
		return nil, trace.Wrap(err, "generating JTI")
	}

	signed, err := params.issuerKey.SignJWTSVID(jwt.SignParamsJWTSVID{
		Audiences: params.jwtParams.Audiences,
		SPIFFEID:  spiffeID,
		JTI:       jti,
		Issuer:    params.issuerURI,

		SetIssuedAt: now,
		SetExpiry:   notAfter,

		PrivateClaims: params.workloadIdentity.GetSpec().GetSpiffe().GetJwt().GetExtraClaims().AsMap(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "signing jwt")
	}

	evt, err := baseEvent(
		ctx,
		params.workloadIdentity,
		spiffeID,
		params.attrs,
		params.sigstorePolicyResults,
		params.nameSelector,
		params.labelSelectors,
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating base event")
	}
	evt.SVIDType = "jwt"
	evt.JTI = jti
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.WarnContext(
			ctx,
			"failed to emit audit event for SVID issuance",
			"error", err,
			"event", evt,
		)
	}

	return &workloadidentityv1pb.Credential{
		WorkloadIdentityName:     params.workloadIdentity.GetMetadata().GetName(),
		WorkloadIdentityRevision: params.workloadIdentity.GetMetadata().GetRevision(),

		SpiffeId: spiffeID.String(),
		Hint:     params.workloadIdentity.GetSpec().GetSpiffe().GetHint(),

		ExpiresAt: timestamppb.New(notAfter),
		Ttl:       durationpb.New(ttl),

		Credential: &workloadidentityv1pb.Credential_JwtSvid{
			JwtSvid: &workloadidentityv1pb.JWTSVIDCredential{
				Jwt: signed,
				Jti: jti,
			},
		},
	}, nil
}

// authorizeIssuance verifies that the caller may issue an SVID using the given
// WorkloadIdentity. Mirroring the unscoped issuance checks while being
// scope-aware, the caller must, within the WorkloadIdentity's scope, hold the
// given rule verbs (read_no_secrets, plus list for the multi-issue RPC) for the
// workload_identity kind AND match the resource via a workload_identity label
// selector. read_no_secrets is used rather than read because it is the strongest
// read verb scoped roles can grant for workload_identity, and a read_no_secrets
// rule check is satisfied by an unscoped role granting read — preserving the
// behavior of the classic authorizer for unscoped identities.
func (s *IssuanceService) authorizeIssuance(
	ctx context.Context,
	authCtx *authz.ScopedContext,
	wi *workloadidentityv1pb.WorkloadIdentity,
	verbs ...string,
) error {
	ruleCtx := authCtx.RuleContext()
	ruleCtx.Resource153 = wi
	// Rule-based access: the caller must hold the read/list verbs in the scope.
	if err := authCtx.CheckerContext.Decision(
		ctx, wi.GetScope(), func(checker *services.ScopedAccessChecker) error {
			return checker.CheckAccessToRules(&ruleCtx, types.KindWorkloadIdentity, verbs...)
		},
	); err != nil {
		return trace.Wrap(err)
	}
	// Label-based issuance grant: the caller must match the resource by virtue of
	// a workload_identity label selector in the scope.
	if err := authCtx.CheckerContext.Decision(
		ctx, wi.GetScope(), func(checker *services.ScopedAccessChecker) error {
			return checker.CheckAccessToWorkloadIdentity(wi)
		},
	); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// validateRenderedSpiffeIDScope re-validates that a scoped WorkloadIdentity's
// rendered (post-templating) SPIFFE ID still conforms to its scope. This is a
// defense-in-depth measure ensuring templating cannot be used to escape the
// scope-based SPIFFE ID restrictions enforced at create/update time. It is a
// no-op for unscoped WorkloadIdentities.
func validateRenderedSpiffeIDScope(wi *workloadidentityv1pb.WorkloadIdentity) error {
	if wi.GetScope() == "" {
		return nil
	}
	if err := services.ValidateScopedSPIFFEID(
		wi.GetScope(), wi.GetSpec().GetSpiffe().GetId(),
	); err != nil {
		return trace.Wrap(err, "validating rendered SPIFFE ID against scope")
	}
	return nil
}

func convertLabels(selectors []*workloadidentityv1pb.LabelSelector) types.Labels {
	labels := types.Labels{}
	for _, selector := range selectors {
		labels[selector.Key] = selector.Values
	}
	return labels
}

func serialString(serial *big.Int) string {
	hex := serial.Text(16)
	if len(hex)%2 == 1 {
		hex = "0" + hex
	}

	out := strings.Builder{}
	for i := 0; i < len(hex); i += 2 {
		if i != 0 {
			out.WriteString(":")
		}
		out.WriteString(hex[i : i+2])
	}
	return out.String()
}

// certVerifyClockSkewAllowance is the amount of leeway added to the
// certificate's expiration status check to allow for clock drift.
const certVerifyClockSkewAllowance = 1 * time.Minute

func verifyCertValidityWithSkew(cert *x509.Certificate, now time.Time) error {
	if now.Add(certVerifyClockSkewAllowance).Before(cert.NotBefore) {
		return x509.CertificateInvalidError{
			Cert:   cert,
			Reason: x509.Expired,
			Detail: "certificate is not yet valid",
		}
	}

	if now.Add(-certVerifyClockSkewAllowance).After(cert.NotAfter) {
		return x509.CertificateInvalidError{
			Cert:   cert,
			Reason: x509.Expired,
			Detail: "certificate has expired",
		}
	}

	return nil
}
