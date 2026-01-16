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

// Package authcatest holds helpers to create CAs for testing.
package authcatest

import (
	"crypto/x509/pkix"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

// CAConfig defines the configuration for generating a test certificate
// authority.
type CAConfig struct {
	Type        types.CertAuthType
	PrivateKeys [][]byte
	Clock       clockwork.Clock
	ClusterName string
	// the below string fields default to ClusterName if left empty
	ResourceName        string
	SubjectOrganization string
}

// NewCA returns new test authority with a test key as a public and signing key.
func NewCA(
	caType types.CertAuthType,
	clusterName string,
	privateKeys ...[]byte,
) (*types.CertAuthorityV2, error) {
	return NewCAWithConfig(CAConfig{
		Type:        caType,
		ClusterName: clusterName,
		PrivateKeys: privateKeys,
		Clock:       clockwork.NewRealClock(),
	})
}

// NewCAWithConfig generates a new certificate authority with the specified
// configuration.
//
// Keep this function in-sync with lib/auth.newKeySet().
func NewCAWithConfig(config CAConfig) (*types.CertAuthorityV2, error) {
	switch config.Type {
	case
		types.HostCA,
		types.UserCA,
		types.DatabaseCA,
		types.DatabaseClientCA,
		types.OpenSSHCA,
		types.JWTSigner,
		types.SAMLIDPCA,
		types.OIDCIdPCA,
		types.SPIFFECA,
		types.OktaCA,
		types.AWSRACA,
		types.BoundKeypairCA,
		types.WindowsCA:
		// OK, known CA type.
	default:
		return nil, trace.BadParameter("cannot generate new key set for unknown CA type %q", config.Type)
	}

	var keyPEM []byte
	var key *keys.PrivateKey

	if config.ResourceName == "" {
		config.ResourceName = config.ClusterName
	}
	if config.SubjectOrganization == "" {
		config.SubjectOrganization = config.ClusterName
	}

	switch config.Type {
	case types.DatabaseCA, types.SAMLIDPCA, types.OIDCIdPCA:
		// These CAs only support RSA.
		keyPEM = fixtures.PEMBytes["rsa"]
	case types.DatabaseClientCA:
		// The db client CA also only supports RSA, but some tests rely on it
		// being different than the DB CA.
		keyPEM = fixtures.PEMBytes["rsa-db-client"]
	}
	if len(config.PrivateKeys) > 0 {
		// Allow test to override the private key.
		keyPEM = config.PrivateKeys[0]
	}

	if keyPEM != nil {
		var err error
		key, err = keys.ParsePrivateKey(keyPEM)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// If config.PrivateKeys was not set and this CA does not exclusively
		// support RSA, generate an ECDSA key. Signatures are ~10x faster than
		// RSA and generating a new key is actually faster than parsing a PEM
		// fixture.
		signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		key, err = keys.NewPrivateKey(signer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		keyPEM = key.PrivateKeyPEM()
	}

	ca := &types.CertAuthorityV2{
		Kind:    types.KindCertAuthority,
		SubKind: string(config.Type),
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      config.ResourceName,
			Namespace: apidefaults.Namespace,
		},
		Spec: types.CertAuthoritySpecV2{
			Type:        config.Type,
			ClusterName: config.ClusterName,
		},
	}

	// Add SSH keys if necessary.
	switch config.Type {
	case types.UserCA, types.HostCA, types.OpenSSHCA:
		ca.Spec.ActiveKeys.SSH = []*types.SSHKeyPair{{
			PrivateKey: keyPEM,
			PublicKey:  key.MarshalSSHPublicKey(),
		}}
	}

	// Add TLS keys if necessary.
	switch config.Type {
	case types.UserCA,
		types.HostCA,
		types.DatabaseCA,
		types.DatabaseClientCA,
		types.SAMLIDPCA,
		types.SPIFFECA,
		types.AWSRACA,
		types.WindowsCA:
		cert, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
			Signer: key.Signer,
			Entity: pkix.Name{
				CommonName:   config.ClusterName,
				Organization: []string{config.SubjectOrganization},
			},
			TTL:   defaults.CATTL,
			Clock: config.Clock,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ca.Spec.ActiveKeys.TLS = []*types.TLSKeyPair{{
			Key:  keyPEM,
			Cert: cert,
		}}
	}

	// Add JWT keys if necessary.
	switch config.Type {
	case types.JWTSigner, types.OIDCIdPCA, types.SPIFFECA, types.OktaCA, types.BoundKeypairCA:
		pubKeyPEM, err := keys.MarshalPublicKey(key.Public())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ca.Spec.ActiveKeys.JWT = []*types.JWTKeyPair{{
			PrivateKey: keyPEM,
			PublicKey:  pubKeyPEM,
		}}
	}

	// Sanity check that the CA has at least one active key.
	aks := ca.Spec.ActiveKeys
	if aks.Empty() {
		return nil, trace.BadParameter("no keys generated for CA type %q", config.Type)
	}

	return ca, nil
}
