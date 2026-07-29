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
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/subca"
	"github.com/gravitational/teleport/lib/utils/log"
)

var errNoCSRs = &trace.BadParameterError{
	Message: "cannot create CSRs for certificate authority, Auth lacks access to private keys",
}

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
	GetCertAuthorityOverride(
		ctx context.Context, id local.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error)
	CreateCertAuthorityOverride(
		ctx context.Context,
		resource *subcav1.CertAuthorityOverride,
	) (*subcav1.CertAuthorityOverride, error)
	UpdateCertAuthorityOverride(
		ctx context.Context,
		resource *subcav1.CertAuthorityOverride,
	) (*subcav1.CertAuthorityOverride, error)
	UpsertCertAuthorityOverride(
		ctx context.Context,
		resource *subcav1.CertAuthorityOverride,
	) (*subcav1.CertAuthorityOverride, error)
	DeleteCertAuthorityOverride(
		ctx context.Context,
		id local.CertAuthorityOverrideID,
	) error
	ConditionalDeleteCertAuthorityOverride(
		ctx context.Context,
		id local.CertAuthorityOverrideID,
		revision string,
	) error
}

// PendingCSRStorage is the storage interface for PendingCSRRequest resources.
//
// See lib/services/local.SubCAService.
type PendingCSRStorage interface {
	GetPendingCSRRequest(
		ctx context.Context,
		name string,
	) (*subcav1.PendingCSRRequest, error)
	ListPendingCSRRequests(
		ctx context.Context,
		pageSize int,
		pageToken string,
	) (_ []*subcav1.PendingCSRRequest, nextPageToken string, _ error)
	CreatePendingCSRRequest(
		ctx context.Context,
		resource *subcav1.PendingCSRRequest,
	) (*subcav1.PendingCSRRequest, error)
	UpdatePendingCSRRequest(
		ctx context.Context,
		resource *subcav1.PendingCSRRequest,
	) (*subcav1.PendingCSRRequest, error)
	DeletePendingCSRRequest(ctx context.Context, name string) error
}

// KeystoreManager is a subset of keystore.Manager methods used in CRL and CSR
// generation.
//
// See lib/auth/keystore.Manager.
type KeystoreManager interface {
	TLSSigner(ctx context.Context, keypair *types.TLSKeyPair) (crypto.Signer, error)
}

