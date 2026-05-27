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
	"context"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
)

// CAOverrideGetter reads CA overrides from an abstracted source (remote cache,
// local cache, persistent storage, etc).
type CAOverrideGetter interface {
	// GetCertAuthorityOverride reads a CA override resource by ID.
	GetCertAuthorityOverride(ctx context.Context, id types.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error)
}

// Certificates is slice of Certificate with convenience methods attached.
type Certificates []Certificate

// ToPEMs returns all certificates as a slice of PEMs.
func (c Certificates) ToPEMs() [][]byte {
	if c == nil {
		return nil
	}

	res := make([][]byte, len(c))
	for i, cert := range c {
		res[i] = cert.PEM
	}
	return res
}

// Certificate is an X.509 certificate.
type Certificate struct {
	PEM []byte
}

// CAOverrideResolver resolves CA overrides.
type CAOverrideResolver struct {
	overridesActive bool
	parsed          *ParsedCertAuthorityOverride
}

// LoadCAOverrideResolver reads the CA override targeted by `id` from storage
// and creates a CAOverrideResolver for it. All methods of the resolver use the
// same loaded override data for their calculations.
//
// If you want to read the override again, call [LoadCAOverrideResolver] and
// create a new resolver.
//
// LoadCAOverrideResolver may skip reading the CA override if other flags
// disable the feature.
//
//   - caGetter a CA override source, likely a cached
//     services.SubCAServiceGetter implementation.
//   - isEnterpriseBuild should be set to `modules.Modules.IsEnterpriseBuild()`
//     by production callers
func LoadCAOverrideResolver(
	ctx context.Context,
	caGetter CAOverrideGetter,
	isEnterpriseBuild bool,
	id types.CertAuthorityOverrideID,
) (*CAOverrideResolver, error) {
	if !isEnterpriseBuild {
		return &CAOverrideResolver{}, nil
	}

	parsed, isNotFound, err := getParsedCAOverride(ctx, caGetter, id)
	if isNotFound {
		return &CAOverrideResolver{}, nil
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	overridesActive := false
	for _, co := range parsed.CertificateOverrides {
		if !co.CertificateOverride.GetDisabled() && co.Certificate != nil {
			overridesActive = true
			break
		}
	}

	return &CAOverrideResolver{
		overridesActive: overridesActive,
		parsed:          parsed,
	}, nil
}

func getParsedCAOverride(
	ctx context.Context,
	caGetter CAOverrideGetter,
	id types.CertAuthorityOverrideID,
) (_ *ParsedCertAuthorityOverride, isNotFound bool, _ error) {
	caOverride, err := caGetter.GetCertAuthorityOverride(ctx, id)
	if trace.IsNotFound(err) {
		isNotFound = true
		return nil, isNotFound, nil
	}
	if err != nil {
		return nil, false, trace.Wrap(err, "read CA override")
	}

	parsed, err := ParseCAOverride(caOverride)
	return parsed, false, trace.Wrap(err, "parse CA override")
}

// CalculateOverrideResult is the outcome of [CAOverrideResolver.CalculateOverride].
type CalculateOverrideResult struct {
	// OverrideActive is true if CACertPEM is a CA override certificate, instead
	// of a self-signed certificate.
	OverrideActive bool
	// PublicKeyHash is the public key hash of the overridden certificate.
	// Only set if OverrideActive is true.
	PublicKeyHash string
	// CACertificate is the CA certificate to be used.
	// It's either the input certificate, or the override certificate, depending
	// on the value of OverrideActive.
	CACertificate Certificate
	// CAChain is the certificate override trust chain, sorted leaf-to-root.
	// Includes the override CA certificate if active.
	CAChain Certificates
}

// ToClientOverrideDetailsProto returns a [proto.CAOverrideCertificateDetails]
// instance matching the CalculateOverrideResult.
//
// Returns nil if OverrideActive is false.
func (res *CalculateOverrideResult) ToClientOverrideDetailsProto() *proto.CAOverrideCertificateDetails {
	if res == nil || !res.OverrideActive {
		return nil
	}
	return &proto.CAOverrideCertificateDetails{
		PublicKeyHash: res.PublicKeyHash,
	}
}

// ApplyOverrides applies overrides to a series of certificates from a given CA.
//
// Returns the certificate PEMs to use, either the same as the input or a mix of
// the input and active override certificates.
//
// Useful to determine current/active CA certificates. For more details on each
// override, use [CAOverrideResolver.CalculateOverride].
func (c *CAOverrideResolver) ApplyOverrides(certPEMs [][]byte) ([][]byte, error) {
	if !c.overridesActive {
		return certPEMs, nil
	}

	overridePEMs := make([][]byte, len(certPEMs))
	for i, certPEM := range certPEMs {
		const skipCAChain = true
		overrideResult, err := c.calculateOverride(Certificate{PEM: certPEM}, skipCAChain)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		overridePEMs[i] = overrideResult.CACertificate.PEM
	}
	return overridePEMs, nil
}

// CalculateOverride calculates CA overrides for a self-signed CA certificate.
//
// Returns a result with the CA certificate and chain to be used, with either
// the input certificate or the one acquired from an active, matching CA
// certificate override.
func (c *CAOverrideResolver) CalculateOverride(caCert Certificate) (*CalculateOverrideResult, error) {
	switch {
	case len(caCert.PEM) == 0:
		return nil, trace.BadParameter("caCert required")
	case !c.overridesActive:
		return &CalculateOverrideResult{CACertificate: caCert}, nil
	}

	const skipCAChain = false
	return c.calculateOverride(caCert, skipCAChain)
}

func (c *CAOverrideResolver) calculateOverride(
	caCert Certificate,
	skipCAChain bool,
) (*CalculateOverrideResult, error) {
	res := &CalculateOverrideResult{CACertificate: caCert}

	// Be lazy. There's a chance we won't need this.
	parseCAPublicKey := sync.OnceValues(func() (string, error) {
		cert, err := tlsutils.ParseCertificatePEM(caCert.PEM)
		if err != nil {
			return "", trace.Wrap(err, "parse CA certificate")
		}
		return HashCertificatePublicKey(cert), nil
	})

	for _, co := range c.parsed.CertificateOverrides {
		if co.CertificateOverride.GetDisabled() || co.Certificate == nil {
			continue
		}

		caPublicKey, err := parseCAPublicKey()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if co.PublicKey != caPublicKey {
			continue
		}

		var chain []Certificate
		if !skipCAChain {
			chain = make([]Certificate, 0, len(co.CertificateOverride.Chain)+1)
			chain = append(chain, Certificate{
				PEM: []byte(co.CertificateOverride.Certificate),
			})
			for _, pem := range co.CertificateOverride.Chain {
				chain = append(chain, Certificate{
					PEM: []byte(pem),
				})
			}
		}

		*res = CalculateOverrideResult{
			OverrideActive: true,
			PublicKeyHash:  co.PublicKey,
			CACertificate: Certificate{
				PEM: []byte(co.CertificateOverride.Certificate),
			},
			CAChain: chain,
		}
		break
	}

	return res, nil
}
