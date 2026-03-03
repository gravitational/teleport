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

package local

import (
	"cmp"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/subca"
)

// CA override keys are in the format
// `/cert_authority_overrides/c/{clusterName}/{caType}`.
//
// CA override resources mimic CA resources, which means:
//   - ClusterName is recorded at ca.Resource.Name.
//   - CAType is recorded at ca.SubKind.
var caOverridesPrefix = backend.NewKey("cert_authority_overrides", "c")

var (
	allowedCAOverrideSubKinds = []string{
		string(types.DatabaseClientCA),
		string(types.WindowsCA),
	}

	// Public keys are printed as HEX(SHA256(...)), therefore it's a hex string
	// with exactly 64 characters. See subca.PublicKeyHash.
	certificateOverridePublicKeyRE = regexp.MustCompile(`^[0-9A-Fa-f]{64}$`)
)

// CertAuthorityOverrideID uniquely identifies a CertAuthorityOverride resource.
type CertAuthorityOverrideID struct {
	ClusterName string
	CAType      string
}

// CertAuthorityOverrideIDFromResource returns the id of the specified resource.
//
// If the resource is nil or missing components, then the corresponding ID
// fields will be empty.
func CertAuthorityOverrideIDFromResource(r *subcav1.CertAuthorityOverride) CertAuthorityOverrideID {
	return CertAuthorityOverrideID{
		ClusterName: r.GetMetadata().GetName(),
		CAType:      r.GetSubKind(),
	}
}

// SubCAServiceParams holds creation parameters for [SubCAService].
type SubCAServiceParams struct {
	Backend backend.Backend
	Trust   services.AuthorityGetter
}

// SubCAService manages backend storage of CertAuthorityOverride resources.
//
// Follows RFD 153 / generic.Service semantics.
type SubCAService struct {
	service *generic.ServiceWrapper[*subcav1.CertAuthorityOverride]
	trust   services.AuthorityGetter
}

