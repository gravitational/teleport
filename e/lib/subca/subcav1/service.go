// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package subcav1

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/subca"
)

// CachedSubCAStorage is a subset of SubCAService containing read methods that
// may use cached results.
//
// See lib/services/local.SubCAService.
type CachedSubCAStorage interface {
	GetCertAuthorityOverride(
		ctx context.Context, id local.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error)
	ListCertAuthorityOverrides(
		ctx context.Context,
		pageSize int,
		pageToken string,
	) (_ []*subcav1.CertAuthorityOverride, nextPageToken string, _ error)
}

// SubCAStorage is the storage implementation of SubCAService.
//
// See lib/services/local.SubCAService.
type SubCAStorage interface {
	CreateCertAuthorityOverride(
		ctx context.Context,
		resource *subcav1.CertAuthorityOverride,
	) (*subcav1.CertAuthorityOverride, error)
	DeleteCertAuthorityOverride(
		ctx context.Context,
		id local.CertAuthorityOverrideID,
	) error
}

// KeystoreManager is a subset of keystore.Manager methods used in CRL and CSR
// generation.
//
// See lib/auth/keystore.Manager.
type KeystoreManager interface {
	TLSSigner(ctx context.Context, keypair *types.TLSKeyPair) (crypto.Signer, error)
}

// ServiceParams holds creation parameters for [Service].
type ServiceParams struct {
	Logger *slog.Logger

	CachedClusterNameGetter services.ClusterNameGetter
	// CachedSubCA is a cached Sub CA storage service.
	// Used by read-only RPC.
	CachedSubCA CachedSubCAStorage
	// CachedTrust is a cached Trust storage service.
	CachedTrust services.AuthorityGetter
	// SubCA is a non-cached Sub CA storage service.
	// Used by write RPCs.
	SubCA SubCAStorage

	// KeystoreManager is the interface to the Auth TLS private keys.
	// Used on CRL and CSR signing.
	KeystoreManager KeystoreManager

	Authorizer authz.Authorizer
	Emitter    apievents.Emitter
}

// Service implements the teleport.subca.v1.SubCAService RPC service.
type Service struct {
	subcav1.UnimplementedSubCAServiceServer

	logger *slog.Logger

	cachedClusterNameGetter services.ClusterNameGetter
	cachedSubCA             CachedSubCAStorage
	cachedTrust             services.AuthorityGetter
	subCA                   SubCAStorage

	keystoreManager KeystoreManager

	authorizer authz.Authorizer
	emitter    apievents.Emitter
}

// New creates a new [Service].
func New(p ServiceParams) (*Service, error) {
	switch {
	case p.Logger == nil:
		return nil, trace.BadParameter("param Logger required")
	case p.CachedClusterNameGetter == nil:
		return nil, trace.BadParameter("param CachedClusterNameGetter required")
	case p.CachedSubCA == nil:
		return nil, trace.BadParameter("param CachedSubCA required")
	case p.CachedTrust == nil:
		return nil, trace.BadParameter("param CachedTrust required")
	case p.SubCA == nil:
		return nil, trace.BadParameter("param SubCA required")
	case p.KeystoreManager == nil:
		return nil, trace.BadParameter("param KeystoreManager required")
	case p.Authorizer == nil:
		return nil, trace.BadParameter("param Authorizer required")
	case p.Emitter == nil:
		return nil, trace.BadParameter("param Emitter required")
	}

	return &Service{
		logger:                  p.Logger,
		cachedClusterNameGetter: p.CachedClusterNameGetter,
		cachedSubCA:             p.CachedSubCA,
		cachedTrust:             p.CachedTrust,
		subCA:                   p.SubCA,
		keystoreManager:         p.KeystoreManager,
		authorizer:              p.Authorizer,
		emitter:                 p.Emitter,
	}, nil
}

