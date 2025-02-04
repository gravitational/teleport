/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// CertAuthoritiesEquivalent checks if a pair of certificate authority resources are equivalent.
// This differs from normal equality only in that resource IDs are ignored.
func CertAuthoritiesEquivalent(lhs, rhs types.CertAuthority) bool {
	return cmp.Equal(lhs, rhs,
		ignoreProtoXXXFields(),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		// Optimize types.CAKeySet comparison.
		cmp.Comparer(func(a, b types.CAKeySet) bool {
			// Note that Clone drops XXX_ fields. And it's benchmarked that cloning
			// plus using proto.Equal is more efficient than cmp.Equal.
			aClone := a.Clone()
			bClone := b.Clone()
			return proto.Equal(&aClone, &bClone)
		}),
	)
}

// ValidateCertAuthority validates the CertAuthority
func ValidateCertAuthority(ca types.CertAuthority) (err error) {
	if err = CheckAndSetDefaults(ca); err != nil {
		return trace.Wrap(err)
	}

	switch ca.GetType() {
	case types.UserCA, types.HostCA:
		err = checkUserOrHostCA(ca)
	case types.DatabaseCA, types.DatabaseClientCA:
		err = checkDatabaseCA(ca)
	case types.OpenSSHCA:
		err = checkOpenSSHCA(ca)
	case types.JWTSigner, types.OIDCIdPCA, types.OktaCA:
		err = checkJWTKeys(ca)
	case types.SAMLIDPCA:
		err = checkSAMLIDPCA(ca)
	case types.SPIFFECA:
		err = checkSPIFFECA(ca)
	default:
		return trace.BadParameter("invalid CA type %q", ca.GetType())
	}
	return trace.Wrap(err)
}

func checkSPIFFECA(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}
	if len(ca.Spec.ActiveKeys.TLS) == 0 {
		return trace.BadParameter("certificate authority missing TLS key pairs")
	}
	if len(ca.Spec.ActiveKeys.JWT) == 0 {
		return trace.BadParameter("certificate authority missing JWT key pairs")
	}
	return nil
}

func checkUserOrHostCA(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}
	if len(ca.Spec.ActiveKeys.SSH) == 0 {
		return trace.BadParameter("certificate authority missing SSH key pairs")
	}
	if len(ca.Spec.ActiveKeys.TLS) == 0 {
		return trace.BadParameter("certificate authority missing TLS key pairs")
	}
	if _, err := sshutils.GetCheckers(ca); err != nil {
		return trace.Wrap(err)
	}
	if err := sshutils.ValidateSigners(ca); err != nil {
		return trace.Wrap(err)
	}
	// This is to force users to migrate
	if len(ca.GetRoles()) != 0 && len(ca.GetRoleMap()) != 0 {
		return trace.BadParameter("should set either 'roles' or 'role_map', not both")
	}
	_, err := parseRoleMap(ca.GetRoleMap())
	return trace.Wrap(err)
}