// NewSubCAService creates a new service using the provided params.
func NewSubCAService(p SubCAServiceParams) (*SubCAService, error) {
	if p.Trust == nil {
		return nil, trace.BadParameter("trust service required")
	}

	service, err := generic.NewServiceWrapper(generic.ServiceConfig[*subcav1.CertAuthorityOverride]{
		Backend:       p.Backend,
		ResourceKind:  types.KindCertAuthorityOverride,
		BackendPrefix: caOverridesPrefix,
		MarshalFunc:   services.MarshalCertAuthorityOverride,
		UnmarshalFunc: services.UnmarshalCertAuthorityOverride,
		ValidateFunc: func(*subcav1.CertAuthorityOverride) error {
			// Validation applied manually at each write method.
			return nil
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SubCAService{
		service: service,
		trust:   p.Trust,
	}, nil
}

// CreateCertAuthorityOverride creates a CA override in the backend.
func (s *SubCAService) CreateCertAuthorityOverride(
	ctx context.Context,
	resource *subcav1.CertAuthorityOverride,
) (*subcav1.CertAuthorityOverride, error) {
	if err := s.validateCAOverrideForCreate(ctx, resource); err != nil {
		return nil, trace.Wrap(err)
	}

	service, err := s.serviceForResource(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(codingllama): Create CRLs.

	// TODO(codingllama): Take a condition on the sibling CA resource.
	//  We optimistically skip this for now: CAs can change independently anyway
	//  so they can always become "out of sync" with overrides.
	created, err := service.CreateResource(ctx, resource)
	return created, trace.Wrap(err)
}

func (s *SubCAService) validateCAOverrideForCreate(
	ctx context.Context,
	resource *subcav1.CertAuthorityOverride,
) error {
	parsed, err := validateCAOverride(resource)
	if err != nil {
		return trace.Wrap(err)
	}

	// Fetch corresponding CA.
	ca, err := s.trust.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.CertAuthType(resource.SubKind),
		DomainName: resource.Metadata.Name,
	}, false /* loadSigningKeys */)
	if err != nil {
		return trace.Wrap(err, "read CA resource")
	}
	knownActiveKeys := hashCAPublicKeys(ca.GetActiveKeys().TLS, ca)
	knownAdditionalKeys := hashCAPublicKeys(ca.GetAdditionalTrustedKeys().TLS, ca)

	// Validate overrides against CA certificates.
	for i, co := range parsed.certificateOverrides {
		_, isActive := knownActiveKeys[co.publicKey]
		_, isAdditional := knownAdditionalKeys[co.publicKey]
		if !isActive && !isAdditional {
			return trace.BadParameter(
				"spec.certificate_overrides[%d]: override targets unknown CA certificate", i)
		}
	}

	return nil
}

// GetCertAuthorityOverride reads a CA override from the backend.
func (s *SubCAService) GetCertAuthorityOverride(
	ctx context.Context,
	id CertAuthorityOverrideID,
) (*subcav1.CertAuthorityOverride, error) {
	service, err := s.serviceForClusterAndType(id.ClusterName, id.CAType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource, err := service.GetResource(ctx, "")
	return resource, trace.Wrap(err)
}

func (s *SubCAService) serviceForResource(
	resource *subcav1.CertAuthorityOverride,
) (*generic.ServiceWrapper[*subcav1.CertAuthorityOverride], error) {
	return s.serviceForClusterAndType(
		resource.GetMetadata().GetName(),
		resource.GetSubKind(),
	)
}

func (s *SubCAService) serviceForClusterAndType(
	clusterName string,
	caType string,
) (*generic.ServiceWrapper[*subcav1.CertAuthorityOverride], error) {
	switch {
	case clusterName == "":
		return nil, trace.BadParameter("resource name/clusterName required")
	case caType == "":
		return nil, trace.BadParameter("resource sub_kind/caType required")
	}

	return s.service.WithNameKeyFunc(func(_ string) backend.Key {
		return backend.NewKey(clusterName, caType)
	}), nil
}

func hashCAPublicKeys(
	keys []*types.TLSKeyPair,
	caForLogging types.CertAuthority,
) map[string]struct{} {
	publicKeys := make(map[string]struct{})
	for i, key := range keys {
		cert, err := tlsutils.ParseCertificatePEM(key.Cert)
		if err != nil {
			slog.WarnContext(context.Background(),
				"Failed to parse CA TLS certificate",
				"cluster_name", caForLogging.GetClusterName(),
				"ca_type", caForLogging.GetType(),
				"index", i,
			)
			continue
		}
		pub := subca.HashCertificatePublicKey(cert)
		publicKeys[pub] = struct{}{}
	}
	return publicKeys
}

type parsedResource struct {
	caOverride           *subcav1.CertAuthorityOverride
	certificateOverrides []*parsedCertificateOverride
}

type parsedCertificateOverride struct {
	certificateOverride *subcav1.CertificateOverride
	publicKey           string
	certificate         *x509.Certificate
}

func validateCAOverride(resource *subcav1.CertAuthorityOverride) (*parsedResource, error) {
	switch {
	case resource == nil:
		return nil, trace.BadParameter("resource required")
	case resource.Kind != types.KindCertAuthorityOverride:
		return nil, trace.BadParameter("invalid kind: %q", resource.Kind)
	case !slices.Contains(allowedCAOverrideSubKinds, resource.SubKind):
		return nil, trace.BadParameter("invalid or unsupported sub_kind/caType: %q", resource.SubKind)
	case resource.Version != types.V1:
		return nil, trace.BadParameter("invalid or unsupported version: %q", resource.Version)
	case resource.Metadata == nil:
		return nil, trace.BadParameter("metadata required")
	case resource.Metadata.Name == "":
		return nil, trace.BadParameter("metadata.name/clusterName required")
	case resource.Spec == nil:
		return nil, trace.BadParameter("spec required")
	}

	overrides := resource.Spec.CertificateOverrides
	parsedOverrides := make([]*parsedCertificateOverride, len(overrides))
	seenPublicKeys := make(map[string]struct{})
	for i, co := range overrides {
		parsedCO, fieldName, err := validateCertificateOverride(co)
		if err != nil {
			if fieldName != "" {
				fieldName = "." + fieldName
			}
			return nil, trace.BadParameter("spec.certificate_overrides[%d]%s: %v", i, fieldName, err)
		}

		if _, ok := seenPublicKeys[parsedCO.publicKey]; ok {
			return nil, trace.BadParameter(
				"spec.certificate_overrides[%d]: found duplicate override for public key %q",
				i, parsedCO.publicKey,
			)
		}
		seenPublicKeys[parsedCO.publicKey] = struct{}{}

		parsedOverrides[i] = parsedCO
	}

	return &parsedResource{
		caOverride:           resource,
		certificateOverrides: parsedOverrides,
	}, nil
}

func validateCertificateOverride(
	co *subcav1.CertificateOverride,
) (_ *parsedCertificateOverride, fieldName string, _ error) {
	// Trace not used on purpose. Errors are trace-wrapped up in the chain.
	if co == nil {
		return nil, "", errors.New("nil certificate override")
	}

	// Certificate.
	var cert *x509.Certificate
	var wantPublicKey string
	if co.Certificate != "" {
		var err error
		cert, err = subca.ParseCertificateOverrideCertificate(co.Certificate)
		if err != nil {
			return nil, "certificate", err
		}
		wantPublicKey = subca.HashCertificatePublicKey(cert)
	}

	// PublicKey.
	if co.PublicKey != "" {
		if !certificateOverridePublicKeyRE.MatchString(co.PublicKey) {
			return nil, "", errors.New("invalid public key")
		}
		if wantPublicKey != "" && !strings.EqualFold(co.PublicKey, wantPublicKey) {
			return nil, "public_key", fmt.Errorf("certificate public key mismatch (want %q)", wantPublicKey)
		}
	}

	// Validate "required" fields now that we know both Certificate and PublicKey
	// are valid.
	switch {
	case co.Disabled && co.PublicKey == "" && co.Certificate == "":
		return nil, "", errors.New("certificate or public key required")
	case co.Disabled:
		// OK, determined above to have either PublicKey or Certificate.
	case co.Certificate == "":
		return nil, "", errors.New("certificate required")
	}

	// Chain.
	if len(co.Chain) > 0 {
		if cert == nil {
			return nil, "", errors.New("chain not allowed with an empty certificate")
		}

		// The exact number is arbitrary, the fact that a cap exists isn't.
		const maxChainLength = 10
		if len(co.Chain) > maxChainLength {
			return nil, "chain", fmt.Errorf(
				"certificate chain has too many entries (%d > %d)", len(co.Chain), maxChainLength)
		}

		overrideIssuer := cert.Issuer.String()
		overrideSub := cert.Subject.String()
		nextSubj := overrideIssuer
		for i, chainPEM := range co.Chain {
			chainCert, err := subca.ParseCertificateOverrideCertificate(chainPEM)
			if err != nil {
				return nil, fmt.Sprintf("chain[%d]", i), err
			}

			chainSub := chainCert.Subject.String()
			if i == 0 && chainSub == overrideSub {
				return nil, fmt.Sprintf("chain[%d]", i),
					errors.New("override certificate should not be included in chain")
			}

			// Chain MUST be from leaf to root.
			if chainSub != nextSubj {
				return nil, fmt.Sprintf("chain[%d]", i),
					fmt.Errorf("chain out of order, subject=%q (want %q)", chainSub, nextSubj)
			}

			// TODO(codingllama): Check chain signatures in addition to Subject/Issuer
			//  relationships.

			nextSubj = chainCert.Issuer.String()
		}
	}

	// Normalize public key to lowercase so it matches subca.HashPublicKey.
	publicKey := cmp.Or(wantPublicKey, co.PublicKey)
	publicKey = strings.ToLower(publicKey)

	return &parsedCertificateOverride{
		certificateOverride: co,
		publicKey:           publicKey,
		certificate:         cert,
	}, "", nil
}