func (s *Service) CreateCSR(
	ctx context.Context,
	req *subcav1.CreateCSRRequest,
) (*subcav1.CreateCSRResponse, error) {
	if req.CaType == "" {
		return nil, trace.BadParameter("ca_type required")
	}
	if req.PublicKeyHash != nil {
		return nil, trace.BadParameter("public_key_hash not implemented")
	}
	if req.CustomSubject != nil {
		return nil, trace.BadParameter("custom_subject not implemented")
	}
	if err := s.authorizeCAOverride(
		ctx, adminActionNotNeeded, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	// Read cluster name.
	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}

	// Read CA.
	ca, err := s.cachedTrust.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.CertAuthType(req.CaType),
		DomainName: cn.GetClusterName(),
	}, true /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err, "read CA")
	}

	// Prepare CA signers, as many as possible for this Auth instance.
	candidateSigners, err := s.getCandidateCSRSigners(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "prepare signers")
	}
	if len(candidateSigners) == 0 {
		return nil, trace.BadParameter(
			"cannot create CSRs for certificate authority, Auth lacks access to private keys")
	}

	resp := &subcav1.CreateCSRResponse{
		Csrs: make([]*subcav1.CertificateSigningRequest, 0, len(candidateSigners)),
	}
	for _, candidateSigner := range candidateSigners {
		signer := candidateSigner.Signer
		cert := candidateSigner.Cert

		// Construct Subject from the matching CA certificate.
		subj := pkix.Name{
			ExtraNames: cert.Subject.Names,
		}
		// Remove serial number (OID 2.5.4.5).
		subj.ExtraNames = removeOID(subj.ExtraNames, []int{2, 5, 4, 5})

		// Create CSR.
		certReq := &x509.CertificateRequest{
			PublicKey: signer.Public(),
			Subject:   subj,
		}
		csrDER, err := x509.CreateCertificateRequest(rand.Reader, certReq, signer)
		if err != nil {
			return nil, trace.Wrap(err, "create certificate request")
		}
		csrPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE REQUEST",
			Bytes: csrDER,
		})

		resp.Csrs = append(resp.Csrs, &subcav1.CertificateSigningRequest{
			Pem: string(csrPEM),
		})
	}

	return resp, nil
}

type candidateCSRSigner struct {
	Signer crypto.Signer
	Cert   *x509.Certificate
}

func (s *Service) getCandidateCSRSigners(
	ctx context.Context,
	ca types.CertAuthority,
) ([]*candidateCSRSigner, error) {
	activeTLS := ca.GetActiveKeys().TLS
	additionalTLS := ca.GetAdditionalTrustedKeys().TLS

	lenAllKeys := len(activeTLS) + len(additionalTLS)
	if lenAllKeys == 0 {
		return nil, trace.BadParameter(
			"certificate authority has no active or additional keys")
	}

	resp := make([]*candidateCSRSigner, 0, lenAllKeys)

	// Attempt to create as many signers/CSRs as we can, from both active and
	// additional key sets.
	for i, kps := range [][]*types.TLSKeyPair{
		activeTLS,
		additionalTLS,
	} {
		isActive := i == 0
		for j, kp := range kps {
			signer, err := s.keystoreManager.TLSSigner(ctx, kp)
			switch {
			case errors.Is(err, keystore.ErrUnusableKey):
				s.logger.DebugContext(ctx,
					"Skipping unusable keypair during CSR generation",
					"is_active", isActive,
					"index", j,
				)
				continue
			case err != nil:
				return nil, trace.Wrap(err, "create signer (is_active=%v, index=%d)", isActive, j)
			}

			cert, err := tlsutils.ParseCertificatePEM(kp.Cert)
			if err != nil {
				return nil, trace.Wrap(err, "parse CA certificate (is_active=%v, index=%d)", isActive, j)
			}

			resp = append(resp, &candidateCSRSigner{
				Signer: signer,
				Cert:   cert,
			})
		}
	}

	// TODO(codingllama): Notify user of keys that could not be used?

	return resp, nil
}

