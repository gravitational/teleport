/*
Copyright 2017-2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"crypto"
	"crypto/x509"
	"encoding/json"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// CertAuthoritiesEquivalent checks if a pair of certificate authority resources are equivalent.
// This differs from normal equality only in that resource IDs are ignored.
func CertAuthoritiesEquivalent(lhs, rhs types.CertAuthority) bool {
	return cmp.Equal(lhs, rhs, cmpopts.IgnoreFields(types.Metadata{}, "ID"))
}

// ValidateCertAuthority validates the CertAuthority
func ValidateCertAuthority(ca types.CertAuthority) (err error) {
	if err = ca.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	switch ca.GetType() {
	case types.UserCA, types.HostCA:
		err = checkUserOrHostCA(ca)
	case types.DatabaseCA:
		err = checkDatabaseCA(ca)
	case types.JWTSigner:
		err = checkJWTKeys(ca)
	default:
		return trace.BadParameter("invalid CA type %q", ca.GetType())
	}
	return trace.Wrap(err)
}

func checkUserOrHostCA(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}
	if len(ca.Spec.ActiveKeys.SSH) == 0 && len(ca.Spec.CheckingKeys) == 0 {
		return trace.BadParameter("certificate authority missing SSH key pairs")
	}
	if len(ca.Spec.ActiveKeys.TLS) == 0 && len(ca.Spec.TLSKeyPairs) == 0 {
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

func checkDatabaseCA(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}

	if len(ca.Spec.ActiveKeys.TLS) == 0 && len(ca.Spec.TLSKeyPairs) == 0 {
		return trace.BadParameter("DB certificate authority missing TLS key pairs")
	}

	for _, pair := range ca.GetTrustedTLSKeyPairs() {
		if len(pair.Key) > 0 && pair.KeyType == types.PrivateKeyType_RAW {
			_, err := utils.ParsePrivateKey(pair.Key)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		_, err := tlsca.ParseCertificatePEM(pair.Cert)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func checkJWTKeys(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown CA type %T", cai)
	}
	// Check that some JWT keys have been set on the CA.
	if len(ca.Spec.ActiveKeys.JWT) == 0 && len(ca.Spec.JWTKeyPairs) == 0 {
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
	// CASigningAlg is the signature algorithm used by the CA private key.
	CASigningAlg string
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
	if c.CASigner == nil || c.CASigningAlg == "" {
		return trace.BadParameter("CASigner and CASigningAlg are required")
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

// ChangePasswordReq defines a request to change user password
type ChangePasswordReq struct {
	// User is user ID
	User string
	// OldPassword is user current password
	OldPassword []byte `json:"old_password"`
	// NewPassword is user new password
	NewPassword []byte `json:"new_password"`
	// SecondFactorToken is user 2nd factor token
	SecondFactorToken string `json:"second_factor_token"`
	// U2FSignResponse is U2F sign response
	U2FSignResponse *u2f.AuthenticateChallengeResponse `json:"u2f_sign_response"`
	// WebauthnResponse is Webauthn sign response
	WebauthnResponse *wanlib.CredentialAssertionResponse `json:"webauthn_response"`
}

// UserCertParams defines OpenSSH user certificate parameters
type UserCertParams struct {
	// CASigner is the signer that will sign the public key of the user with the CA private key
	CASigner ssh.Signer
	// CASigningAlg is the signature algorithm used by the CA private key.
	CASigningAlg string
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
	// ClientIP is an IP of the client to embed in the certificate.
	ClientIP string
	// DisallowReissue flags that any attempt to request new certificates while
	// authenticated with this cert should be denied.
	DisallowReissue bool
}

// CheckAndSetDefaults checks the user certificate parameters
func (c *UserCertParams) CheckAndSetDefaults() error {
	if c.CASigner == nil || c.CASigningAlg == "" {
		return trace.BadParameter("CASigner and CASigningAlg are required")
	}
	if c.TTL < defaults.MinCertDuration {
		c.TTL = defaults.MinCertDuration
	}
	if len(c.AllowedLogins) == 0 {
		return trace.BadParameter("AllowedLogins are required")
	}
	return nil
}

// CertPoolFromCertAuthorities returns certificate pools from TLS certificates
// set up in the certificate authorities list
func CertPoolFromCertAuthorities(cas []types.CertAuthority) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	for _, ca := range cas {
		keyPairs := ca.GetTrustedTLSKeyPairs()
		if len(keyPairs) == 0 {
			continue
		}
		for _, keyPair := range keyPairs {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			certPool.AddCert(cert)
		}
	}
	return certPool, nil
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
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *certAuthority
			copy.SetResourceID(0)
			certAuthority = &copy
		}
		if err := SyncCertAuthorityKeys(certAuthority); err != nil {
			return nil, trace.Wrap(err, "failed to sync CertAuthority key formats for %v: %v", certAuthority, err)
		}
		return utils.FastMarshal(certAuthority)
	default:
		return nil, trace.BadParameter("unrecognized certificate authority version %T", certAuthority)
	}
}

// CertAuthorityNeedsMigration returns true if the given CertAuthority needs to be migrated
func CertAuthorityNeedsMigration(cai types.CertAuthority) (bool, error) {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return false, trace.BadParameter("unknown type %T", cai)
	}
	haveOldCAKeys := len(ca.Spec.CheckingKeys) > 0 || len(ca.Spec.TLSKeyPairs) > 0 || len(ca.Spec.JWTKeyPairs) > 0
	haveNewCAKeys := len(ca.Spec.ActiveKeys.SSH) > 0 || len(ca.Spec.ActiveKeys.TLS) > 0 || len(ca.Spec.ActiveKeys.JWT) > 0
	return haveOldCAKeys && !haveNewCAKeys, nil
}

// SyncCertAuthorityKeys backfills the old or new key formats, if one of them
// is empty. If both formats are present, SyncCertAuthorityKeys does nothing.
func SyncCertAuthorityKeys(cai types.CertAuthority) error {
	ca, ok := cai.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("unknown type %T", cai)
	}
	haveOldCAKeys := len(ca.Spec.CheckingKeys) > 0 || len(ca.Spec.TLSKeyPairs) > 0 || len(ca.Spec.JWTKeyPairs) > 0
	haveNewCAKeys := len(ca.Spec.ActiveKeys.SSH) > 0 || len(ca.Spec.ActiveKeys.TLS) > 0 || len(ca.Spec.ActiveKeys.JWT) > 0
	switch {
	case haveOldCAKeys && !haveNewCAKeys:
		return trace.Wrap(fillNewCertAuthorityKeys(ca))
	case !haveOldCAKeys && haveNewCAKeys:
		return trace.Wrap(fillOldCertAuthorityKeys(ca))
	}
	return nil
}

func fillNewCertAuthorityKeys(ca *types.CertAuthorityV2) error {
	// Reset any old state.
	ca.Spec.ActiveKeys = types.CAKeySet{}
	ca.Spec.AdditionalTrustedKeys = types.CAKeySet{}

	// Convert all the keypair fields to new format.

	// SigningKeys key may be missing in the CA from a remote cluster.
	if len(ca.Spec.SigningKeys) > 0 && len(ca.Spec.SigningKeys) != len(ca.Spec.CheckingKeys) {
		return trace.BadParameter("mis-matched SSH private (%d) and public (%d) key counts", len(ca.Spec.SigningKeys), len(ca.Spec.CheckingKeys))
	}
	for i := range ca.Spec.CheckingKeys {
		kp := &types.SSHKeyPair{
			PrivateKeyType: types.PrivateKeyType_RAW,
			PublicKey:      apiutils.CopyByteSlice(ca.Spec.CheckingKeys[i]),
		}
		if len(ca.Spec.SigningKeys) > 0 {
			kp.PrivateKey = apiutils.CopyByteSlice(ca.Spec.SigningKeys[i])
		}
		ca.Spec.ActiveKeys.SSH = append(ca.Spec.ActiveKeys.SSH, kp)
	}
	for _, kp := range ca.Spec.TLSKeyPairs {
		ca.Spec.ActiveKeys.TLS = append(ca.Spec.ActiveKeys.TLS, kp.Clone())
	}
	for _, kp := range ca.Spec.JWTKeyPairs {
		ca.Spec.ActiveKeys.JWT = append(ca.Spec.ActiveKeys.JWT, kp.Clone())
	}
	return nil
}

func fillOldCertAuthorityKeys(ca *types.CertAuthorityV2) error {
	// Reset any old state.
	ca.Spec.SigningKeys = nil
	ca.Spec.CheckingKeys = nil
	ca.Spec.TLSKeyPairs = nil
	ca.Spec.JWTKeyPairs = nil

	// Convert all the keypair fields to new format.
	for _, ks := range []types.CAKeySet{ca.Spec.ActiveKeys, ca.Spec.AdditionalTrustedKeys} {
		for _, kp := range ks.SSH {
			ca.Spec.CheckingKeys = append(ca.Spec.CheckingKeys, apiutils.CopyByteSlice(kp.PublicKey))
			// PrivateKey may be empty.
			if len(kp.PrivateKey) > 0 {
				ca.Spec.SigningKeys = append(ca.Spec.SigningKeys, apiutils.CopyByteSlice(kp.PrivateKey))
			}
		}
		for _, kp := range ks.TLS {
			ca.Spec.TLSKeyPairs = append(ca.Spec.TLSKeyPairs, *kp.Clone())
		}
		for _, kp := range ks.JWT {
			ca.Spec.JWTKeyPairs = append(ca.Spec.JWTKeyPairs, *kp.Clone())
		}
	}
	return nil
}