// WatcherSource is a source of watchers, typically a lib/cache.Cache instance
// in production.
type WatcherSource interface {
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

// ServiceParams holds creation parameters for [Service].
type ServiceParams struct {
	Clock  clockwork.Clock
	Logger *slog.Logger

	// CachedClusterNameGetter is a cached ClusterNameGetter.
	CachedClusterNameGetter services.ClusterNameGetter
	// CachedSubCA is a cached Sub CA storage service.
	// Used by read-only RPC.
	CachedSubCA CachedSubCAStorage
	// SubCA is a non-cached Sub CA storage service.
	// Used by write RPCs.
	SubCA SubCAStorage
	// PendingCSR is the PendingCSRRequest storage service.
	PendingCSR PendingCSRStorage
	// Trust is the Trust storage service.
	// A non-cached trust is used so CSRs and lateral validation are always
	// executed against fresh data. (Writes are infrequent enough that it's not a
	// burden)
	// Used by CSR generation and write RPCs.
	Trust services.AuthorityGetter

	// WatcherContext is the context used to stop the background watchers created
	// by Service.
	WatcherContext context.Context
	// WatcherSource is a source of watchers.
	// Used to listen and react to CA override changes.
	WatcherSource WatcherSource

	// KeystoreManager is the interface to the Auth TLS private keys.
	// Used on CRL and CSR signing.
	KeystoreManager KeystoreManager

	Authorizer authz.Authorizer
	Emitter    apievents.Emitter
}

// Service implements the teleport.subca.v1.SubCAService RPC service.
type Service struct {
	subcav1.UnimplementedSubCAServiceServer

	clock  clockwork.Clock
	logger *slog.Logger

	cachedClusterNameGetter services.ClusterNameGetter
	cachedSubCA             CachedSubCAStorage
	subCA                   SubCAStorage
	pendingCSR              PendingCSRStorage
	trust                   services.AuthorityGetter

	watcherSource WatcherSource

	keystoreManager KeystoreManager

	authorizer authz.Authorizer
	emitter    apievents.Emitter
}

// New creates a new [Service].
func New(p ServiceParams) (*Service, error) {
	switch {
	case p.Clock == nil:
		return nil, trace.BadParameter("param Clock required")
	case p.Logger == nil:
		return nil, trace.BadParameter("param Logger required")
	case p.CachedClusterNameGetter == nil:
		return nil, trace.BadParameter("param CachedClusterNameGetter required")
	case p.CachedSubCA == nil:
		return nil, trace.BadParameter("param CachedSubCA required")
	case p.SubCA == nil:
		return nil, trace.BadParameter("param SubCA required")
	case p.PendingCSR == nil:
		return nil, trace.BadParameter("param PendingCSR required")
	case p.Trust == nil:
		return nil, trace.BadParameter("param Trust required")
	case p.WatcherContext == nil:
		return nil, trace.BadParameter("param WatcherContext required")
	case p.WatcherSource == nil:
		return nil, trace.BadParameter("param WatcherSource required")
	case p.KeystoreManager == nil:
		return nil, trace.BadParameter("param KeystoreManager required")
	case p.Authorizer == nil:
		return nil, trace.BadParameter("param Authorizer required")
	case p.Emitter == nil:
		return nil, trace.BadParameter("param Emitter required")
	}

	logger := p.Logger.With(teleport.ComponentKey, "subca.service")

	s := &Service{
		clock:                   p.Clock,
		logger:                  logger,
		cachedClusterNameGetter: p.CachedClusterNameGetter,
		cachedSubCA:             p.CachedSubCA,
		subCA:                   p.SubCA,
		pendingCSR:              p.PendingCSR,
		trust:                   p.Trust,
		watcherSource:           p.WatcherSource,
		keystoreManager:         p.KeystoreManager,
		authorizer:              p.Authorizer,
		emitter:                 p.Emitter,
	}
	go s.runSubCAWatcher(p.WatcherContext)
	return s, nil
}

func (s *Service) CreateCSR(
	ctx context.Context,
	req *subcav1.CreateCSRRequest,
) (*subcav1.CreateCSRResponse, error) {
	if err := validateCATypeForCSR(req.GetCaType()); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.HasPublicKeyHash() && req.GetPublicKeyHash().GetValue() == "" {
		return nil, trace.BadParameter("public_key_hash invalid: %q", req.GetPublicKeyHash())
	}
	if err := s.authorizeCAOverride(ctx, adminActionNotNeeded, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Read cluster name.
	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}

	// Parse custom Subject.
	var customSubject []pkix.AttributeTypeAndValue
	if req.HasCustomSubject() {
		rdns, err := subca.DistinguishedNameProtoToRDNSequence(req.GetCustomSubject())
		if err != nil {
			return nil, trace.Wrap(err, "custom subject")
		}
		customSubject = flattenRDNSequence(rdns)

		// Assign ClusterName to custom Subject.
		customSubject, err = assignClusterNameToATVs(customSubject, cn.GetClusterName())
		if err != nil {
			return nil, trace.Wrap(err, "assign cluster name to custom subject")
		}
	}

	// Read CA.
	ca, err := s.trust.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.CertAuthType(req.GetCaType()),
		DomainName: cn.GetClusterName(),
	}, true /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err, "read CA")
	}

	// Prepare CA signers, as many as possible for this Auth instance.
	candidateResp, err := s.getCandidateCSRSigners(ctx, ca, req.GetPublicKeyHash().GetValue())
	if err != nil {
		return nil, trace.Wrap(err, "prepare signers")
	}
	// Early abort local requests if we can't sign anything.
	if req.GetLocalOnly() && len(candidateResp.Signers) == 0 {
		return nil, trace.Wrap(errNoCSRs)
	}
	totalCSRs := len(candidateResp.Signers) + len(candidateResp.MissingPublicKeys)
	if len(customSubject) > 0 && totalCSRs > 1 {
		return nil, trace.BadParameter("" +
			"requests with a custom subject cannot match more than one certificate, " +
			"use public key hash to match a single certificate")
	}

	var respBuilder subcav1.CreateCSRResponse_builder

	// Sign as many CSRs as we can with the current Auth.
	respBuilder.Csrs, err = createCSRs(candidateResp.Signers, customSubject)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(candidateResp.MissingPublicKeys) > 0 {
		if req.GetLocalOnly() {
			respBuilder.Warnings = appendMissingKeyWarnings(
				respBuilder.Warnings,
				candidateResp.MissingPublicKeys,
				"private key inaccessible for this Auth server instance",
			)
		} else {
			// Request async signing of missing keys.
			id := types.CertAuthorityOverrideID{
				ClusterName: cn.GetClusterName(),
				CAType:      req.GetCaType(),
			}
			remoteCSRs, warnings, err := s.requestAsyncCSRs(ctx, id, customSubject, candidateResp.MissingPublicKeys)
			if err != nil {
				s.logger.WarnContext(ctx, "Async CSR request attempt failed. Only local CSRs will be returned",
					"error", err,
					"ca_type", id.CAType,
					"cluster_name", id.ClusterName,
				)
				respBuilder.Warnings = appendMissingKeyWarnings(
					respBuilder.Warnings,
					candidateResp.MissingPublicKeys,
					"remote signing request errored or timed out",
				)
			} else {
				respBuilder.Csrs = append(respBuilder.Csrs, remoteCSRs...)
				respBuilder.Warnings = append(respBuilder.Warnings, warnings...)
			}
		}
	}

	// Don't reply with zero CSRs.
	if len(respBuilder.Csrs) == 0 {
		return nil, trace.Wrap(errNoCSRs)
	}

	return respBuilder.Build(), nil
}

func validateCATypeForCSR(caType string) error {
	if caType == "" {
		return trace.BadParameter("ca_type required")
	}
	if allowedTypes := subca.SupportedCATypes(); !slices.Contains(allowedTypes, caType) {
		allowedTypesJoined := strings.Join(allowedTypes, ", ")
		return trace.BadParameter("ca_type not allowed: %q (must be one of %s)", caType, allowedTypesJoined)
	}
	return nil
}

func flattenRDNSequence(rdns pkix.RDNSequence) []pkix.AttributeTypeAndValue {
	// Pre-allocate. This is a guess, actual size could be larger.
	atvs := make([]pkix.AttributeTypeAndValue, 0, len(rdns))
	// Flatten.
	for _, set := range rdns {
		for _, atv := range set {
			atvs = append(atvs, atv)
		}
	}
	return atvs
}

func createCSRs(
	candidateSigners []*candidateCSRSigner,
	customSubject []pkix.AttributeTypeAndValue,
) ([]*subcav1.CertificateSigningRequest, error) {
	csrs := make([]*subcav1.CertificateSigningRequest, 0, len(candidateSigners))
	for _, candidateSigner := range candidateSigners {
		csr, err := createCSR(candidateSigner, customSubject)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		csrs = append(csrs, csr)
	}
	return csrs, nil
}

