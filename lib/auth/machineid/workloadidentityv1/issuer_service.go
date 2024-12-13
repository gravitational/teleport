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
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"go.opentelemetry.io/otel"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/experiment"
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

// getFieldStringValue returns a string value from the given attribute set.
// The attribute is specified as a dot-separated path to the field in the
// attribute set.
//
// The specified attribute must be a string field. If the attribute is not
// found, an error is returned.
//
// TODO(noah): This function will be replaced by the Teleport predicate language
// in a coming PR.
func getFieldStringValue(attrs *workloadidentityv1pb.Attrs, attr string) (string, error) {
	attrParts := strings.Split(attr, ".")
	message := attrs.ProtoReflect()
	// TODO(noah): Improve errors by including the fully qualified attribute
	// (e.g add up the parts of the attribute path processed thus far)
	for i, part := range attrParts {
		fieldDesc := message.Descriptor().Fields().ByTextName(part)
		if fieldDesc == nil {
			return "", trace.NotFound("attribute %q not found", part)
		}
		// We expect the final key to point to a string field - otherwise - we
		// return an error.
		if i == len(attrParts)-1 {
			if fieldDesc.Kind() != protoreflect.StringKind {
				return "", trace.BadParameter("attribute %q is not a string", part)
			}
			return message.Get(fieldDesc).String(), nil
		}
		// If we're not processing the final key part, we expect this to point
		// to a message that we can further explore.
		if fieldDesc.Kind() != protoreflect.MessageKind {
			return "", trace.BadParameter("attribute %q is not a message", part)
		}
		message = message.Get(fieldDesc).Message()
	}
	return "", nil
}

// templateString takes a given input string and replaces any values within
// {{ }} with values from the attribute set.
//
// If the specified value is not found in the attribute set, an error is
// returned.
//
// TODO(noah): In a coming PR, this will be replaced by evaluating the values
// within the handlebars as expressions.
func templateString(in string, attrs *workloadidentityv1pb.Attrs) (string, error) {
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	matches := re.FindAllStringSubmatch(in, -1)

	for _, match := range matches {
		attrKey := strings.Trim(match[0], "{}")
		attrKey = strings.TrimFunc(attrKey, unicode.IsSpace)
		value, err := getFieldStringValue(attrs, attrKey)
		if err != nil {
			return "", trace.Wrap(err, "fetching attribute value for %q", attrKey)
		}
		// We want to have an implicit rule here that if an attribute is
		// included in the template, but is not set, we should refuse to issue
		// the credential.
		if value == "" {
			return "", trace.NotFound("attribute %q unset", attrKey)
		}
		in = strings.Replace(in, match[0], value, 1)
	}

	return in, nil
}

func evaluateRules(
	wi *workloadidentityv1pb.WorkloadIdentity,
	attrs *workloadidentityv1pb.Attrs,
) error {
	if len(wi.GetSpec().GetRules().GetAllow()) == 0 {
		return nil
	}
ruleLoop:
	for _, rule := range wi.GetSpec().GetRules().GetAllow() {
		for _, condition := range rule.GetConditions() {
			val, err := getFieldStringValue(attrs, condition.Attribute)
			if err != nil {
				return trace.Wrap(err)
			}
			if val != condition.Equals {
				continue ruleLoop
			}
		}
		return nil
	}
	return trace.AccessDenied("no matching rule found")
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
	}

	return attrs, nil
}

var defaultMaxTTL = 24 * time.Hour

