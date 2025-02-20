// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"iter"
	"log/slog"
	"net"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apitypes "github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

type CertAuthorityGetter interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
}

type X509OverridesServiceConfig struct {
	Authorizer          authz.Authorizer
	CertAuthorityGetter CertAuthorityGetter
	Emitter             apievents.Emitter
	KeyStore            KeyStorer
	Logger              *slog.Logger
	Storage             services.WorkloadIdentityX509Overrides

	ClusterName string

	Clock clockwork.Clock
}

func NewX509OverridesService(cfg X509OverridesServiceConfig) (*X509OverridesService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.CertAuthorityGetter == nil {
		return nil, trace.BadParameter("cert authority getter is required")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required")
	}
	if cfg.KeyStore == nil {
		return nil, trace.BadParameter("key store is required")
	}
	if cfg.Logger == nil {
		return nil, trace.BadParameter("logger is required")
	}
	if cfg.Storage == nil {
		return nil, trace.BadParameter("storage is required")
	}

	if cfg.ClusterName == "" {
		return nil, trace.BadParameter("cluster name is required")
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &X509OverridesService{
		authorizer:          cfg.Authorizer,
		certAuthorityGetter: cfg.CertAuthorityGetter,
		clock:               cfg.Clock,
		emitter:             cfg.Emitter,
		keyStore:            cfg.KeyStore,
		logger:              cfg.Logger,
		storage:             cfg.Storage,

		clusterName: cfg.ClusterName,
	}, nil
}

type X509OverridesService struct {
	workloadidentityv1pb.UnsafeX509OverridesServiceServer

	authorizer          authz.Authorizer
	certAuthorityGetter CertAuthorityGetter
	clock               clockwork.Clock
	emitter             apievents.Emitter
	keyStore            KeyStorer
	logger              *slog.Logger
	storage             services.WorkloadIdentityX509Overrides

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

func (s *X509OverridesService) requireEnterprise() error {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return trace.AccessDenied("SPIFFE X.509 issuer override is only available with an enterprise license")
	}
	return nil
}

// SignX509IssuerCSR implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) SignX509IssuerCSR(ctx context.Context, req *workloadidentityv1pb.SignX509IssuerCSRRequest) (*workloadidentityv1pb.SignX509IssuerCSRResponse, error) {
	if err := s.requireEnterprise(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverrideCSR, apitypes.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	const loadKeysTrue = true
	ca, err := s.certAuthorityGetter.GetCertAuthority(ctx, apitypes.CertAuthID{
		Type:       apitypes.SPIFFECA,
		DomainName: s.clusterName,
	}, loadKeysTrue)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyPair := s.searchIssuerInCertAuthority(ca, req.GetIssuer())
	if keyPair == nil {
		return nil, trace.NotFound("issuer not found in SPIFFE CA")
	}
	issuerCert, err := x509.ParseCertificate(req.GetIssuer())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(espadolini): this is sort of a hack but this synchronous API is also
	// sort of a hack anyway
	ca.SetActiveKeys(apitypes.CAKeySet{TLS: []*apitypes.TLSKeyPair{keyPair}})

	_, signer, err := s.keyStore.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: issuerCert.Subject,

		DNSNames:       []string(nil),
		EmailAddresses: []string(nil),
		IPAddresses:    []net.IP(nil),
		URIs:           []*url.URL(nil),

		ExtraExtensions: []pkix.Extension(nil),
	}, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadidentityv1pb.SignX509IssuerCSRResponse{
		Csr: csr,
	}, nil
}

func (*X509OverridesService) searchIssuerInCertAuthority(ca apitypes.CertAuthority, issuerDER []byte) *apitypes.TLSKeyPair {
	for _, kp := range chain(
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

// GetX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) GetX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.GetX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.storage.GetX509IssuerOverride(ctx, req.GetName())
}

// ListX509IssuerOverrides implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) ListX509IssuerOverrides(ctx context.Context, req *workloadidentityv1pb.ListX509IssuerOverridesRequest) (*workloadidentityv1pb.ListX509IssuerOverridesResponse, error) {
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbList, apitypes.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	overrides, nextPageToken, err := s.storage.ListX509IssuerOverrides(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadidentityv1pb.ListX509IssuerOverridesResponse{
		X509IssuerOverrides: overrides,
		NextPageToken:       nextPageToken,
	}, nil
}

// CreateX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) CreateX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.CreateX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.requireEnterprise(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	resource := req.GetX509IssuerOverride()
	if _, err := services.ParseWorkloadIdentityX509IssuerOverride(resource); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.storage.CreateX509IssuerOverride(ctx, resource)
}

// UpdateX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) UpdateX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.UpdateX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.requireEnterprise(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	resource := req.GetX509IssuerOverride()
	if _, err := services.ParseWorkloadIdentityX509IssuerOverride(resource); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.storage.UpdateX509IssuerOverride(ctx, resource)
}

// UpsertX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) UpsertX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.UpsertX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.requireEnterprise(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbCreate, apitypes.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	resource := req.GetX509IssuerOverride()
	if _, err := services.ParseWorkloadIdentityX509IssuerOverride(resource); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.storage.UpsertX509IssuerOverride(ctx, req.GetX509IssuerOverride())
}

// DeleteX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) DeleteX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.DeleteX509IssuerOverrideRequest) (*emptypb.Empty, error) {
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, s.storage.DeleteX509IssuerOverride(ctx, req.GetName())
}

func chain[T any, S1, S2 ~[]T](s1 S1, s2 S2) iter.Seq2[int, T] {
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