func createCSR(
	candidateSigner *candidateCSRSigner,
	customSubject []pkix.AttributeTypeAndValue,
) (*subcav1.CertificateSigningRequest, error) {
	signer := candidateSigner.Signer
	cert := candidateSigner.Cert

	// Subject.
	var subj pkix.Name
	if len(customSubject) > 0 {
		subj.ExtraNames = customSubject
	} else {
		subj.ExtraNames = cert.Subject.Names
		// Remove serial number (OID 2.5.4.5).
		subj.ExtraNames = removeOID(subj.ExtraNames, []int{2, 5, 4, 5})
	}

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

	return subcav1.CertificateSigningRequest_builder{
		Pem: string(csrPEM),
	}.Build(), nil
}

func appendMissingKeyWarnings(
	warns []*subcav1.CreateCSRWarning,
	publicKeys []string,
	userMessage string,
) []*subcav1.CreateCSRWarning {
	warns = slices.Grow(warns, len(publicKeys))
	for _, pkh := range publicKeys {
		warns = append(warns, subcav1.CreateCSRWarning_builder{
			UserMessage:   userMessage,
			PublicKeyHash: pkh,
		}.Build())
	}
	return warns
}

type candidateCSRSignerResponse struct {
	// Signers holds the (successfully created) signer+certificate pairs.
	Signers []*candidateCSRSigner
	// MissingPublicKeys records the public key hashes for signers that could not
	// be created. It's expected that other Auth server instances are able to use
	// the missing keys.
	MissingPublicKeys []string
}

type candidateCSRSigner struct {
	Signer crypto.Signer
	Cert   *x509.Certificate
}

func (s *Service) getCandidateCSRSigners(
	ctx context.Context,
	ca types.CertAuthority,
	publicKeyHash string,
) (*candidateCSRSignerResponse, error) {
	activeTLS := ca.GetActiveKeys().TLS
	additionalTLS := ca.GetAdditionalTrustedKeys().TLS

	lenAllKeys := len(activeTLS) + len(additionalTLS)
	if lenAllKeys == 0 {
		return nil, trace.BadParameter(
			"certificate authority has no active or additional keys")
	}

	// Make sure comparisons are case-insensitive.
	publicKeyHash = subca.NormalizePublicKey(publicKeyHash)

	pkhPresent := publicKeyHash != ""
	var pkhMatched bool

	numSigners := lenAllKeys
	if pkhPresent {
		numSigners = 1
	}
	signers := make([]*candidateCSRSigner, 0, numSigners)
	var missingPublicKeys []string

	// Attempt to create as many signers/CSRs as we can, from both active and
	// additional key sets.
	for i, kps := range [][]*types.TLSKeyPair{
		activeTLS,
		additionalTLS,
	} {
		isActive := i == 0
		for j, kp := range kps {
			cert, err := tlsutils.ParseCertificatePEM(kp.Cert)
			if err != nil {
				return nil, trace.Wrap(err, "parse CA certificate (is_active=%v, index=%d)", isActive, j)
			}
			pkhCurrent := subca.HashCertificatePublicKey(cert)

			// Apply publicKeyHash filter.
			if pkhPresent && publicKeyHash != pkhCurrent {
				continue
			}
			pkhMatched = pkhPresent

			signer, err := s.keystoreManager.TLSSigner(ctx, kp)
			switch {
			case errors.Is(err, keystore.ErrUnusableKey):
				missingPublicKeys = append(missingPublicKeys, pkhCurrent)
				s.logger.Log(ctx, log.TraceLevel,
					"Skipping unusable keypair during CSR generation",
					"is_active", isActive,
					"index", j,
				)
				continue
			case err != nil:
				return nil, trace.Wrap(err, "create signer (is_active=%v, index=%d)", isActive, j)
			}

			signers = append(signers, &candidateCSRSigner{
				Signer: signer,
				Cert:   cert,
			})
		}
	}
	if pkhPresent && !pkhMatched {
		return nil, trace.BadParameter("public_key_hash %q matches no CA certificates", publicKeyHash)
	}

	return &candidateCSRSignerResponse{
		Signers:           signers,
		MissingPublicKeys: missingPublicKeys,
	}, nil
}

