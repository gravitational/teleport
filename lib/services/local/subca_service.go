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
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/subca"
)

// newCAOverridesPrefix returns the CertAuthorityOverride storage prefix key.
//
// CA override keys are in the format
// `/cert_authority_overrides/cluster/{clusterName}/{caType}`.
//
// CA override resources mimic CA resources, which means:
//   - ClusterName is recorded at ca.Resource.Name.
//   - CAType is recorded at ca.SubKind.
func newCAOverridesPrefix() backend.Key {
	return backend.NewKey("cert_authority_overrides", "cluster")
}

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
}

// SubCAService manages backend storage of CertAuthorityOverride resources.
//
// Follows RFD 153 / generic.Service semantics.
type SubCAService struct {
	service *generic.ServiceWrapper[*subcav1.CertAuthorityOverride]
}

// NewSubCAService creates a new service using the provided params.
func NewSubCAService(p SubCAServiceParams) (*SubCAService, error) {
	service, err := generic.NewServiceWrapper(generic.ServiceConfig[*subcav1.CertAuthorityOverride]{
		Backend:       p.Backend,
		ResourceKind:  types.KindCertAuthorityOverride,
		BackendPrefix: newCAOverridesPrefix(),
		MarshalFunc:   services.MarshalCertAuthorityOverride,
		UnmarshalFunc: services.UnmarshalCertAuthorityOverride,
		ValidateFunc:  validateCAOverride,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SubCAService{
		service: service,
	}, nil
}

// CreateCertAuthorityOverride creates a CA override in the backend.
func (s *SubCAService) CreateCertAuthorityOverride(
	ctx context.Context,
	resource *subcav1.CertAuthorityOverride,
) (*subcav1.CertAuthorityOverride, error) {
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

	return s.service.WithNameKeyFunc(func() backend.Key {
		return backend.NewKey(clusterName, caType)
	}), nil
}

func validateCAOverride(resource *subcav1.CertAuthorityOverride) error {
	switch {
	case resource == nil:
		return trace.BadParameter("resource required")
	case resource.Kind != types.KindCertAuthorityOverride:
		return trace.BadParameter("invalid kind: %q", resource.Kind)
	case !slices.Contains(allowedCAOverrideSubKinds, resource.SubKind):
		return trace.BadParameter("invalid or unsupported sub_kind/caType: %q", resource.SubKind)
	case resource.Version != types.V1:
		return trace.BadParameter("invalid or unsupported version: %q", resource.Version)
	case resource.Metadata == nil:
		return trace.BadParameter("metadata required")
	case resource.Metadata.Name == "":
		return trace.BadParameter("metadata.name/clusterName required")
	case resource.Spec == nil:
		return trace.BadParameter("spec required")
	}

	overrides := resource.Spec.CertificateOverrides
	seenPublicKeys := make(map[string]struct{})
	for i, co := range overrides {
		parsedCO, fieldName, err := validateCertificateOverride(co)
		if err != nil {
			if fieldName != "" {
				fieldName = "." + fieldName
			}
			return trace.BadParameter("spec.certificate_overrides[%d]%s: %v", i, fieldName, err)
		}

		if _, ok := seenPublicKeys[parsedCO.publicKey]; ok {
			return trace.BadParameter(
				"spec.certificate_overrides[%d]: found duplicate override for public key %q",
				i, parsedCO.publicKey,
			)
		}
		seenPublicKeys[parsedCO.publicKey] = struct{}{}
	}
	return nil
}

type parsedCertificateOverride struct {
	certificateOverride *subcav1.CertificateOverride
	publicKey           string
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

		prev := cert
		for i, chainPEM := range co.Chain {
			chainCert, err := subca.ParseCertificateOverrideCertificate(chainPEM)
			if err != nil {
				return nil, fmt.Sprintf("chain[%d]", i), err
			}
			chainSub := chainCert.Subject.String()

			// Certificate not in chain.
			if i == 0 && cert.Subject.String() == chainSub {
				return nil, fmt.Sprintf("chain[%d]", i),
					errors.New("override certificate should not be included in chain")
			}

			// Issuer/Subject relationship.
			if issuer := prev.Issuer.String(); issuer != chainSub {
				return nil, fmt.Sprintf("chain[%d]", i),
					fmt.Errorf("chain out of order, subject=%q (want %q)", chainSub, issuer)
			}

			// Verify signature.
			if err := prev.CheckSignatureFrom(chainCert); err != nil {
				return nil,
					fmt.Sprintf("chain[%d]", i),
					fmt.Errorf("chain signature check failed, previous certificate not signed by current: %w", err)
			}

			// Note: we purposefully avoid time-based chain validation at this layer,
			// as that could make an override that was once valid impossible to
			// bootstrap or update without destructive action.

			prev = chainCert
		}
	}

	// Normalize public key to lowercase so it matches subca.HashPublicKey.
	publicKey := cmp.Or(wantPublicKey, co.PublicKey)
	publicKey = strings.ToLower(publicKey)

	return &parsedCertificateOverride{
		certificateOverride: co,
		publicKey:           publicKey,
	}, "", nil
}
