package workloadidentityv1

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"iter"
	"net"
	"net/url"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ErrOverrideDeprecated indicates that workload_identity_x509_issuer_override resources are deprecated and can no
// longer be created or updated.
var ErrOverrideDeprecated = &trace.BadParameterError{
	Message: "creating or updating workload_identity_x509_issuer_override resources is no longer supported " +
		"as of Teleport v19; existing overrides remain in effect and can still be read and deleted. " +
		"To configure an X.509 issuer override, create a cert_authority_override resource instead " +
		"(tctl auth create-override-csr --type=spiffe-tls, then tctl auth create-override --type=spiffe-tls <cert.pem>). " +
		"See https://goteleport.com/docs/zero-trust-access/management/security/ca-overrides/ for details",
}

type CertAuthorityGetter interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
}

type TLSCertAndSignerGetter interface {
	GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error)
}

type X509OverridesServiceConfig struct {
	Authorizer authz.Authorizer
	Storage    services.WorkloadIdentityX509Overrides
	Emitter    apievents.Emitter
	CAGetter   CertAuthorityGetter
	KeyStore   TLSCertAndSignerGetter

	ClusterName string
}

// NewX509OverridesService returns a fully featured implementation of
// [workloadidentityv1pb.X509OverridesServiceServer], unlike the OSS
// implementation in lib/auth/machineid/workloadidentityv1.
func NewX509OverridesService(cfg X509OverridesServiceConfig) (*X509OverridesService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Storage == nil {
		return nil, trace.BadParameter("storage is required")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required")
	}
	if cfg.CAGetter == nil {
		return nil, trace.BadParameter("CA getter is required")
	}
	if cfg.KeyStore == nil {
		return nil, trace.BadParameter("key store is required")
	}

	if cfg.ClusterName == "" {
		return nil, trace.BadParameter("cluster name is required")
	}

	return &X509OverridesService{
		authorizer: cfg.Authorizer,
		storage:    cfg.Storage,
		emitter:    cfg.Emitter,
		caGetter:   cfg.CAGetter,
		keyStore:   cfg.KeyStore,

		clusterName: cfg.ClusterName,
	}, nil
}

// X509OverridesService is a fully featured implementation of
// [workloadidentityv1pb.X509OverridesServiceServer].
type X509OverridesService struct {
	workloadidentityv1pb.UnimplementedX509OverridesServiceServer

	authorizer authz.Authorizer
	storage    services.WorkloadIdentityX509Overrides
	emitter    apievents.Emitter
	caGetter   CertAuthorityGetter
	keyStore   TLSCertAndSignerGetter

	clusterName string
}

var _ workloadidentityv1pb.X509OverridesServiceServer = (*X509OverridesService)(nil)

func (s *X509OverridesService) authorizeAccessToKind(ctx context.Context, kind string, verb string, additionalVerbs ...string) error {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return authzCtx.CheckAccessToKind(kind, verb, additionalVerbs...)
}

func (s *X509OverridesService) authorizeAccessToKindAdminReusedMFA(ctx context.Context, kind string, verb string, additionalVerbs ...string) error {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authzCtx.CheckAccessToKind(kind, verb, additionalVerbs...); err != nil {
		return trace.Wrap(err)
	}
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SignX509IssuerCSR implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) SignX509IssuerCSR(ctx context.Context, req *workloadidentityv1pb.SignX509IssuerCSRRequest) (*workloadidentityv1pb.SignX509IssuerCSRResponse, error) {
	if err := s.authorizeAccessToKind(ctx, types.KindWorkloadIdentityX509IssuerOverrideCSR, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	const loadKeysTrue = true
	ca, err := s.caGetter.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: s.clusterName,
	}, loadKeysTrue)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyPair := s.searchIssuerInCA(ca, req.GetIssuer())
	if keyPair == nil {
		return nil, trace.NotFound("issuer not found in SPIFFE CA")
	}
	issuerCert, err := x509.ParseCertificate(req.GetIssuer())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// HACK(espadolini): this is sort of a hack but this synchronous API is also
	// sort of a hack anyway (we don't really have a guarantee that the
	// requested issuer is usable by this auth server, it could be some other
	// auth server - or it could be a dead key that's unusable by any auth
	// server, too); keystore.Manager could be expanded to allow getting a
	// signer from a specific (public?) key, but at least by doing it this way
	// we are somewhat sure that we have sourced the key from the actual
	// cert_authority we intended to source it from and we were not tricked to
	// use some other unrelated key
	ca.SetActiveKeys(types.CAKeySet{TLS: []*types.TLSKeyPair{keyPair}})
	ca.SetAdditionalTrustedKeys(types.CAKeySet{})

	_, signer, err := s.keyStore.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	template, err := s.getCSRTemplateForIssuer(issuerCert, req.GetCsrCreationMode())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, template, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadidentityv1pb.SignX509IssuerCSRResponse_builder{
		Csr: csr,
	}.Build(), nil
}