func (s *Service) CreateCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.CreateCertAuthorityOverrideRequest,
) (_ *subcav1.CreateCertAuthorityOverrideResponse, err error) {
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	audit := s.newAuditWriter(
		events.CertAuthOverrideCreateEvent,
		events.CertAuthOverrideCreateCode,
		req.GetCaOverride(),
	)
	defer func() { audit.emitAuditEvent(ctx, err) }()

	created, err := s.writeCAOverride(ctx, writeCAOverrideParams{
		mode:          writeCreate,
		audit:         audit,
		newCAOverride: req.GetCaOverride(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return subcav1.CreateCertAuthorityOverrideResponse_builder{
		CaOverride: created,
	}.Build(), nil
}

func (s *Service) UpdateCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.UpdateCertAuthorityOverrideRequest,
) (_ *subcav1.UpdateCertAuthorityOverrideResponse, err error) {
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	audit := s.newAuditWriter(
		events.CertAuthOverrideUpdateEvent,
		events.CertAuthOverrideUpdateCode,
		req.GetCaOverride(),
	)
	defer func() { audit.emitAuditEvent(ctx, err) }()

	updated, err := s.writeCAOverride(ctx, writeCAOverrideParams{
		mode:                  writeUpdate,
		audit:                 audit,
		newCAOverride:         req.GetCaOverride(),
		forceImmediateDisable: req.GetForceImmediateDisable(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return subcav1.UpdateCertAuthorityOverrideResponse_builder{
		CaOverride: updated,
	}.Build(), nil
}

func (s *Service) UpsertCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.UpsertCertAuthorityOverrideRequest,
) (_ *subcav1.UpsertCertAuthorityOverrideResponse, err error) {
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	audit := s.newAuditWriter(
		events.CertAuthOverrideUpsertEvent,
		events.CertAuthOverrideUpsertCode,
		req.GetCaOverride(),
	)
	defer func() { audit.emitAuditEvent(ctx, err) }()

	updated, err := s.writeCAOverride(ctx, writeCAOverrideParams{
		mode:                  writeUpsert,
		audit:                 audit,
		newCAOverride:         req.GetCaOverride(),
		forceImmediateDisable: req.GetForceImmediateDisable(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return subcav1.UpsertCertAuthorityOverrideResponse_builder{
		CaOverride: updated,
	}.Build(), nil
}

func (s *Service) AddCertificateOverride(
	ctx context.Context,
	req *subcav1.AddCertificateOverrideRequest,
) (_ *subcav1.AddCertificateOverrideResponse, err error) {
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	audit := s.newAuditWriter(
		// Assume the operation is a create to begin with.
		events.CertAuthOverrideCreateEvent,
		events.CertAuthOverrideCreateCode,
		// Start with a basic representation of the override we intend to write.
		// It gets updated as we query and parse the override.
		subcav1.CertAuthorityOverride_builder{
			SubKind:  req.GetCaId().GetCaType(),
			Metadata: &headerv1.Metadata{},
			Spec: subcav1.CertAuthorityOverrideSpec_builder{
				CertificateOverrides: []*subcav1.CertificateOverride{
					req.GetCertificateOverride(),
				},
			}.Build(),
		}.Build(),
	)
	defer func() { audit.emitAuditEvent(ctx, err) }()

	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	audit.caOverride.GetMetadata().SetName(cn.GetClusterName())

	switch {
	case req.GetCaId().GetCaType() == "":
		return nil, trace.BadParameter("ca_id.ca_type required")
	case !req.HasCertificateOverride():
		return nil, trace.BadParameter("certificate_override required")
	}

	// Discover if the underlying operation is a create or update.
	id := local.CertAuthorityOverrideID{
		ClusterName: cn.GetClusterName(),
		CAType:      req.GetCaId().GetCaType(),
	}
	var mode writeMode
	var newCAOverride *subcav1.CertAuthorityOverride
	existingCAOverride, err := s.subCA.GetCertAuthorityOverride(ctx, id)
	switch {
	case err == nil:
		mode = writeUpdate
		audit.eventType = events.CertAuthOverrideUpdateEvent
		audit.eventCode = events.CertAuthOverrideUpdateCode
		// Append new CO. Validation ensures the same public key can't be targeted
		// twice.
		newCAOverride = proto.CloneOf(existingCAOverride)
		newCAOverride.GetSpec().SetCertificateOverrides(append(
			newCAOverride.GetSpec().GetCertificateOverrides(), req.GetCertificateOverride()))
	case trace.IsNotFound(err):
		mode = writeCreate
		newCAOverride = makeCAOverrideForCertificate(
			id.CAType,
			id.ClusterName,
			req.GetCertificateOverride(),
		)
	default:
		return nil, trace.Wrap(err)
	}
	audit.caOverride = newCAOverride

	// Create or update.
	if _, err := s.writeCAOverride(ctx, writeCAOverrideParams{
		mode:          mode,
		audit:         audit,
		newCAOverride: newCAOverride,
		// Adds can never be an enabled-to-disabled transition, but --force skips CA
		// lateral validation as well. To be consistent with other RPCs we pass it
		// along.
		forceImmediateDisable: req.GetForceImmediateDisable(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return subcav1.AddCertificateOverrideResponse_builder{
		CertificateOverride: req.GetCertificateOverride(),
	}.Build(), nil
}

func makeCAOverrideForCertificate(
	caType, clusterName string,
	co *subcav1.CertificateOverride,
) *subcav1.CertAuthorityOverride {
	return subcav1.CertAuthorityOverride_builder{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: caType,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: clusterName,
		}.Build(),
		Spec: subcav1.CertAuthorityOverrideSpec_builder{
			CertificateOverrides: []*subcav1.CertificateOverride{
				co,
			},
		}.Build(),
	}.Build()
}

func (s *Service) UpdateCertificateOverride(
	ctx context.Context,
	req *subcav1.UpdateCertificateOverrideRequest,
) (_ *subcav1.UpdateCertificateOverrideResponse, err error) {
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	audit := s.newAuditWriter(
		events.CertAuthOverrideUpdateEvent,
		events.CertAuthOverrideUpdateCode,
		// Start with a basic representation of the override we intend to write.
		// It gets updated as we query and parse the override.
		subcav1.CertAuthorityOverride_builder{
			SubKind:  req.GetCaId().GetCaType(),
			Metadata: &headerv1.Metadata{},
			Spec: subcav1.CertAuthorityOverrideSpec_builder{
				CertificateOverrides: []*subcav1.CertificateOverride{
					req.GetCertificateOverride(),
				},
			}.Build(),
		}.Build(),
	)
	defer func() { audit.emitAuditEvent(ctx, err) }()

	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	audit.caOverride.GetMetadata().SetName(cn.GetClusterName())

	switch {
	case req.GetCaId().GetCaType() == "":
		return nil, trace.BadParameter("ca_id.ca_type required")
	case !req.HasCertificateOverride():
		return nil, trace.BadParameter("certificate_override required")
	case len(req.GetUpdateMask().GetPaths()) == 0:
		return nil, trace.BadParameter("update_mask required")
	}
	if err := validateCertificateOverrideMask(req.GetUpdateMask()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Optimistically determine the target public key hash.
	// The new override is validated by writeCAOverride.
	co := req.GetCertificateOverride()
	pkh := co.GetPublicKey()
	if pkh == "" {
		// Derive public key from the certificate field IF the "certificate" field
		// mask is present, but otherwise be well-behaved and ignore it.
		if !slices.Contains(req.GetUpdateMask().GetPaths(), "certificate") || co.GetCertificate() == "" {
			return nil, trace.BadParameter("public_key is required to target the certificate_override")
		}
		cert, err := subca.ParseCertificateOverrideCertificate(co.GetCertificate())
		if err != nil {
			return nil, trace.Wrap(err, "certificate_override.certificate")
		}
		pkh = subca.HashCertificatePublicKey(cert)
	}

	// Find the override to update.
	parsed, index, err := s.findCertificateOverride(
		ctx,
		cn.GetClusterName(),
		req.GetCaId().GetCaType(),
		pkh,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	audit.setParsed(parsed)

	// Apply field mask.
	current := parsed.CAOverride.GetSpec().GetCertificateOverrides()[index]
	if err := applyCertificateOverrideMask(current, co, req.GetUpdateMask()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Update.
	if _, err := s.writeCAOverride(ctx, writeCAOverrideParams{
		mode:                  writeUpdate,
		audit:                 audit,
		newCAOverride:         parsed.CAOverride,
		forceImmediateDisable: req.GetForceImmediateDisable(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return subcav1.UpdateCertificateOverrideResponse_builder{
		CertificateOverride: current,
	}.Build(), nil
}

func validateCertificateOverrideMask(mask *fieldmaskpb.FieldMask) error {
	dummy := &subcav1.CertificateOverride{}
	return applyCertificateOverrideMask(dummy, dummy, mask)
}

func applyCertificateOverrideMask(dst, src *subcav1.CertificateOverride, mask *fieldmaskpb.FieldMask) error {
	for _, path := range mask.GetPaths() {
		switch path {
		case "public_key":
			dst.SetPublicKey(src.GetPublicKey())
		case "certificate":
			dst.SetCertificate(src.GetCertificate())
		case "chain":
			dst.SetChain(src.GetChain())
		case "disabled":
			dst.SetDisabled(src.GetDisabled())
		default:
			return trace.BadParameter("update_mask: invalid path %q", path)
		}
	}
	return nil
}

func (s *Service) RemoveCertificateOverride(
	ctx context.Context,
	req *subcav1.RemoveCertificateOverrideRequest,
) (_ *subcav1.RemoveCertificateOverrideResponse, err error) {
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbUpdate, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	audit := s.newAuditWriter(
		// Assume the operation is an update to begin with.
		events.CertAuthOverrideUpdateEvent,
		events.CertAuthOverrideUpdateCode,
		// Start with a basic representation of the override we intend to write.
		// It gets updated as we query and parse the override.
		subcav1.CertAuthorityOverride_builder{
			SubKind:  req.GetCertificateOverrideId().GetCaType(),
			Metadata: &headerv1.Metadata{},
			Spec: subcav1.CertAuthorityOverrideSpec_builder{
				CertificateOverrides: []*subcav1.CertificateOverride{
					subcav1.CertificateOverride_builder{
						PublicKey: req.GetCertificateOverrideId().GetPublicKeyHash().GetValue(),
					}.Build(),
				},
			}.Build(),
		}.Build(),
	)
	defer func() { audit.emitAuditEvent(ctx, err) }()

	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	audit.caOverride.GetMetadata().SetName(cn.GetClusterName())

	switch {
	case req.GetCertificateOverrideId().GetCaType() == "":
		return nil, trace.BadParameter("certificate_override_id.ca_type required")
	case req.GetCertificateOverrideId().GetPublicKeyHash().GetValue() == "":
		return nil, trace.BadParameter("certificate_override_id.public_key_hash required")
	}

	parsed, index, err := s.findCertificateOverride(
		ctx,
		cn.GetClusterName(),
		req.GetCertificateOverrideId().GetCaType(),
		req.GetCertificateOverrideId().GetPublicKeyHash().GetValue())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	audit.setParsed(parsed)

	// Escalate to Delete if this is the last certificate override in the spec.
	if len(parsed.CertificateOverrides) == 1 {
		audit.eventType = events.CertAuthOverrideDeleteEvent
		audit.eventCode = events.CertAuthOverrideDeleteCode
		const unconditional = false
		err := s.deleteCAOverride(ctx, parsed, req.GetForceImmediateDelete(), unconditional)
		return &subcav1.RemoveCertificateOverrideResponse{}, trace.Wrap(err)
	}

	// Remove target override.
	overrides := parsed.CAOverride.GetSpec().GetCertificateOverrides()
	parsed.CAOverride.GetSpec().SetCertificateOverrides(
		slices.Delete(overrides, index, index+1),
	)

	// Update.
	_, err = s.writeCAOverride(ctx, writeCAOverrideParams{
		mode:                  writeUpdate,
		audit:                 audit,
		newCAOverride:         parsed.CAOverride,
		forceImmediateDisable: req.GetForceImmediateDelete(),
	})
	return &subcav1.RemoveCertificateOverrideResponse{}, trace.Wrap(err)
}

func (s *Service) findCertificateOverride(
	ctx context.Context,
	clusterName string,
	caType string,
	publicKeyHash string,
) (parsed *subca.ParsedCertAuthorityOverride, certificateOverrideIndex int, _ error) {
	id := local.CertAuthorityOverrideID{
		ClusterName: clusterName,
		CAType:      caType,
	}
	caOverride, err := s.subCA.GetCertAuthorityOverride(ctx, id)
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}
	parsed, err = subca.ParseCAOverride(caOverride)
	if err != nil {
		return nil, 0, trace.Wrap(err, "parse existing override")
	}

	// Make sure comparisons are case-insensitive.
	publicKeyHash = subca.NormalizePublicKey(publicKeyHash)

	for i, parsedCO := range parsed.CertificateOverrides {
		if parsedCO.PublicKey == publicKeyHash {
			return parsed, i, nil
		}
	}

	return nil, 0, trace.CompareFailed("certificate override not found: %q", publicKeyHash)
}

type writeMode int

const (
	writeCreate writeMode = iota + 1
	writeUpdate
	writeUpsert
)

type writeCAOverrideParams struct {
	mode                  writeMode
	audit                 *auditWriter
	newCAOverride         *subcav1.CertAuthorityOverride
	forceImmediateDisable bool
}

func (s *Service) writeCAOverride(
	ctx context.Context,
	params writeCAOverrideParams,
) (_ *subcav1.CertAuthorityOverride, err error) {
	switch {
	case params.newCAOverride.GetMetadata().GetName() == "":
		return nil, trace.BadParameter("ca_override.metadata.name required")
	case params.newCAOverride.GetSubKind() == "":
		return nil, trace.BadParameter("ca_override.sub_kind required")
	}

	parsed, err := subca.ValidateAndParseCAOverride(params.newCAOverride)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	params.audit.setParsed(parsed)

	// Only allow overrides for the current cluster.
	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}
	if cn.GetClusterName() != parsed.CAOverride.GetMetadata().GetName() {
		return nil, trace.BadParameter(
			"invalid metadata.name/clusterName: %q, only %q is allowed",
			parsed.CAOverride.GetMetadata().GetName(), cn.GetClusterName(),
		)
	}

	// Prepare CA read.
	// Needed for lateral validation and to issue CRLs.
	// In some scenarios we don't actually need to read the CA (forced updates
	// with known CRLs, creates/updates with empty overrides, etc), so the read is
	// lazy.
	const loadKeys = true // Necessary for new CRLs.
	getParsedCA := s.getParsedCAOnce(ctx, types.CertAuthID{
		Type:       types.CertAuthType(parsed.CAOverride.GetSubKind()),
		DomainName: cn.GetClusterName(),
	}, loadKeys)

	// Read existing CA override.
	// Needed for lateral validation and to consolidate the Status field.
	mode := params.mode
	var existingCAOverride *subcav1.CertAuthorityOverride
	if mode != writeCreate {
		id := local.CertAuthorityOverrideIDFromResource(parsed.CAOverride)
		var err error
		existingCAOverride, err = s.subCA.GetCertAuthorityOverride(ctx, id)
		switch {
		case mode == writeUpsert && err != nil && trace.IsNotFound(err):
			// OK, new resource.
		case err != nil:
			return nil, trace.Wrap(err, "read existing override")
		case mode == writeUpdate && parsed.CAOverride.GetMetadata().GetRevision() != existingCAOverride.GetMetadata().GetRevision():
			// Don't bother continuing, doomed to fail.
			return nil, trace.Wrap(backend.ErrIncorrectRevision)
		}
	}

	// If --force is set we allow overrides to be created as long as they are
	// internally consistent (ValidateAndParseCAOverride passes), regardless of
	// the system state.
	// This allows "tctl create -f" (and similar) to reconstruct a previous
	// system state.
	//
	// The system still needs the CRLs in Status to function, so either the caller
	// provides the old Status (which we do accept), or we must be able to
	// re-create the necessary CRLs.
	if !params.forceImmediateDisable {
		if err := s.performWriteLateralValidation(getParsedCA, parsed, existingCAOverride); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	status := subcav1.CertAuthorityOverrideStatus_builder{
		PublicKeyHashToCrl: make(map[string]*subcav1.CertificateRevocationList),
	}.Build()
	// Combine CRLs from existing and new overrides. createOverrideCRLs trims the
	// list as necessary.
	// We'll give the new CA override precedence, so it's possible to use a CRL
	// from a previously stored resource.
	maps.Copy(status.GetPublicKeyHashToCrl(), existingCAOverride.GetStatus().GetPublicKeyHashToCrl())
	// Input keys are normalized to lowercase for trivial comparison.
	for k, v := range parsed.CAOverride.GetStatus().GetPublicKeyHashToCrl() {
		status.GetPublicKeyHashToCrl()[subca.NormalizePublicKey(k)] = v
	}
	parsed.CAOverride.SetStatus(status)

	if err := s.createOverrideCRLs(ctx, getParsedCA, parsed); err != nil {
		return nil, trace.Wrap(err)
	}

	var updated *subcav1.CertAuthorityOverride
	switch mode {
	case writeCreate:
		s.logger.DebugContext(ctx, "CA override write in Create mode")
		updated, err = s.subCA.CreateCertAuthorityOverride(ctx, parsed.CAOverride)
	case writeUpdate:
		s.logger.DebugContext(ctx, "CA override write in Update mode")
		updated, err = s.subCA.UpdateCertAuthorityOverride(ctx, parsed.CAOverride)
	case writeUpsert:
		s.logger.DebugContext(ctx, "CA override write in Upsert mode")
		updated, err = s.subCA.UpsertCertAuthorityOverride(ctx, parsed.CAOverride)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return updated, nil
}

// Write lateral validation checks the new CA override against its sibling CA
// resource, as well against a possibly-existing CA override, in an attempt to
// prevent a multitude of invalid, ineffective or dangerous changes.
func (s *Service) performWriteLateralValidation(
	getParsedCA getParsedCAFunc,
	parsedNew *subca.ParsedCertAuthorityOverride,
	existingCAOverride *subcav1.CertAuthorityOverride,
) error {
	// Read/parse CA.
	parsedCA, err := getParsedCA()
	if err != nil {
		return trace.Wrap(err)
	}

	// Parse existing CA override.
	var parsedExisting *subca.ParsedCertAuthorityOverride
	if existingCAOverride != nil {
		var err error
		parsedExisting, err = subca.ParseCAOverride(existingCAOverride)
		if err != nil {
			return trace.Wrap(err, "parse existing override")
		}
	}

	// Prepare data for cross-validation.
	correlatedData := correlateOverrides(parsedNew, parsedExisting)

	// Note: We can't truly keep validation invariants on Upsert, as it could be
	// racing and doing something ungainly (like disabling an enabled override).
	// Either we follow Upsert invariants, or validation invariants. For
	// consistency with the rest of the Teleport we choose the former.
	//
	// (We can't keep validation invariants against CAs either, as they can
	// change independently from their overrides.)
	return trace.Wrap(
		validateCAOverrideAgainstSystemState(parsedCA, correlatedData),
	)
}

type getParsedCAFunc func() (*parsedCertAuthority, error)

func (s *Service) getParsedCAOnce(ctx context.Context, id types.CertAuthID, loadKeys bool) getParsedCAFunc {
	return sync.OnceValues(func() (*parsedCertAuthority, error) {
		ca, err := s.trust.GetCertAuthority(ctx, id, loadKeys)
		if err != nil {
			return nil, trace.Wrap(err, "read CA")
		}
		parsed, err := parseCA(ctx, ca)
		if err != nil {
			return nil, trace.Wrap(err, "parse CA")
		}
		return parsed, nil
	})
}

// correlateOverrideData is created by comparing 2 versions of the same CA
// override resource.
//
// See [correlateOverrides].
type correlateOverrideData struct {
	New     []*subca.ParsedCertificateOverride
	Updated []*correlateOverrideCertificateData
	Deleted []*subca.ParsedCertificateOverride
}

type correlateOverrideCertificateData struct {
	*subca.ParsedCertificateOverride
	// IsDisable is true if the existing override is considered disabled by the
	// new.
	IsDisable bool
	// IsEnable is true if the existing override is considered enabled by the
	// new.
	IsEnable bool
}

func correlateOverrides(newCA, existingCA *subca.ParsedCertAuthorityOverride) *correlateOverrideData {
	data := &correlateOverrideData{}

	// OK, happens when creating new resources. This means all overrides are New.
	if existingCA == nil {
		data.New = newCA.CertificateOverrides
		return data
	}

	// Discover New and Updated.
	seenOverrides := make(map[string]struct{})
	for _, overrideNew := range newCA.CertificateOverrides {
		seenOverrides[overrideNew.PublicKey] = struct{}{}
		found := false
		for _, overrideExisting := range existingCA.CertificateOverrides {
			if overrideNew.PublicKey == overrideExisting.PublicKey {
				isDisable := !overrideExisting.CertificateOverride.GetDisabled() &&
					overrideNew.CertificateOverride.GetDisabled()
				isEnable := overrideExisting.CertificateOverride.GetDisabled() &&
					!overrideNew.CertificateOverride.GetDisabled()
				data.Updated = append(data.Updated, &correlateOverrideCertificateData{
					ParsedCertificateOverride: overrideNew,
					IsDisable:                 isDisable,
					IsEnable:                  isEnable,
				})
				found = true
				break
			}
		}
		if !found {
			data.New = append(data.New, overrideNew)
		}
	}

	// Discover Deleted.
	for _, overrideExisting := range existingCA.CertificateOverrides {
		if _, ok := seenOverrides[overrideExisting.PublicKey]; !ok {
			data.Deleted = append(data.Deleted, overrideExisting)
		}
	}

	return data
}

// Validate the CA override against the CA resource, per rules below:
//
//  1. Override certificates may not expire after the self-signed CA
//     certificate.
//  2. New certificate overrides must target known CA certificates.
//  3. Disables are only allowed if a) forced or b) the override doesn't target
//     a known active certificate. This is true for both Updates and Deletes.
//  4. Existing certificate overrides may be interacted with, regardless of
//     matching an existing CA certificate. CAs may change independently of
//     their overrides. If that happens the user should still be able to
//     interact with the now-obsolete override.
//  5. Existing certificate overrides cannot be enabled if they target an
//     unknown CA certificate. This is special case of (4).
func validateCAOverrideAgainstSystemState(
	parsedCA *parsedCertAuthority,
	correlatedData *correlateOverrideData,
) error {
	validateTargetAndBounds := func(co *subca.ParsedCertificateOverride, allowUnknown bool) error {
		c1, ok1 := parsedCA.ActiveKeyHashes[co.PublicKey]
		c2, ok2 := parsedCA.AdditionalKeyHashes[co.PublicKey]
		var caCert *x509.Certificate
		switch {
		case ok1:
			caCert = c1
		case ok2:
			caCert = c2
		case allowUnknown:
			return nil // OK.
		default:
			return trace.BadParameter("certificate override %q targets unknown CA certificate", co.PublicKey)
		}

		if co.Certificate != nil && co.Certificate.NotAfter.After(caCert.NotAfter) {
			return trace.BadParameter(
				"certificate override %q expires after self-signed CA certificate, that is not allowed: %v > %v",
				co.PublicKey,
				co.Certificate.NotAfter,
				caCert.NotAfter,
			)
		}
		return nil
	}

	for _, co := range correlatedData.New {
		// Validate bounds (1) and known target (2).
		const allowUnknown = false
		if err := validateTargetAndBounds(co, allowUnknown); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, co := range correlatedData.Updated {
		// Validate bounds (1), allow unknown (4) if not enabling (5).
		allowUnknown := !co.IsEnable
		if err := validateTargetAndBounds(co.ParsedCertificateOverride, allowUnknown); err != nil {
			return trace.Wrap(err)
		}
		// Validate disable (3).
		if !co.IsDisable {
			continue
		}
		if _, isActive := parsedCA.ActiveKeyHashes[co.PublicKey]; isActive {
			return trace.BadParameter(
				"Attempt to disable enabled override %q of active certificate denied. Retry with --force if you are certain.",
				co.PublicKey,
			)
		}
	}

	for _, co := range correlatedData.Deleted {
		// Validate disable (3).
		if co.CertificateOverride.GetDisabled() {
			continue
		}
		if _, isActive := parsedCA.ActiveKeyHashes[co.PublicKey]; isActive {
			return trace.BadParameter(
				"Attempt to delete enabled override %q of active certificate denied. Disable the override or retry with --force if you are certain.",
				co.PublicKey)
		}
	}

	return nil
}

func (s *Service) GetCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.GetCertAuthorityOverrideRequest,
) (*subcav1.GetCertAuthorityOverrideResponse, error) {
	if req.GetCaId().GetCaType() == "" {
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
		CAType:      req.GetCaId().GetCaType(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return subcav1.GetCertAuthorityOverrideResponse_builder{
		CaOverride: caOverride,
	}.Build(), nil
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
		ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return subcav1.ListCertAuthorityOverrideResponse_builder{
		CaOverrides:   caOverrides,
		NextPageToken: nextPageToken,
	}.Build(), nil
}

func (s *Service) DeleteCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.DeleteCertAuthorityOverrideRequest,
) (_ *subcav1.DeleteCertAuthorityOverrideResponse, err error) {
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	audit := s.newAuditWriter(
		events.CertAuthOverrideDeleteEvent,
		events.CertAuthOverrideDeleteCode,
		// Start with a basic representation of the override we intend to delete.
		// It gets updated as/if we query and parse the override.
		subcav1.CertAuthorityOverride_builder{
			SubKind:  req.GetCaId().GetCaType(),
			Metadata: &headerv1.Metadata{},
		}.Build(),
	)
	defer func() { audit.emitAuditEvent(ctx, err) }()

	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}
	audit.caOverride.GetMetadata().SetName(cn.GetClusterName())

	if req.GetCaId().GetCaType() == "" {
		return nil, trace.BadParameter("ca_id.ca_type required")
	}

	var parsed *subca.ParsedCertAuthorityOverride
	force := req.GetForceImmediateDelete()
	if force {
		// Don't query or parse the override in force mode, proceed to an
		// unconditional delete.
		// This lets deletes recover the system from corrupted states.
		parsed = &subca.ParsedCertAuthorityOverride{
			CAOverride: subcav1.CertAuthorityOverride_builder{
				SubKind: req.GetCaId().GetCaType(),
				Metadata: headerv1.Metadata_builder{
					Name: cn.GetClusterName(),
				}.Build(),
			}.Build(),
		}
	} else {
		caOverride, err := s.subCA.GetCertAuthorityOverride(ctx, local.CertAuthorityOverrideID{
			ClusterName: cn.GetClusterName(),
			CAType:      req.GetCaId().GetCaType(),
		})
		if err != nil {
			return nil, trace.Wrap(err, "read CA override")
		}
		parsed, err = subca.ParseCAOverride(caOverride)
		if err != nil {
			return nil, trace.Wrap(err, "parse CA override")
		}
	}
	audit.setParsed(parsed)

	unconditional := force
	if err := s.deleteCAOverride(ctx, parsed, force, unconditional); err != nil {
		return nil, trace.Wrap(err)
	}

	return &subcav1.DeleteCertAuthorityOverrideResponse{}, nil
}

func (s *Service) deleteCAOverride(
	ctx context.Context,
	parsed *subca.ParsedCertAuthorityOverride,
	forceImmediateDelete bool,
	unconditional bool,
) error {
	// Skip disable validation on forced deletes.
	// Disables are always allowed if forced.
	if !forceImmediateDelete {
		if err := s.performDeleteLateralValidation(ctx, parsed); err != nil {
			return trace.Wrap(err)
		}
	}

	id := local.CertAuthorityOverrideIDFromResource(parsed.CAOverride)
	if unconditional {
		if err := s.subCA.DeleteCertAuthorityOverride(ctx, id); err != nil {
			return trace.Wrap(err)
		}
	} else {
		revision := parsed.CAOverride.GetMetadata().GetRevision()
		if err := s.subCA.ConditionalDeleteCertAuthorityOverride(ctx, id, revision); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// Delete lateral validation checks the to-be-deleted CA override against its
// sibling CA resource and existing CA override, making sure enabled and active
// overrides aren't being deleted.
func (s *Service) performDeleteLateralValidation(
	ctx context.Context,
	parsed *subca.ParsedCertAuthorityOverride,
) error {
	caType := parsed.CAOverride.GetSubKind()
	clusterName := parsed.CAOverride.GetMetadata().GetName()

	// Read CA.
	const loadKeys = false
	ca, err := s.trust.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.CertAuthType(caType),
		DomainName: clusterName,
	}, loadKeys)
	if err != nil {
		return trace.Wrap(err, "read CA")
	}
	parsedCA, err := parseCA(ctx, ca)
	if err != nil {
		return trace.Wrap(err, "parse CA")
	}

	correlatedData := &correlateOverrideData{
		Deleted: parsed.CertificateOverrides, // all about to be deleted
	}

	// Validate disables.
	return trace.Wrap(
		validateCAOverrideAgainstSystemState(parsedCA, correlatedData),
	)
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
