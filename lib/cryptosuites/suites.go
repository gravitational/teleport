// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

// Package cryptosuites implements software cryptographic key generation using the appropriate key/signature
// algorithm for they key's purpose and the current configured algorithm suite in the cluster.
package cryptosuites

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
)

const defaultSuite = types.SignatureAlgorithmSuite_LEGACY

// KeyPurpose represents the purpose of a keypair.
type KeyPurpose int

const (
	keyPurposeUnspecified KeyPurpose = iota

	// UserCATLS represents the TLS key for the user CA.
	UserCATLS
	// UserCASSH represents the SSH key for the user CA.
	UserCASSH

	// HostCATLS represents the TLS key for the host CA.
	HostCATLS
	// HostCASSH represents the SSH key for the host CA.
	HostCASSH

	// DatabaseCATLS represents the TLS key for the db CA.
	DatabaseCATLS
	// DatabaseClientCATLS represents the TLS key for the db_client CA.
	DatabaseClientCATLS

	// OpenSSHCASSH represents the SSH key for the openssh CA.
	OpenSSHCASSH

	// JWTCAJWT represents the JWT key for the JWT CA.
	JWTCAJWT

	// OIDCIdPCAJWT represents the JWT key for the oidc_idp CA.
	OIDCIdPCAJWT

	// SAMLIdPCATLS represents the TLS key for the saml_idp CA.
	SAMLIdPCATLS

	// SPIFFECATLS represents the TLS key for the spiffe CA.
	SPIFFECATLS

	// SPIFFECAJWT represents the JWT key for the spiffe CA.
	SPIFFECAJWT

	// New key purposes should be added here.

	// TODO(nklaassen): define subject key purposes. Currently only CA key purposes are defined above.

	// keyPurposeMax is 1 greater than the last valid key purpose, used to test that all values less than this
	// are valid for each suite.
	keyPurposeMax
)

// Algorithm represents a cryptographic signature algorithm.
type Algorithm int

const (
	algorithmUnspecified Algorithm = iota

	// RSA2048 represents RSA 2048-bit keys.
	RSA2048
	// ECDSAP256 represents ECDSA keys using NIST curve P-256.
	ECDSAP256
	// Ed25519 represents Ed25519 keys.
	Ed25519

	// algorithmMax is 1 greater than the last valid algorithm, used to test that an algorithm is valid.
	algorithmMax
)

// Suite defines the cryptographic signature algorithm used for each unique key purpose.
type Suite map[KeyPurpose]Algorithm

var (
	// Legacy is the original algorithm suite, which exclusively uses RSA2048. It is the current default
	// suite.
	Legacy = Suite{
		UserCATLS:           RSA2048,
		UserCASSH:           RSA2048,
		HostCATLS:           RSA2048,
		HostCASSH:           RSA2048,
		DatabaseCATLS:       RSA2048,
		DatabaseClientCATLS: RSA2048,
		OpenSSHCASSH:        RSA2048,
		JWTCAJWT:            RSA2048,
		OIDCIdPCAJWT:        RSA2048,
		SAMLIdPCATLS:        RSA2048,
		SPIFFECATLS:         RSA2048,
		SPIFFECAJWT:         RSA2048,
		// TODO(nklaassen): subject key purposes.
	}

	// BalancedV1 strikes a balance between security, compatibility, and performance. It uses ECDSA256,
	// Ed25591, and 2048-bit RSA. It is not completely implemented yet.
	BalancedV1 = Suite{
		UserCATLS:           ECDSAP256,
		UserCASSH:           Ed25519,
		HostCATLS:           ECDSAP256,
		HostCASSH:           Ed25519,
		DatabaseCATLS:       RSA2048,
		DatabaseClientCATLS: RSA2048,
		OpenSSHCASSH:        Ed25519,
		JWTCAJWT:            ECDSAP256,
		OIDCIdPCAJWT:        ECDSAP256,
		SAMLIdPCATLS:        ECDSAP256,
		SPIFFECATLS:         ECDSAP256,
		SPIFFECAJWT:         ECDSAP256,
		// TODO(nklaassen): subject key purposes.
	}

	// FIPSV1 is an algorithm suite tailored for FIPS compliance. It is based on the BALANCED suite but
	// replaces all instances of Ed25519 with ECDSA on the NIST P256 curve. It is not completely implemented
	// yet.
	FIPSV1 = Suite{
		UserCATLS:           ECDSAP256,
		UserCASSH:           ECDSAP256,
		HostCATLS:           ECDSAP256,
		HostCASSH:           ECDSAP256,
		DatabaseCATLS:       RSA2048,
		DatabaseClientCATLS: RSA2048,
		OpenSSHCASSH:        ECDSAP256,
		JWTCAJWT:            ECDSAP256,
		OIDCIdPCAJWT:        ECDSAP256,
		SAMLIdPCATLS:        ECDSAP256,
		SPIFFECATLS:         ECDSAP256,
		SPIFFECAJWT:         ECDSAP256,
		// TODO(nklaassen): subject key purposes.
	}

	// HSMV1 in an algorithm suite tailored for clusters using an HSM or KMS service to back CA private material.
	// It is based on the BALANCED suite but replaces Ed25519 with ECDSA on the NIST P256 curve *for CA keys
	// only*. It is also valid to use the LEGACY for FIPS_v1 suites if your cluster uses an HSM or KMS. It is
	// not completely implemented yet.
	HSMV1 = Suite{
		UserCATLS:           ECDSAP256,
		UserCASSH:           ECDSAP256,
		HostCATLS:           ECDSAP256,
		HostCASSH:           ECDSAP256,
		DatabaseCATLS:       RSA2048,
		DatabaseClientCATLS: RSA2048,
		OpenSSHCASSH:        ECDSAP256,
		JWTCAJWT:            ECDSAP256,
		OIDCIdPCAJWT:        ECDSAP256,
		SAMLIdPCATLS:        ECDSAP256,
		SPIFFECATLS:         ECDSAP256,
		SPIFFECAJWT:         ECDSAP256,
		// TODO(nklaassen): subject key purposes.
	}

	allSuites = map[types.SignatureAlgorithmSuite]Suite{
		types.SignatureAlgorithmSuite_LEGACY:       Legacy,
		types.SignatureAlgorithmSuite_BALANCED_DEV: BalancedV1,
		types.SignatureAlgorithmSuite_FIPS_DEV:     FIPSV1,
		types.SignatureAlgorithmSuite_HSM_DEV:      HSMV1,
	}
)

