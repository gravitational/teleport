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
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"log/slog"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	traitv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trait/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
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

type issuerCache interface {
	workloadIdentityReader
	GetProxies() ([]types.Server, error)
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
}

// IssuanceServiceConfig holds configuration options for the IssuanceService.
type IssuanceServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      issuerCache
	Clock      clockwork.Clock
	Emitter    apievents.Emitter
	Logger     *slog.Logger
	KeyStore   KeyStorer

	ClusterName string
}

// IssuanceService is the gRPC service for managing workload identity resources.
// It implements the workloadidentityv1pb.WorkloadIdentityIssuanceServiceServer.
type IssuanceService struct {
	workloadidentityv1pb.UnimplementedWorkloadIdentityIssuanceServiceServer

	authorizer authz.Authorizer
	cache      issuerCache
	clock      clockwork.Clock
	emitter    apievents.Emitter
	logger     *slog.Logger
	keyStore   KeyStorer

	clusterName string
}

// NewIssuanceService returns a new instance of the IssuanceService.
func NewIssuanceService(cfg *IssuanceServiceConfig) (*IssuanceService, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.KeyStore == nil:
		return nil, trace.BadParameter("key store is required")
	case cfg.ClusterName == "":
		return nil, trace.BadParameter("cluster name is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "workload_identity_issuance.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &IssuanceService{
		authorizer:  cfg.Authorizer,
		cache:       cfg.Cache,
		clock:       cfg.Clock,
		emitter:     cfg.Emitter,
		logger:      cfg.Logger,
		keyStore:    cfg.KeyStore,
		clusterName: cfg.ClusterName,
	}, nil
}

func (s *IssuanceService) deriveAttrs(
	authzCtx *authz.Context,
	workloadAttrs *workloadidentityv1pb.WorkloadAttrs,
) (*workloadidentityv1pb.Attrs, error) {
	attrs := &workloadidentityv1pb.Attrs{
		Workload: workloadAttrs,
		User: &workloadidentityv1pb.UserAttrs{
			Name:    authzCtx.Identity.GetIdentity().Username,
			IsBot:   authzCtx.Identity.GetIdentity().BotName != "",
			BotName: authzCtx.Identity.GetIdentity().BotName,
			Labels:  authzCtx.User.GetAllLabels(),
		},
		Join: authzCtx.Identity.GetIdentity().JoinAttributes,
	}

	for key, values := range authzCtx.Identity.GetIdentity().Traits {
		attrs.User.Traits = append(attrs.User.Traits, &traitv1.Trait{
			Key:    key,
			Values: values,
		})
	}

	return attrs, nil
}

var defaultMaxTTL = 24 * time.Hour

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

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentity, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	attrs, err := s.deriveAttrs(authCtx, req.GetWorkloadAttrs())
	if err != nil {
		return nil, trace.Wrap(err, "deriving attributes")
	}

	wi, err := s.cache.GetWorkloadIdentity(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Check the principal has access to the workload identity resource by
	// virtue of WorkloadIdentityLabels on a role.
	if err := authCtx.Checker.CheckAccess(
		types.Resource153ToResourceWithLabels(wi),
		services.AccessState{},
	); err != nil {
		return nil, trace.Wrap(err)
	}

	decision := decide(ctx, wi, attrs)
	if !decision.shouldIssue {
		return nil, trace.Wrap(decision.reason, "workload identity failed evaluation")
	}

	var cred *workloadidentityv1pb.Credential
	switch v := req.GetCredential().(type) {
	case *workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams:
		ca, err := s.getX509CA(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "fetching X509 SPIFFE CA")
		}
		cred, err = s.issueX509SVID(
			ctx,
			ca,
			decision.templatedWorkloadIdentity,
			v.X509SvidParams,
			req.RequestedTtl.AsDuration(),
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
			key,
			issuer,
			decision.templatedWorkloadIdentity,
			v.JwtSvidParams,
			req.RequestedTtl.AsDuration(),
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

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindWorkloadIdentity, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	attrs, err := s.deriveAttrs(authCtx, req.GetWorkloadAttrs())
	if err != nil {
		return nil, trace.Wrap(err, "deriving attributes")
	}

	// Fetch all workload identities that match the label selectors AND the
	// principal can access.
	workloadIdentities, err := s.matchingAndAuthorizedWorkloadIdentities(
		ctx,
		authCtx,
		convertLabels(req.LabelSelectors),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Evaluate rules/templating for each worklaod identity, filtering out those
	// that should not be issued.
	shouldIssue := []*workloadidentityv1pb.WorkloadIdentity{}
	for _, wi := range workloadIdentities {
		decision := decide(ctx, wi, attrs)
		if decision.shouldIssue {
			shouldIssue = append(shouldIssue, decision.templatedWorkloadIdentity)
		}
		if len(shouldIssue) > maxWorkloadIdentitiesIssued {
			// If we're now above the limit, then we want to exit out...
			return nil, trace.BadParameter(
				"number of identities that would be issued exceeds maximum permitted (max = %d), use more specific labels",
				maxWorkloadIdentitiesIssued,
			)
		}
	}

	var creds = make([]*workloadidentityv1pb.Credential, 0, len(shouldIssue))
	switch v := req.GetCredential().(type) {
	case *workloadidentityv1pb.IssueWorkloadIdentitiesRequest_X509SvidParams:
		ca, err := s.getX509CA(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "fetching CA to sign X509 SVID")
		}
		for _, wi := range shouldIssue {
			cred, err := s.issueX509SVID(
				ctx,
				ca,
				wi,
				v.X509SvidParams,
				req.RequestedTtl.AsDuration(),
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
				key,
				issuer,
				wi,
				v.JwtSvidParams,
				req.RequestedTtl.AsDuration(),
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
) (_ *tlsca.CertAuthority, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/getX509CA")
	defer func() { tracing.EndSpan(span, err) }()

	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: s.clusterName,
	}, true)
	tlsCert, tlsSigner, err := s.keyStore.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "getting CA cert and key")
	}
	tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsCA, nil
}

func baseEvent(
	ctx context.Context,
	wi *workloadidentityv1pb.WorkloadIdentity,
	spiffeID spiffeid.ID,
) *apievents.SPIFFESVIDIssued {
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
	}
}