func (s *IssuanceService) IssueWorkloadIdentity(
	ctx context.Context,
	req *workloadidentityv1pb.IssueWorkloadIdentityRequest,
) (*workloadidentityv1pb.IssueWorkloadIdentityResponse, error) {
	if !experiment.Enabled() {
		return nil, trace.AccessDenied("workload identity issuance experiment is disabled")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case req.GetName() == "":
		return nil, trace.BadParameter("name: is required")
	case req.GetCredential() == nil:
		return nil, trace.BadParameter("at least one credential type must be requested")
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

	attrs, err := s.deriveAttrs(authCtx, req.GetWorkloadAttrs())
	if err != nil {
		return nil, trace.Wrap(err, "deriving attributes")
	}
	// Evaluate any rules explicitly configured by the user
	if err := evaluateRules(wi, attrs); err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform any templating
	spiffeIDPath, err := templateString(wi.GetSpec().GetSpiffe().GetId(), attrs)
	if err != nil {
		return nil, trace.Wrap(err, "templating spec.spiffe.id")
	}
	spiffeID, err := spiffeid.FromURI(&url.URL{
		Scheme: "spiffe",
		Host:   s.clusterName,
		Path:   spiffeIDPath,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating SPIFFE ID")
	}

	hint, err := templateString(wi.GetSpec().GetSpiffe().GetHint(), attrs)
	if err != nil {
		return nil, trace.Wrap(err, "templating spec.spiffe.hint")
	}

	// TODO(noah): Add more sophisticated control of the TTL.
	ttl := time.Hour
	if req.RequestedTtl != nil && req.RequestedTtl.AsDuration() != 0 {
		ttl := req.RequestedTtl.AsDuration()
		if ttl > defaultMaxTTL {
			ttl = defaultMaxTTL
		}
	}

	now := s.clock.Now()
	notBefore := now.Add(-1 * time.Minute)
	notAfter := now.Add(ttl)

	// Prepare event
	evt := &apievents.SPIFFESVIDIssued{
		Metadata: apievents.Metadata{
			Type: events.SPIFFESVIDIssuedEvent,
			Code: events.SPIFFESVIDIssuedSuccessCode,
		},
		UserMetadata:             authz.ClientUserMetadata(ctx),
		ConnectionMetadata:       authz.ConnectionMetadata(ctx),
		SPIFFEID:                 spiffeID.String(),
		Hint:                     hint,
		WorkloadIdentity:         wi.GetMetadata().GetName(),
		WorkloadIdentityRevision: wi.GetMetadata().GetRevision(),
	}
	cred := &workloadidentityv1pb.Credential{
		WorkloadIdentityName:     wi.GetMetadata().GetName(),
		WorkloadIdentityRevision: wi.GetMetadata().GetRevision(),

		SpiffeId: spiffeID.String(),
		Hint:     hint,

		ExpiresAt: timestamppb.New(notAfter),
		Ttl:       durationpb.New(ttl),
	}

	switch v := req.GetCredential().(type) {
	case *workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams:
		evt.SVIDType = "x509"
		certDer, certSerial, err := s.issueX509SVID(
			ctx,
			v.X509SvidParams,
			notBefore,
			notAfter,
			spiffeID,
		)
		if err != nil {
			return nil, trace.Wrap(err, "issuing X509 SVID")
		}
		serialStr := serialString(certSerial)
		cred.Credential = &workloadidentityv1pb.Credential_X509Svid{
			X509Svid: &workloadidentityv1pb.X509SVIDCredential{
				Cert:         certDer,
				SerialNumber: serialStr,
			},
		}
		evt.SerialNumber = serialStr
	case *workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams:
		evt.SVIDType = "jwt"
		signedJwt, jti, err := s.issueJWTSVID(
			ctx,
			v.JwtSvidParams,
			now,
			notAfter,
			spiffeID,
		)
		if err != nil {
			return nil, trace.Wrap(err, "issuing JWT SVID")
		}
		cred.Credential = &workloadidentityv1pb.Credential_JwtSvid{
			JwtSvid: &workloadidentityv1pb.JWTSVIDCredential{
				Jwt: signedJwt,
				Jti: jti,
			},
		}
		evt.JTI = jti
	default:
		return nil, trace.BadParameter("credential: unknown type %T", req.GetCredential())
	}

	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.WarnContext(
			ctx,
			"failed to emit audit event for SVID issuance",
			"error", err,
			"event", evt,
		)
	}

	return &workloadidentityv1pb.IssueWorkloadIdentityResponse{
		Credential: cred,
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
) *x509.Certificate {
	return &x509.Certificate{
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
		URIs: []*url.URL{spiffeID.URL()},
	}
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

func (s *IssuanceService) issueX509SVID(
	ctx context.Context,
	params *workloadidentityv1pb.X509SVIDParams,
	notBefore time.Time,
	notAfter time.Time,
	spiffeID spiffeid.ID,
) (_ []byte, _ *big.Int, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/issueX509SVID")
	defer func() { tracing.EndSpan(span, err) }()

	switch {
	case params == nil:
		return nil, nil, trace.BadParameter("x509_svid_params: is required")
	case len(params.PublicKey) == 0:
		return nil, nil, trace.BadParameter("x509_svid_params.public_key: is required")
	}

	pubKey, err := x509.ParsePKIXPublicKey(params.PublicKey)
	if err != nil {
		return nil, nil, trace.Wrap(err, "parsing public key")
	}

	certSerial, err := generateCertSerial()
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating certificate serial")
	}
	template := x509Template(certSerial, notBefore, notAfter, spiffeID)

	ca, err := s.getX509CA(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err, "fetching CA to sign X509 SVID")
	}
	certBytes, err := x509.CreateCertificate(
		rand.Reader, template, ca.Cert, pubKey, ca.Signer,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return certBytes, certSerial, nil
}

const jtiLength = 16

func (s *IssuanceService) getJWTIssuerKey(
	ctx context.Context,
) (_ *jwt.Key, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/getJWTIssuerKey")
	defer func() { tracing.EndSpan(span, err) }()

	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: s.clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err, "getting SPIFFE CA")
	}

	jwtSigner, err := s.keyStore.GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "getting JWT signer")
	}

	jwtKey, err := services.GetJWTSigner(
		jwtSigner, s.clusterName, s.clock,
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating JWT signer")
	}
	return jwtKey, nil
}

