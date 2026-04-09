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

package subca

import (
	"cmp"
	"crypto/x509"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
)

var (
	allowedCAOverrideSubKinds = []string{
		string(types.DatabaseClientCA),
		string(types.WindowsCA),
	}

	// Public keys are printed as HEX(SHA256(...)), therefore it's a hex string
	// with exactly 64 characters. See PublicKeyHash.
	certificateOverridePublicKeyRE = regexp.MustCompile(`^[0-9A-Fa-f]{64}$`)
)

// ParsedCertAuthorityOverride is a CertAuthorityOverride with a
// ParsedCertificateOverride list.
type ParsedCertAuthorityOverride struct {
	// CAOverride is the original resource.
	CAOverride *subcav1.CertAuthorityOverride
	// CertificateOverrides is a ParsedCertAuthorityOverride list, parallel to
	// CAOverride.Spec.CertificateOverrides.
	CertificateOverrides []*ParsedCertificateOverride
}

// ParsedCertificateOverride is a CertificateOverride with its certificates
// parsed and a normalized PublicKeyHash field.
type ParsedCertificateOverride struct {
	// CertificateOverride is the original resource.
	CertificateOverride *subcav1.CertificateOverride
	// PublicKey is a normalized public key from CertificateOverride.
	// If CertificateOverride.Certificate is present it's calculated from it,
	// otherwise it's normalized from CertificateOverride.PublicKey.
	PublicKey string
	// Certificate is the parsed certificate.
	Certificate *x509.Certificate
	// Chain is the parsed certificate chain.
	Chain []*x509.Certificate
}

// ValidateAndParseCAOverride validates a CertAuthorityOverride resource and
// returns it in parsed form.
//
// Used by storage to validate resources in write paths.
//
// It should not be used by any layers in read paths. Stored resources are
// considered valid and must not be subject to validation (lest they fail new
// validation rules, making reading impossible).
func ValidateAndParseCAOverride(resource *subcav1.CertAuthorityOverride) (*ParsedCertAuthorityOverride, error) {
	switch {
	case resource == nil:
		return nil, trace.BadParameter("ca override required")
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
	parsedOverrides := make([]*ParsedCertificateOverride, len(overrides))
	seenPublicKeys := make(map[string]struct{})
	for i, co := range overrides {
		parsedCO, fieldName, err := validateCertificateOverride(co)
		if err != nil {
			if fieldName != "" {
				fieldName = "." + fieldName
			}
			return nil, trace.BadParameter("spec.certificate_overrides[%d]%s: %v", i, fieldName, err)
		}

		if _, ok := seenPublicKeys[parsedCO.PublicKey]; ok {
			return nil, trace.BadParameter(
				"spec.certificate_overrides[%d]: found duplicate override for public key %q",
				i, parsedCO.PublicKey,
			)
		}
		seenPublicKeys[parsedCO.PublicKey] = struct{}{}

		parsedOverrides[i] = parsedCO
	}

	return &ParsedCertAuthorityOverride{
		CAOverride:           resource,
		CertificateOverrides: parsedOverrides,
	}, nil
}

func validateCertificateOverride(
	co *subcav1.CertificateOverride,
) (_ *ParsedCertificateOverride, fieldName string, _ error) {
	// Trace not used on purpose. Errors are trace-wrapped up in the chain.
	if co == nil {
		return nil, "", errors.New("nil certificate override")
	}

	// Certificate.
	var cert *x509.Certificate
	var wantPublicKey string
	if co.Certificate != "" {
		var err error
		cert, err = ParseCertificateOverrideCertificate(co.Certificate)
		if err != nil {
			return nil, "certificate", err
		}
		wantPublicKey = HashCertificatePublicKey(cert)
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

	var chain []*x509.Certificate
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

		chain = make([]*x509.Certificate, len(co.Chain))
		prev := cert
		for i, chainPEM := range co.Chain {
			chainCert, err := ParseCertificateOverrideCertificate(chainPEM)
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

			chain[i] = chainCert
			prev = chainCert
		}
	}

	// Normalize public key to lowercase so it matches HashPublicKey.
	publicKey := cmp.Or(wantPublicKey, co.PublicKey)
	publicKey = strings.ToLower(publicKey)

	return &ParsedCertificateOverride{
		CertificateOverride: co,
		PublicKey:           publicKey,
		Certificate:         cert,
		Chain:               chain,
	}, "", nil
}