// checkDatabaseCA checks if provided certificate authority contains a valid TLS key pair.
// This function is used to verify Database CA.
func checkDatabaseCA(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}

	if len(ca.Spec.ActiveKeys.TLS) == 0 {
		return trace.BadParameter("%s certificate authority missing TLS key pairs", ca.GetType())
	}

	for _, pair := range ca.GetTrustedTLSKeyPairs() {
		if len(pair.Key) > 0 && pair.KeyType == types.PrivateKeyType_RAW {
			var err error
			if len(pair.Cert) > 0 {
				_, err = tls.X509KeyPair(pair.Cert, pair.Key)
			} else {
				_, err = keys.ParsePrivateKey(pair.Key)
			}
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			_, err := tlsca.ParseCertificatePEM(pair.Cert)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// checkOpenSSHCA checks if provided certificate authority contains a valid SSH key pair.
func checkOpenSSHCA(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}
	if len(ca.Spec.ActiveKeys.SSH) == 0 {
		return trace.BadParameter("certificate authority missing SSH key pairs")
	}
	if _, err := sshutils.GetCheckers(ca); err != nil {
		return trace.Wrap(err)
	}
	if err := sshutils.ValidateSigners(ca); err != nil {
		return trace.Wrap(err)
	}
	// This is to force users to migrate
	if len(ca.GetRoles()) != 0 && len(ca.GetRoleMap()) != 0 {
		return trace.BadParameter("should set either 'roles' or 'role_map', not both")
	}
	_, err := parseRoleMap(ca.GetRoleMap())
	return trace.Wrap(err)
}

func checkJWTKeys(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}
	// Check that some JWT keys have been set on the CA.
	if len(ca.Spec.ActiveKeys.JWT) == 0 {
		return trace.BadParameter("missing JWT CA")
	}

	var err error
	var privateKey crypto.Signer

	// Check that the JWT keys set are valid.
	for _, pair := range ca.GetTrustedJWTKeyPairs() {
		// TODO(nic): validate PKCS11 private keys
		if len(pair.PrivateKey) > 0 && pair.PrivateKeyType == types.PrivateKeyType_RAW {
			privateKey, err = keys.ParsePrivateKey(pair.PrivateKey)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		publicKey, err := keys.ParsePublicKey(pair.PublicKey)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg := &jwt.Config{
			ClusterName: ca.GetClusterName(),
			PrivateKey:  privateKey,
			PublicKey:   publicKey,
		}
		if _, err = jwt.New(cfg); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// checkSAMLIDPCA checks if provided certificate authority contains a valid TLS key pair.
// This function is used to verify the SAML IDP CA.
func checkSAMLIDPCA(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}

	if len(ca.Spec.ActiveKeys.TLS) == 0 {
		return trace.BadParameter("missing SAML IdP CA")
	}

	for _, pair := range ca.GetTrustedTLSKeyPairs() {
		if len(pair.Key) != 0 && pair.KeyType == types.PrivateKeyType_RAW {
			var err error
			if len(pair.Cert) > 0 {
				_, err = tls.X509KeyPair(pair.Cert, pair.Key)
			} else {
				_, err = keys.ParsePrivateKey(pair.Key)
			}
			if err != nil {
				return trace.Wrap(err)
			}
		}
		if _, err := tlsca.ParseCertificatePEM(pair.Cert); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetJWTSigner returns the active JWT key used to sign tokens.
func GetJWTSigner(signer crypto.Signer, clusterName string, clock clockwork.Clock) (*jwt.Key, error) {
	key, err := jwt.New(&jwt.Config{
		Clock:       clock,
		ClusterName: clusterName,
		PrivateKey:  signer,
	})
	return key, trace.Wrap(err)
}

// GetTLSCerts returns TLS certificates from CA
func GetTLSCerts(ca types.CertAuthority) [][]byte {
	pairs := ca.GetTrustedTLSKeyPairs()
	out := make([][]byte, len(pairs))
	for i, pair := range pairs {
		out[i] = append([]byte{}, pair.Cert...)
	}
	return out
}

// GetSSHCheckingKeys returns SSH public keys from CA
func GetSSHCheckingKeys(ca types.CertAuthority) [][]byte {
	pairs := ca.GetTrustedSSHKeyPairs()
	out := make([][]byte, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, append([]byte{}, pair.PublicKey...))
	}
	return out
}

// CertPoolFromCertAuthorities returns a certificate pool from the TLS certificates
// set up in the certificate authorities list, as well as the number of certificates
// that were added to the pool.
func CertPoolFromCertAuthorities(cas []types.CertAuthority) (*x509.CertPool, int, error) {
	certPool := x509.NewCertPool()
	count := 0
	for _, ca := range cas {
		keyPairs := ca.GetTrustedTLSKeyPairs()
		if len(keyPairs) == 0 {
			continue
		}
		for _, keyPair := range keyPairs {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, 0, trace.Wrap(err)
			}
			certPool.AddCert(cert)
			count++
		}
	}
	return certPool, count, nil
}

// CertPool returns certificate pools from TLS certificates
// set up in the certificate authority
func CertPool(ca types.CertAuthority) (*x509.CertPool, error) {
	keyPairs := ca.GetTrustedTLSKeyPairs()
	if len(keyPairs) == 0 {
		return nil, trace.BadParameter("certificate authority has no TLS certificates")
	}
	certPool := x509.NewCertPool()
	for _, keyPair := range keyPairs {
		cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool.AddCert(cert)
	}
	return certPool, nil
}

// MarshalCertRoles marshal roles list to OpenSSH
func MarshalCertRoles(roles []string) (string, error) {
	out, err := json.Marshal(types.CertRoles{Version: types.V1, Roles: roles})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(out), err
}

// UnmarshalCertRoles marshals roles list to OpenSSH format
func UnmarshalCertRoles(data string) ([]string, error) {
	var certRoles types.CertRoles
	if err := utils.FastUnmarshal([]byte(data), &certRoles); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	return certRoles.Roles, nil
}

// UnmarshalCertAuthority unmarshals the CertAuthority resource to JSON.
func UnmarshalCertAuthority(bytes []byte, opts ...MarshalOption) (types.CertAuthority, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	err = utils.FastUnmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V2:
		var ca types.CertAuthorityV2
		if err := utils.FastUnmarshal(bytes, &ca); err != nil {
			return nil, trace.BadParameter("%s", err)
		}

		if err := ValidateCertAuthority(&ca); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			ca.SetRevision(cfg.Revision)
		}
		// Correct problems with existing CAs that contain non-UTC times, which
		// causes panics when doing a gogoproto Clone; should only ever be
		// possible with LastRotated, but we enforce it on all the times anyway.
		// See https://github.com/gogo/protobuf/issues/519 .
		if ca.Spec.Rotation != nil {
			apiutils.UTC(&ca.Spec.Rotation.Started)
			apiutils.UTC(&ca.Spec.Rotation.LastRotated)
			apiutils.UTC(&ca.Spec.Rotation.Schedule.UpdateClients)
			apiutils.UTC(&ca.Spec.Rotation.Schedule.UpdateServers)
			apiutils.UTC(&ca.Spec.Rotation.Schedule.Standby)
		}

		return &ca, nil
	}

	return nil, trace.BadParameter("cert authority resource version %v is not supported", h.Version)
}

// MarshalCertAuthority marshals the CertAuthority resource to JSON.
func MarshalCertAuthority(certAuthority types.CertAuthority, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateCertAuthority(certAuthority); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch certAuthority := certAuthority.(type) {
	case *types.CertAuthorityV2:
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, certAuthority))
	default:
		return nil, trace.BadParameter("unrecognized certificate authority version %T", certAuthority)
	}
}