// AuthPreferenceGetter is an interface for retrieving the current configured cluster auth preference.
type AuthPreferenceGetter interface {
	// GetAuthPreference returns the current cluster auth preference.
	GetAuthPreference(context.Context) (types.AuthPreference, error)
}

func getSignatureAlgorithmSuite(ctx context.Context, authPrefGetter AuthPreferenceGetter) (types.SignatureAlgorithmSuite, error) {
	authPref, err := authPrefGetter.GetAuthPreference(ctx)
	if err != nil {
		return types.SignatureAlgorithmSuite_UNSPECIFIED, trace.Wrap(err)
	}
	suite := authPref.GetSignatureAlgorithmSuite()
	if suite == types.SignatureAlgorithmSuite_UNSPECIFIED {
		return defaultSuite, nil
	}
	return suite, nil
}

// AlgorithmForKey returns the cryptographic signature algorithm that should be used for new keys with the
// given purpose, based on the currently configured algorithm suite.
func AlgorithmForKey(ctx context.Context, authPrefGetter AuthPreferenceGetter, purpose KeyPurpose) (Algorithm, error) {
	suite, err := getSignatureAlgorithmSuite(ctx, authPrefGetter)
	if err != nil {
		return algorithmUnspecified, trace.Wrap(err)
	}
	s, ok := allSuites[suite]
	if !ok {
		return algorithmUnspecified, trace.BadParameter("unsupported signature algorithm suite %v", suite)
	}
	alg, ok := s[purpose]
	if !ok {
		return algorithmUnspecified, trace.BadParameter("unsupported key purpose %v", purpose)
	}
	return alg, nil
}

// AlgorithmForKey generates a new cryptographic keypair for the given purpose, with a signature algorithm
// chosen based on the currently configured algorithm suite.
func GenerateKey(ctx context.Context, authPrefGetter AuthPreferenceGetter, purpose KeyPurpose) (crypto.Signer, error) {
	alg, err := AlgorithmForKey(ctx, authPrefGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return GenerateKeyWithAlgorithm(alg)
}

// GenerateKeyWithAlgorithm generates a new cryptographic keypair with the given algorithm.
func GenerateKeyWithAlgorithm(alg Algorithm) (crypto.Signer, error) {
	switch alg {
	case RSA2048:
		return generateRSA2048()
	case ECDSAP256:
		return generateECDSAP256()
	case Ed25519:
		return generateEd25519()
	default:
		return nil, trace.BadParameter("unsupported key algorithm %v", alg)
	}
}

func generateRSA2048() (*rsa.PrivateKey, error) {
	key, err := native.GenerateRSAPrivateKey()
	return key, trace.Wrap(err)
}

func generateECDSAP256() (*ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return key, trace.Wrap(err)
}

func generateEd25519() (ed25519.PrivateKey, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	return key, trace.Wrap(err)
}
