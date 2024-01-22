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
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/defaults"
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
	case types.JWTSigner, types.OIDCIdPCA:
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
				_, err = utils.ParsePrivateKey(pair.Key)
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
			privateKey, err = utils.ParsePrivateKey(pair.PrivateKey)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		publicKey, err := utils.ParsePublicKey(pair.PublicKey)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg := &jwt.Config{
			Algorithm:   defaults.ApplicationTokenAlgorithm,
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
				_, err = utils.ParsePrivateKey(pair.Key)
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
		Algorithm:   defaults.ApplicationTokenAlgorithm,
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

// HostCertParams defines all parameters needed to generate a host certificate
type HostCertParams struct {
	// CASigner is the signer that will sign the public key of the host with the CA private key.
	CASigner ssh.Signer
	// PublicHostKey is the public key of the host
	PublicHostKey []byte
	// HostID is used by Teleport to uniquely identify a node within a cluster
	HostID string
	// Principals is a list of additional principals to add to the certificate.
	Principals []string
	// NodeName is the DNS name of the node
	NodeName string
	// ClusterName is the name of the cluster within which a node lives
	ClusterName string
	// Role identifies the role of a Teleport instance
	Role types.SystemRole
	// TTL defines how long a certificate is valid for
	TTL time.Duration
}

// Check checks parameters for errors
func (c HostCertParams) Check() error {
	if c.CASigner == nil {
		return trace.BadParameter("CASigner is required")
	}
	if c.HostID == "" && len(c.Principals) == 0 {
		return trace.BadParameter("HostID [%q] or Principals [%q] are required",
			c.HostID, c.Principals)
	}
	if c.ClusterName == "" {
		return trace.BadParameter("ClusterName [%q] is required", c.ClusterName)
	}

	if err := c.Role.Check(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UserCertParams defines OpenSSH user certificate parameters
type UserCertParams struct {
	// CASigner is the signer that will sign the public key of the user with the CA private key
	CASigner ssh.Signer
	// PublicUserKey is the public key of the user
	PublicUserKey []byte
	// TTL defines how long a certificate is valid for
	TTL time.Duration
	// Username is teleport username
	Username string
	// Impersonator is set when a user requests certificate for another user
	Impersonator string
	// AllowedLogins is a list of SSH principals
	AllowedLogins []string
	// PermitX11Forwarding permits X11 forwarding for this cert
	PermitX11Forwarding bool
	// PermitAgentForwarding permits agent forwarding for this cert
	PermitAgentForwarding bool
	// PermitPortForwarding permits port forwarding.
	PermitPortForwarding bool
	// PermitFileCopying permits the use of SCP/SFTP.
	PermitFileCopying bool
	// Roles is a list of roles assigned to this user
	Roles []string
	// CertificateFormat is the format of the SSH certificate.
	CertificateFormat string
	// RouteToCluster specifies the target cluster
	// if present in the certificate, will be used
	// to route the requests to
	RouteToCluster string
	// Traits hold claim data used to populate a role at runtime.
	Traits wrappers.Traits
	// ActiveRequests tracks privilege escalation requests applied during
	// certificate construction.
	ActiveRequests RequestIDs
	// MFAVerified is the UUID of an MFA device when this Identity was
	// confirmed immediately after an MFA check.
	MFAVerified string
	// PreviousIdentityExpires is the expiry time of the identity/cert that this
	// identity/cert was derived from. It is used to determine a session's hard
	// deadline in cases where both require_session_mfa and disconnect_expired_cert
	// are enabled. See https://github.com/gravitational/teleport/issues/18544.
	PreviousIdentityExpires time.Time
	// LoginIP is an observed IP of the client on the moment of certificate creation.
	LoginIP string
	// PinnedIP is an IP from which client must communicate with Teleport.
	PinnedIP string
	// DisallowReissue flags that any attempt to request new certificates while
	// authenticated with this cert should be denied.
	DisallowReissue bool
	// CertificateExtensions are user configured ssh key extensions
	CertificateExtensions []*types.CertExtension
	// Renewable indicates this certificate is renewable.
	Renewable bool
	// Generation counts the number of times a certificate has been renewed.
	Generation uint64
	// BotName is set to the name of the bot, if the user is a Machine ID bot user.
	// Empty for human users.
	BotName string
	// AllowedResourceIDs lists the resources the user should be able to access.
	AllowedResourceIDs string
	// ConnectionDiagnosticID references the ConnectionDiagnostic that we should use to append traces when testing a Connection.
	ConnectionDiagnosticID string
	// PrivateKeyPolicy is the private key policy supported by this certificate.
	PrivateKeyPolicy keys.PrivateKeyPolicy
	// DeviceID is the trusted device identifier.
	DeviceID string
	// DeviceAssetTag is the device inventory identifier.
	DeviceAssetTag string
	// DeviceCredentialID is the identifier for the credential used by the device
	// to authenticate itself.
	DeviceCredentialID string
}

// CheckAndSetDefaults checks the user certificate parameters
func (c *UserCertParams) CheckAndSetDefaults() error {
	if c.CASigner == nil {
		return trace.BadParameter("CASigner is required")
	}
	if c.TTL < apidefaults.MinCertDuration {
		c.TTL = apidefaults.MinCertDuration
	}
	if len(c.AllowedLogins) == 0 {
		return trace.BadParameter("AllowedLogins are required")
	}
	return nil
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
		return nil, trace.BadParameter(err.Error())
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
			return nil, trace.BadParameter(err.Error())
		}

		if err := ValidateCertAuthority(&ca); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			ca.SetResourceID(cfg.ID)
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
		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, certAuthority))
	default:
		return nil, trace.BadParameter("unrecognized certificate authority version %T", certAuthority)
	}
}
