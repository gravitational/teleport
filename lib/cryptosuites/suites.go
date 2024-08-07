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
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
)

const defaultSuite = types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY

// KeyPurpose represents the purpose of a keypair.
type KeyPurpose int

const (
	KeyPurposeUnspecified KeyPurpose = iota

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

	// ProxyToDatabaseAgent represents keys used by the Proxy to dial the
	// Database agent over a reverse tunnel.
	ProxyToDatabaseAgent

	// UserSSH represents a user SSH key.
	UserSSH
	// UserTLS represents a user TLS key.
	UserTLS

	// TODO(nklaassen): define remaining key purposes.

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

func (a Algorithm) String() string {
	switch a {
	case algorithmUnspecified:
		return "algorithm unspecified"
	case RSA2048:
		return "RSA2048"
	case ECDSAP256:
		return "ECDSAP256"
	case Ed25519:
		return "Ed25519"
	default:
		return fmt.Sprintf("unknown algorithm %d", a)
	}
}

// suite defines the cryptographic signature algorithm used for each unique key purpose.
type suite map[KeyPurpose]Algorithm

var (
	// legacy is the original algorithm suite, which exclusively uses RSA2048.
	legacy = suite{
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
		UserSSH:             RSA2048,
		UserTLS:             RSA2048,
		// We could consider updating this algorithm even in the legacy suite, only database agents need to
		// accept these connections and they have never restricted algorithm support.
		ProxyToDatabaseAgent: RSA2048,
		// TODO(nklaassen): define remaining key purposes.
	}

	// balancedV1 strikes a balance between security, compatibility, and
	// performance. It uses ECDSA256, Ed25591, and 2048-bit RSA. It is not
	// completely implemented yet.
	balancedV1 = suite{
		UserCATLS:            ECDSAP256,
		UserCASSH:            Ed25519,
		HostCATLS:            ECDSAP256,
		HostCASSH:            Ed25519,
		DatabaseCATLS:        RSA2048,
		DatabaseClientCATLS:  RSA2048,
		OpenSSHCASSH:         Ed25519,
		JWTCAJWT:             ECDSAP256,
		OIDCIdPCAJWT:         ECDSAP256,
		SAMLIdPCATLS:         RSA2048,
		SPIFFECATLS:          ECDSAP256,
		SPIFFECAJWT:          ECDSAP256,
		UserSSH:              Ed25519,
		UserTLS:              ECDSAP256,
		ProxyToDatabaseAgent: ECDSAP256,
		// TODO(nklaassen): define remaining key purposes.
	}

	// fipsv1 is an algorithm suite tailored for FIPS compliance. It is based on
	// the balancedv1 suite but replaces all instances of Ed25519 with ECDSA on
	// the NIST P256 curve. It is not completely implemented yet.
	fipsv1 = suite{
		UserCATLS:            ECDSAP256,
		UserCASSH:            ECDSAP256,
		HostCATLS:            ECDSAP256,
		HostCASSH:            ECDSAP256,
		DatabaseCATLS:        RSA2048,
		DatabaseClientCATLS:  RSA2048,
		OpenSSHCASSH:         ECDSAP256,
		JWTCAJWT:             ECDSAP256,
		OIDCIdPCAJWT:         ECDSAP256,
		SAMLIdPCATLS:         RSA2048,
		SPIFFECATLS:          ECDSAP256,
		SPIFFECAJWT:          ECDSAP256,
		UserSSH:              ECDSAP256,
		UserTLS:              ECDSAP256,
		ProxyToDatabaseAgent: ECDSAP256,
		// TODO(nklaassen): define remaining key purposes.
	}

	// hsmv1 in an algorithm suite tailored for clusters using an HSM or KMS
	// service to back CA private material.  It is based on the balancedv1 suite
	// but replaces Ed25519 with ECDSA on the NIST P256 curve *for CA keys
	// only*. It is also valid to use the legacy or fipsv1 suites if your
	// cluster uses an HSM or KMS. It is not completely implemented yet.
	hsmv1 = suite{
		UserCATLS:            ECDSAP256,
		UserCASSH:            ECDSAP256,
		HostCATLS:            ECDSAP256,
		HostCASSH:            ECDSAP256,
		DatabaseCATLS:        RSA2048,
		DatabaseClientCATLS:  RSA2048,
		OpenSSHCASSH:         ECDSAP256,
		JWTCAJWT:             ECDSAP256,
		OIDCIdPCAJWT:         ECDSAP256,
		SAMLIdPCATLS:         RSA2048,
		SPIFFECATLS:          ECDSAP256,
		SPIFFECAJWT:          ECDSAP256,
		UserSSH:              Ed25519,
		UserTLS:              ECDSAP256,
		ProxyToDatabaseAgent: ECDSAP256,
		// TODO(nklaassen): define remaining key purposes.
	}

	allSuites = map[types.SignatureAlgorithmSuite]suite{
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY:      legacy,
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1: balancedV1,
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1:     fipsv1,
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1:      hsmv1,
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
		return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, trace.Wrap(err)
	}
	suite := authPref.GetSignatureAlgorithmSuite()
	if suite == types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED {
		return defaultSuite, nil
	}
	return suite, nil
}

func algorithmForKey(suite types.SignatureAlgorithmSuite, purpose KeyPurpose) (Algorithm, error) {
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

// AlgorithmForKey returns the cryptographic signature algorithm that should be used for new keys with the
// given purpose, based on the currently configured algorithm suite.
func AlgorithmForKey(ctx context.Context, authPrefGetter AuthPreferenceGetter, purpose KeyPurpose) (Algorithm, error) {
	suite, err := getSignatureAlgorithmSuite(ctx, authPrefGetter)
	if err != nil {
		return algorithmUnspecified, trace.Wrap(err)
	}
	alg, err := algorithmForKey(suite, purpose)
	if err != nil {
		return algorithmUnspecified, trace.Wrap(err)
	}
	return alg, nil
}

// GenerateUserSSHAndTLSKey generates and returns a pair of keys to be used for
// user SSH and TLS keys. If the legacy algorithm suite is currently configured,
// a single key will be generated and returned.
func GenerateUserSSHAndTLSKey(ctx context.Context, authPrefGetter AuthPreferenceGetter) (sshKey, tlsKey crypto.Signer, err error) {
	currentSuite, err := getSignatureAlgorithmSuite(ctx, authPrefGetter)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if currentSuite == types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY {
		// If the legacy suite is configured, generate a single RSA2048 key and
		// use it for both SSH and TLS.
		key, err := generateRSA2048()
		return key, key, trace.Wrap(err)
	}

	sshKeyAlgorithm, err := algorithmForKey(currentSuite, UserSSH)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sshKey, err = GenerateKeyWithAlgorithm(sshKeyAlgorithm)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsKeyAlgorithm, err := algorithmForKey(currentSuite, UserTLS)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	tlsKey, err = GenerateKeyWithAlgorithm(tlsKeyAlgorithm)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return sshKey, tlsKey, nil
}

// AlgorithmForKey generates a new cryptographic keypair for the given [purpose],
// with a signature algorithm chosen based on the algorithm suite currently
// configured in the cluster auth preference.
func GenerateKey(ctx context.Context, authPrefGetter AuthPreferenceGetter, purpose KeyPurpose) (crypto.Signer, error) {
	alg, err := AlgorithmForKey(ctx, authPrefGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return GenerateKeyWithAlgorithm(alg)
}

// GenerateKeyWithSuite generates a new cryptographic keypair for the given
// [purpose], with a signature algorithm chosen from [suite].
func GenerateKeyWithSuite(ctx context.Context, suite types.SignatureAlgorithmSuite, purpose KeyPurpose) (crypto.Signer, error) {
	if suite == types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED {
		suite = defaultSuite
	}
	alg, err := algorithmForKey(suite, purpose)
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
