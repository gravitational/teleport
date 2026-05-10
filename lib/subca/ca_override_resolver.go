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

// Certificate is an X.509 certificate.
type Certificate struct {
	PEM []byte
}

// CAOverrideResolver resolves CA overrides.
type CAOverrideResolver struct {
	caGetter       CAOverrideGetter
	featureEnabled bool
}

// NewCAOverrideResolver creates a new CAOverrideResolver instance.
//
// caGetter a CA override source, likely a cached services.SubCAServiceGetter
// implementation.
//
// Production callers should use [Enabled] as the featureEnabled value.
func NewCAOverrideResolver(caGetter CAOverrideGetter, featureEnabled bool) (*CAOverrideResolver, error) {
	if caGetter == nil {
		return nil, trace.BadParameter("nil caGetter")
	}
	return &CAOverrideResolver{
		caGetter:       caGetter,
		featureEnabled: featureEnabled,
	}, nil
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
	CAChain []Certificate
}

// CalculateOverride calculates CA overrides for a self-signed CA certificate.
//
// Returns a result with the CA certificate and chain to be used, with either
// the input certificate or the one acquired from an active, matching CA
// certificate override.
func (c *CAOverrideResolver) CalculateOverride(
	ctx context.Context,
	id types.CertAuthorityOverrideID,
	caCert Certificate,
) (*CalculateOverrideResult, error) {
	switch {
	case len(caCert.PEM) == 0:
		return nil, trace.BadParameter("caCert required")
	case !c.featureEnabled:
		return &CalculateOverrideResult{CACertificate: caCert}, nil
	}
	return calculateOverrides(ctx, c.caGetter, id, caCert)
}

func calculateOverrides(
	ctx context.Context,
	caOverrideGetter CAOverrideGetter,
	id types.CertAuthorityOverrideID,
	caCert Certificate,
) (*CalculateOverrideResult, error) {

	res := &CalculateOverrideResult{CACertificate: caCert}

	caOverride, err := caOverrideGetter.GetCertAuthorityOverride(ctx, id)
	if trace.IsNotFound(err) {
		// OK, no override exists.
		return res, nil
	}
	if err != nil {
		return nil, trace.Wrap(err, "read CA override")
	}
	parsed, err := ParseCAOverride(caOverride)
	if err != nil {
		return nil, trace.Wrap(err, "parse CA override")
	}

	// Be lazy. There's a chance we won't need this.
	parseCAPublicKey := sync.OnceValues(func() (string, error) {
		cert, err := tlsutils.ParseCertificatePEM(caCert.PEM)
		if err != nil {
			return "", trace.Wrap(err, "parse CA certificate")
		}
		return HashCertificatePublicKey(cert), nil
	})

	for _, co := range parsed.CertificateOverrides {
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
		if len(co.CertificateOverride.Chain) > 0 {
			chain = make([]Certificate, len(co.CertificateOverride.Chain))
			for i, pem := range co.CertificateOverride.Chain {
				chain[i] = Certificate{
					PEM: []byte(pem),
				}
			}
		}

		*res = CalculateOverrideResult{
			OverrideActive: true,
			CACertificate: Certificate{
				PEM: []byte(co.CertificateOverride.Certificate),
			},
			CAChain:       chain,
			PublicKeyHash: co.PublicKey,
		}
		break
	}

	return res, nil
}