func (s *Service) CreateCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.CreateCertAuthorityOverrideRequest,
) (*subcav1.CreateCertAuthorityOverrideResponse, error) {
	switch {
	case req.CaOverride.GetMetadata().GetName() == "":
		return nil, trace.BadParameter("ca_override.metadata.name required")
	case req.CaOverride.GetSubKind() == "":
		return nil, trace.BadParameter("ca_override.sub_kind required")
	}
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	parsed, err := subca.ValidateAndParseCAOverride(req.CaOverride)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Only allow overrides for the current cluster.
	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}
	if cn.GetClusterName() != parsed.CAOverride.Metadata.Name {
		return nil, trace.BadParameter(
			"invalid metadata.name/clusterName: %q, only %q is allowed",
			parsed.CAOverride.Metadata.Name, cn.GetClusterName(),
		)
	}

	// TODO(codingllama): Validate against CA resource.

	// TODO(codingllama): Create CRLs.

	created, err := s.subCA.CreateCertAuthorityOverride(ctx, parsed.CAOverride)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitCAOverrideEvent(ctx,
		parsed,
		nil, // err
		events.CertAuthOverrideCreateEvent,
		events.CertAuthOverrideCreateCode,
	)

	return &subcav1.CreateCertAuthorityOverrideResponse{
		CaOverride: created,
	}, nil
}

func (s *Service) GetCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.GetCertAuthorityOverrideRequest,
) (*subcav1.GetCertAuthorityOverrideResponse, error) {
	if req.CaId.GetCaType() == "" {
		return nil, trace.BadParameter("ca_id.ca_type required")
	}
	if err := s.authorizeCAOverride(ctx, adminActionNotNeeded, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}
	caOverride, err := s.cachedSubCA.GetCertAuthorityOverride(ctx, local.CertAuthorityOverrideID{
		ClusterName: cn.GetClusterName(),
		CAType:      req.CaId.CaType,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &subcav1.GetCertAuthorityOverrideResponse{
		CaOverride: caOverride,
	}, nil
}

func (s *Service) ListCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.ListCertAuthorityOverrideRequest,
) (*subcav1.ListCertAuthorityOverrideResponse, error) {
	if err := s.authorizeCAOverride(
		ctx, adminActionNotNeeded, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	caOverrides, nextPageToken, err := s.cachedSubCA.ListCertAuthorityOverrides(
		ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &subcav1.ListCertAuthorityOverrideResponse{
		CaOverrides:   caOverrides,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *Service) DeleteCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.DeleteCertAuthorityOverrideRequest,
) (*subcav1.DeleteCertAuthorityOverrideResponse, error) {
	if req.CaId.GetCaType() == "" {
		return nil, trace.BadParameter("ca_id.ca_type required")
	}
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(codingllama): Validate against enabled overrides of the existing CA
	//  override resource.

	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}
	id := local.CertAuthorityOverrideID{
		ClusterName: cn.GetClusterName(),
		CAType:      req.CaId.CaType,
	}
	if err := s.subCA.DeleteCertAuthorityOverride(ctx, id); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fill in identifying fields for audit.
	parsed := &subca.ParsedCertAuthorityOverride{
		CAOverride: &subcav1.CertAuthorityOverride{
			SubKind: id.CAType,
			Metadata: &headerv1.Metadata{
				Name: id.ClusterName,
			},
		},
	}
	s.emitCAOverrideEvent(
		ctx,
		parsed,
		nil, // error
		events.CertAuthOverrideDeleteEvent,
		events.CertAuthOverrideDeleteCode,
	)

	return &subcav1.DeleteCertAuthorityOverrideResponse{}, nil
}

type adminActionMode int

const (
	adminActionYes adminActionMode = iota // stricter mode first, err on strict
	adminActionNotNeeded
)

func (s *Service) authorizeCAOverride(
	ctx context.Context,
	adminMode adminActionMode,
	verb string,
	additionalVerbs ...string,
) error {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(
		types.KindCertAuthorityOverride, verb, additionalVerbs...); err != nil {
		return trace.Wrap(err)
	}
	if adminMode == adminActionNotNeeded {
		return nil
	}

	return trace.Wrap(authzCtx.AuthorizeAdminActionAllowReusedMFA())
}