func (*X509OverridesService) searchIssuerInCA(ca types.CertAuthority, issuerDER []byte) *types.TLSKeyPair {
	for _, kp := range chainSlices(
		ca.GetActiveKeys().TLS,
		ca.GetAdditionalTrustedKeys().TLS,
	) {
		if kp == nil {
			continue
		}
		block, _ := pem.Decode(kp.Cert)
		if block == nil || block.Type != "CERTIFICATE" {
			continue
		}
		if bytes.Equal(issuerDER, block.Bytes) {
			return kp
		}
	}
	return nil
}

func (*X509OverridesService) getCSRTemplateForIssuer(issuerCert *x509.Certificate, mode workloadidentityv1pb.CSRCreationMode) (*x509.CertificateRequest, error) {
	var subject pkix.Name
	switch mode {
	case workloadidentityv1pb.CSRCreationMode_CSR_CREATION_MODE_EMPTY:
		// empty subject, empty mind
	case workloadidentityv1pb.CSRCreationMode_CSR_CREATION_MODE_SAME:
		subject = issuerCert.Subject
	case workloadidentityv1pb.CSRCreationMode_CSR_CREATION_MODE_UNSPECIFIED:
		return nil, trace.BadParameter("missing csr_creation_mode")
	default:
		return nil, trace.BadParameter("unknown csr_creation_mode %d", mode)
	}

	return &x509.CertificateRequest{
		Subject: subject,

		DNSNames:       []string(nil),
		EmailAddresses: []string(nil),
		IPAddresses:    []net.IP(nil),
		URIs:           []*url.URL(nil),

		ExtraExtensions: []pkix.Extension(nil),
	}, nil
}

// GetX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) GetX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.GetX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.authorizeAccessToKind(ctx, types.KindWorkloadIdentityX509IssuerOverride, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.storage.GetX509IssuerOverride(ctx, req.GetName())
}

// ListX509IssuerOverrides implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) ListX509IssuerOverrides(ctx context.Context, req *workloadidentityv1pb.ListX509IssuerOverridesRequest) (*workloadidentityv1pb.ListX509IssuerOverridesResponse, error) {
	if err := s.authorizeAccessToKind(ctx, types.KindWorkloadIdentityX509IssuerOverride, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	overrides, nextPageToken, err := s.storage.ListX509IssuerOverrides(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return workloadidentityv1pb.ListX509IssuerOverridesResponse_builder{
		X509IssuerOverrides: overrides,
		NextPageToken:       nextPageToken,
	}.Build(), nil
}

// CreateX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) CreateX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.CreateX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return nil, trace.Wrap(ErrOverrideDeprecated)
}

// UpdateX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) UpdateX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.UpdateX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return nil, trace.Wrap(ErrOverrideDeprecated)
}

// UpsertX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) UpsertX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.UpsertX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return nil, trace.Wrap(ErrOverrideDeprecated)
}

// DeleteX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) DeleteX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.DeleteX509IssuerOverrideRequest) (*emptypb.Empty, error) {
	if err := s.authorizeAccessToKindAdminReusedMFA(ctx, types.KindWorkloadIdentityX509IssuerOverride, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteX509IssuerOverride(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityX509IssuerOverrideDelete{
		Metadata: apievents.Metadata{
			Type: events.WorkloadIdentityX509IssuerOverrideDeleteEvent,
			Code: events.WorkloadIdentityX509IssuerOverrideDeleteCode,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.GetName(),
		},
	})

	return &emptypb.Empty{}, nil
}

func chainSlices[T any](s1, s2 []T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, v := range s1 {
			if !yield(i, v) {
				return
			}
		}
		for i, v := range s2 {
			if !yield(i, v) {
				return
			}
		}
	}
}