func calculateTTL(
	clock clockwork.Clock,
	requestedTTL time.Duration,
) (time.Time, time.Time, time.Time, time.Duration) {
	ttl := time.Hour
	if requestedTTL != 0 {
		ttl = requestedTTL
		if ttl > defaultMaxTTL {
			ttl = defaultMaxTTL
		}
	}
	now := clock.Now()
	notBefore := now.Add(-1 * time.Minute)
	notAfter := now.Add(ttl)
	return now, notBefore, notAfter, ttl
}

func (s *IssuanceService) issueX509SVID(
	ctx context.Context,
	ca *tlsca.CertAuthority,
	wid *workloadidentityv1pb.WorkloadIdentity,
	params *workloadidentityv1pb.X509SVIDParams,
	requestedTTL time.Duration,
) (_ *workloadidentityv1pb.Credential, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/issueX509SVID")
	defer func() { tracing.EndSpan(span, err) }()

	switch {
	case params == nil:
		return nil, trace.BadParameter("x509_svid_params: is required")
	case len(params.PublicKey) == 0:
		return nil, trace.BadParameter("x509_svid_params.public_key: is required")
	}

	spiffeID, err := spiffeid.FromURI(&url.URL{
		Scheme: "spiffe",
		Host:   s.clusterName,
		Path:   wid.GetSpec().GetSpiffe().GetId(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "parsing SPIFFE ID")
	}
	_, notBefore, notAfter, ttl := calculateTTL(s.clock, requestedTTL)

	pubKey, err := x509.ParsePKIXPublicKey(params.PublicKey)
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
			wid.GetSpec().GetSpiffe().GetX509().GetDnsSans(),
			wid.GetSpec().GetSpiffe().GetX509().GetSubjectTemplate(),
		),
		ca.Cert,
		pubKey,
		ca.Signer,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evt := baseEvent(ctx, wid, spiffeID)
	evt.SVIDType = "x509"
	evt.SerialNumber = serialString
	evt.DNSSANs = wid.GetSpec().GetSpiffe().GetX509().GetDnsSans()
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.WarnContext(
			ctx,
			"failed to emit audit event for SVID issuance",
			"error", err,
			"event", evt,
		)
	}

	return &workloadidentityv1pb.Credential{
		WorkloadIdentityName:     wid.GetMetadata().GetName(),
		WorkloadIdentityRevision: wid.GetMetadata().GetRevision(),

		SpiffeId: spiffeID.String(),
		Hint:     wid.GetSpec().GetSpiffe().GetHint(),

		ExpiresAt: timestamppb.New(notAfter),
		Ttl:       durationpb.New(ttl),

		Credential: &workloadidentityv1pb.Credential_X509Svid{
			X509Svid: &workloadidentityv1pb.X509SVIDCredential{
				Cert:         certBytes,
				SerialNumber: serialString,
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

func (s *IssuanceService) issueJWTSVID(
	ctx context.Context,
	issuerKey *jwt.Key,
	issuerURI string,
	wid *workloadidentityv1pb.WorkloadIdentity,
	params *workloadidentityv1pb.JWTSVIDParams,
	requestedTTL time.Duration,
) (_ *workloadidentityv1pb.Credential, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/issueJWTSVID")
	defer func() { tracing.EndSpan(span, err) }()

	switch {
	case params == nil:
		return nil, trace.BadParameter("jwt_svid_params: is required")
	case len(params.Audiences) == 0:
		return nil, trace.BadParameter("jwt_svid_params.audiences: at least one audience should be specified")
	}

	spiffeID, err := spiffeid.FromURI(&url.URL{
		Scheme: "spiffe",
		Host:   s.clusterName,
		Path:   wid.GetSpec().GetSpiffe().GetId(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "parsing SPIFFE ID")
	}
	now, _, notAfter, ttl := calculateTTL(s.clock, requestedTTL)

	jti, err := utils.CryptoRandomHex(jtiLength)
	if err != nil {
		return nil, trace.Wrap(err, "generating JTI")
	}

	signed, err := issuerKey.SignJWTSVID(jwt.SignParamsJWTSVID{
		Audiences: params.Audiences,
		SPIFFEID:  spiffeID,
		JTI:       jti,
		Issuer:    issuerURI,

		SetIssuedAt: now,
		SetExpiry:   notAfter,
	})
	if err != nil {
		return nil, trace.Wrap(err, "signing jwt")
	}

	evt := baseEvent(ctx, wid, spiffeID)
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
		WorkloadIdentityName:     wid.GetMetadata().GetName(),
		WorkloadIdentityRevision: wid.GetMetadata().GetRevision(),

		SpiffeId: spiffeID.String(),
		Hint:     wid.GetSpec().GetSpiffe().GetHint(),

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

func (s *IssuanceService) getAllWorkloadIdentities(
	ctx context.Context,
) ([]*workloadidentityv1pb.WorkloadIdentity, error) {
	workloadIdentities := []*workloadidentityv1pb.WorkloadIdentity{}
	page := ""
	for {
		pageItems, nextPage, err := s.cache.ListWorkloadIdentities(ctx, 0, page)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		workloadIdentities = append(workloadIdentities, pageItems...)
		if nextPage == "" {
			break
		}
		page = nextPage
	}
	return workloadIdentities, nil
}

// matchingAndAuthorizedWorkloadIdentities returns the workload identities that
// match the provided labels and the principal has access to.
func (s *IssuanceService) matchingAndAuthorizedWorkloadIdentities(
	ctx context.Context,
	authCtx *authz.Context,
	labels types.Labels,
) ([]*workloadidentityv1pb.WorkloadIdentity, error) {
	allWorkloadIdentities, err := s.getAllWorkloadIdentities(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	canAccess := []*workloadidentityv1pb.WorkloadIdentity{}
	// Filter out identities user cannot access.
	for _, wid := range allWorkloadIdentities {
		if err := authCtx.Checker.CheckAccess(
			types.Resource153ToResourceWithLabels(wid),
			services.AccessState{},
		); err == nil {
			canAccess = append(canAccess, wid)
		}
	}

	canAccessAndInSearch := []*workloadidentityv1pb.WorkloadIdentity{}
	for _, wid := range canAccess {
		match, _, err := services.MatchLabelGetter(
			labels, types.Resource153ToResourceWithLabels(wid),
		)
		if err != nil {
			// Maybe log and skip rather than returning an error?
			return nil, trace.Wrap(err)
		}
		if match {
			canAccessAndInSearch = append(canAccessAndInSearch, wid)
		}
	}

	return canAccessAndInSearch, nil
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