func (s *IssuanceService) issueJWTSVID(
	ctx context.Context,
	params *workloadidentityv1pb.JWTSVIDParams,
	now time.Time,
	notAfter time.Time,
	spiffeID spiffeid.ID,
) (_ string, _ string, err error) {
	ctx, span := tracer.Start(ctx, "IssuanceService/issueJWTSVID")
	defer func() { tracing.EndSpan(span, err) }()

	switch {
	case params == nil:
		return "", "", trace.BadParameter("jwt_svid_params: is required")
	case len(params.Audiences) == 0:
		return "", "", trace.BadParameter("jwt_svid_params.audiences: at least one audience should be specified")
	}

	jti, err := utils.CryptoRandomHex(jtiLength)
	if err != nil {
		return "", "", trace.Wrap(err, "generating JTI")
	}

	key, err := s.getJWTIssuerKey(ctx)
	if err != nil {
		return "", "", trace.Wrap(err, "getting JWT issuer key")
	}

	// Determine the public address of the proxy for inclusion in the JWT as
	// the issuer for purposes of OIDC compatibility.
	issuer, err := oidc.IssuerForCluster(ctx, s.cache, "/workload-identity")
	if err != nil {
		return "", "", trace.Wrap(err, "determining issuer URI")
	}

	signed, err := key.SignJWTSVID(jwt.SignParamsJWTSVID{
		Audiences: params.Audiences,
		SPIFFEID:  spiffeID,
		JTI:       jti,
		Issuer:    issuer,

		SetIssuedAt: now,
		SetExpiry:   notAfter,
	})
	if err != nil {
		return "", "", trace.Wrap(err, "signing jwt")
	}

	return signed, jti, nil
}

func (s *IssuanceService) IssueWorkloadIdentities(
	ctx context.Context,
	req *workloadidentityv1pb.IssueWorkloadIdentitiesRequest,
) (*workloadidentityv1pb.IssueWorkloadIdentitiesResponse, error) {
	// TODO(noah): Coming to a PR near you soon!
	return nil, trace.NotImplemented("not implemented")
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
