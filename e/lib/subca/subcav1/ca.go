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
	"crypto/x509"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/subca"
)

type parsedCertAuthority struct {
	CA                  types.CertAuthority
	ActiveKeyHashes     map[string]*x509.Certificate // key is the PublicKeyHash
	AdditionalKeyHashes map[string]*x509.Certificate // key is the PublicKeyHash
	IndexedKeyPairs     map[string]*types.TLSKeyPair // key is the PublicKeyHash
}

func parseCA(ctx context.Context, ca types.CertAuthority) (*parsedCertAuthority, error) {
	// Unexpected. Defensive only.
	if ca == nil {
		return nil, trace.BadParameter("nil CA")
	}

	parsed := &parsedCertAuthority{
		CA:                  ca,
		ActiveKeyHashes:     make(map[string]*x509.Certificate),
		AdditionalKeyHashes: make(map[string]*x509.Certificate),
		IndexedKeyPairs:     make(map[string]*types.TLSKeyPair),
	}

	activeKeys := ca.GetActiveKeys()
	additionalKeys := ca.GetAdditionalTrustedKeys()

	for i, keys := range [][]*types.TLSKeyPair{
		activeKeys.TLS,
		additionalKeys.TLS,
	} {
		isActive := i == 0
		for j, kp := range keys {
			// Unexpected. Defensive only.
			if len(kp.Cert) == 0 {
				slog.WarnContext(ctx,
					"CA TLSKeyPair has empty certificate",
					"ca_type", ca.GetType(),
					"cluster_name", ca.GetClusterName(),
					"is_active", isActive,
					"index", j,
				)
				continue
			}

			cert, err := tlsutils.ParseCertificatePEM(kp.Cert)
			if err != nil {
				return nil, trace.Wrap(err, "parse CA certificate (is_active=%v, index=%d)", isActive, j)
			}
			pkh := subca.HashCertificatePublicKey(cert)

			if isActive {
				parsed.ActiveKeyHashes[pkh] = cert
			} else {
				parsed.AdditionalKeyHashes[pkh] = cert
			}
			parsed.IndexedKeyPairs[pkh] = kp
		}
	}

	return parsed, nil
}
