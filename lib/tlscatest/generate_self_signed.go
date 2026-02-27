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

package tlscatest

import (
	"crypto/x509/pkix"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
)

// GenerateCAConfig is the input for [GenerateSelfSignedCA].
type GenerateCAConfig struct {
	// ClusterName is the Teleport cluster name.
	ClusterName string
	// NotBefore and NotAfter are the certificate timestamps.
	// Optional.
	NotBefore, NotAfter time.Time
}

// GenerateSelfSignedCA creates a self-signed Teleport CA certificate for
// testing.
//
// The private key is marshaled using [keys.MarshalPrivateKey].
//
// Returns the key and certificate in PEM form.
func GenerateSelfSignedCA(config GenerateCAConfig) (keyPEM, certPEM []byte, _ error) {
	if config.ClusterName == "" {
		return nil, nil, trace.BadParameter("cluster name required")
	}

	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	keyPEM, err = keys.MarshalPrivateKey(signer)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certPEM, err = tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Signer: signer,
		Entity: pkix.Name{
			Organization: []string{config.ClusterName},
			CommonName:   config.ClusterName,
		},
		// Default to 1h / real time...
		TTL: 1 * time.Hour,
		// ...but allow customized timestamps.
		NotBefore: config.NotBefore,
		NotAfter:  config.NotAfter,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyPEM, certPEM, nil
}
