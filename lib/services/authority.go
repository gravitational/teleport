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
	"crypto/x509"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/wrappers"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/tstranex/u2f"
	"golang.org/x/crypto/ssh"
)

// HostCertParams defines all parameters needed to generate a host certificate
type HostCertParams struct {
	// PrivateCASigningKey is the private key of the CA that will sign the public key of the host
	PrivateCASigningKey []byte
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
	// Roles identifies the roles of a Teleport instance
	Roles teleport.Roles
	// TTL defines how long a certificate is valid for
	TTL time.Duration
}

// Check checks parameters for errors
func (c HostCertParams) Check() error {
	if len(c.PrivateCASigningKey) == 0 || c.CASigningAlg == "" {
		return trace.BadParameter("PrivateCASigningKey and CASigningAlg are required")
	}
	if c.HostID == "" && len(c.Principals) == 0 {
		return trace.BadParameter("HostID [%q] or Principals [%q] are required",
			c.HostID, c.Principals)
	}
	if c.ClusterName == "" {
		return trace.BadParameter("ClusterName [%q] is required", c.ClusterName)
	}

	if err := c.Roles.Check(); err != nil {
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
	U2FSignResponse *u2f.SignResponse `json:"u2f_sign_response"`
}

// UserCertParams defines OpenSSH user certificate parameters
type UserCertParams struct {
	// PrivateCASigningKey is the private key of the CA that will sign the public key of the user
	PrivateCASigningKey []byte
	// CASigningAlg is the signature algorithm used by the CA private key.
	CASigningAlg string
	// PublicUserKey is the public key of the user
	PublicUserKey []byte
	// TTL defines how long a certificate is valid for
	TTL time.Duration
	// Username is teleport username
	Username string
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
}

func (c UserCertParams) Check() error {
	if len(c.PrivateCASigningKey) == 0 || c.CASigningAlg == "" {
		return trace.BadParameter("PrivateCASigningKey and CASigningAlg are required")
	}
	if c.TTL < defaults.MinCertDuration {
		return trace.BadParameter("TTL can't be less than %v", defaults.MinCertDuration)
	}
	if len(c.AllowedLogins) == 0 {
		return trace.BadParameter("AllowedLogins are required")
	}
	return nil
}

// CertRoles defines certificate roles
type CertRoles struct {
	// Version is current version of the roles
	Version string `json:"version"`
	// Roles is a list of roles
	Roles []string `json:"roles"`
}

// CertRolesSchema defines cert roles schema
const CertRolesSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "version": {"type": "string"},
    "roles": {
      "type": "array",
      "items": {
        "type": "string"
      }
    }
  }
}`

// MarshalCertRoles marshal roles list to OpenSSH
func MarshalCertRoles(roles []string) (string, error) {
	out, err := json.Marshal(CertRoles{Version: V1, Roles: roles})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(out), err
}

// UnmarshalCertRoles marshals roles list to OpenSSH
func UnmarshalCertRoles(data string) ([]string, error) {
	var certRoles CertRoles
	if err := utils.UnmarshalWithSchema(CertRolesSchema, &certRoles, []byte(data)); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return certRoles.Roles, nil
}

// CertAuthority is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthority interface {
	// ResourceWithSecrets sets common resource properties
	ResourceWithSecrets
	// GetID returns certificate authority ID -
	// combined type and name
	GetID() CertAuthID
	// GetType returns user or host certificate authority
	GetType() CertAuthType
	// GetClusterName returns cluster name this cert authority
	// is associated with
	GetClusterName() string
	// GetCheckingKeys returns public keys to check signature
	GetCheckingKeys() [][]byte
	// GetSigningKeys returns signing keys
	GetSigningKeys() [][]byte
	// CombinedMapping is used to specify combined mapping from legacy property Roles
	// and new property RoleMap
	CombinedMapping() RoleMap
	// GetRoleMap returns role map property
	GetRoleMap() RoleMap
	// SetRoleMap sets role map
	SetRoleMap(m RoleMap)
	// GetRoles returns a list of roles assumed by users signed by this CA
	GetRoles() []string
	// SetRoles sets assigned roles for this certificate authority
	SetRoles(roles []string)
	// FirstSigningKey returns first signing key or returns error if it's not here
	// The first key is returned because multiple keys can exist during key rotation.
	FirstSigningKey() ([]byte, error)
	// Check checks object for errors
	Check() error
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
	// SetSigningKeys sets signing keys
	SetSigningKeys([][]byte) error
	// SetCheckingKeys sets signing keys
	SetCheckingKeys([][]byte) error
	// AddRole adds a role to ca role list
	AddRole(name string)
	// Checkers returns public keys that can be used to check cert authorities
	Checkers() ([]ssh.PublicKey, error)
	// Signers returns a list of signers that could be used to sign keys
	Signers() ([]ssh.Signer, error)
	// V1 returns V1 version of the resource
	V1() *CertAuthorityV1
	// V2 returns V2 version of the resource
	V2() *CertAuthorityV2
	// String returns human readable version of the CertAuthority
	String() string
	// TLSCA returns first TLS certificate authority from the list of key pairs
	TLSCA() (*tlsca.CertAuthority, error)
	// SetTLSKeyPairs sets TLS key pairs
	SetTLSKeyPairs(keyPairs []TLSKeyPair)
	// GetTLSKeyPairs returns first PEM encoded TLS cert
	GetTLSKeyPairs() []TLSKeyPair
	// GetRotation returns rotation state.
	GetRotation() Rotation
	// SetRotation sets rotation state.
	SetRotation(Rotation)
	// GetSigningAlg returns the signing algorithm used by signing keys.
	GetSigningAlg() string
	// SetSigningAlg sets the signing algorithm used by signing keys.
	SetSigningAlg(string)
	// Clone returns a copy of the cert authority object.
	Clone() CertAuthority
}

// CertPoolFromCertAuthorities returns certificate pools from TLS certificates
// set up in the certificate authorities list
func CertPoolFromCertAuthorities(cas []CertAuthority) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	for _, ca := range cas {
		keyPairs := ca.GetTLSKeyPairs()
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
		return certPool, nil
	}
	return certPool, nil
}

// CertPool returns certificate pools from TLS certificates
// set up in the certificate authority
func CertPool(ca CertAuthority) (*x509.CertPool, error) {
	keyPairs := ca.GetTLSKeyPairs()
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

// TLSCerts returns TLS certificates from CA
func TLSCerts(ca CertAuthority) [][]byte {
	pairs := ca.GetTLSKeyPairs()
	out := make([][]byte, len(pairs))
	for i, pair := range pairs {
		out[i] = append([]byte{}, pair.Cert...)
	}
	return out
}

// NewCertAuthority returns new cert authority
func NewCertAuthority(
	caType CertAuthType,
	clusterName string,
	signingKeys [][]byte,
	checkingKeys [][]byte,
	roles []string,
	signingAlg CertAuthoritySpecV2_SigningAlgType,
) CertAuthority {
	return &CertAuthorityV2{
		Kind:    KindCertAuthority,
		Version: V2,
		SubKind: string(caType),
		Metadata: Metadata{
			Name:      clusterName,
			Namespace: defaults.Namespace,
		},
		Spec: CertAuthoritySpecV2{
			Roles:        roles,
			Type:         caType,
			ClusterName:  clusterName,
			CheckingKeys: checkingKeys,
			SigningKeys:  signingKeys,
			SigningAlg:   signingAlg,
		},
	}
}

// CertAuthoritiesToV1 converts list of cert authorities to V1 slice
func CertAuthoritiesToV1(in []CertAuthority) ([]CertAuthorityV1, error) {
	out := make([]CertAuthorityV1, len(in))
	type cav1 interface {
		V1() *CertAuthorityV1
	}
	for i, ca := range in {
		v1, ok := ca.(cav1)
		if !ok {
			return nil, trace.BadParameter("could not transform object to V1")
		}
		out[i] = *(v1.V1())
	}
	return out, nil
}

// GetVersion returns resource version
func (c *CertAuthorityV2) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *CertAuthorityV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *CertAuthorityV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *CertAuthorityV2) SetSubKind(s string) {
	c.SubKind = s
}

// Clone returns a copy of the cert authority object.
func (c *CertAuthorityV2) Clone() CertAuthority {
	out := *c
	out.Spec.CheckingKeys = utils.CopyByteSlices(c.Spec.CheckingKeys)
	out.Spec.SigningKeys = utils.CopyByteSlices(c.Spec.SigningKeys)
	for i, kp := range c.Spec.TLSKeyPairs {
		out.Spec.TLSKeyPairs[i] = TLSKeyPair{
			Key:  utils.CopyByteSlice(kp.Key),
			Cert: utils.CopyByteSlice(kp.Cert),
		}
	}
	out.Spec.Roles = utils.CopyStrings(c.Spec.Roles)
	return &out
}

// GetRotation returns rotation state.
func (c *CertAuthorityV2) GetRotation() Rotation {
	if c.Spec.Rotation == nil {
		return Rotation{}
	}
	return *c.Spec.Rotation
}

// SetRotation sets rotation state.
func (c *CertAuthorityV2) SetRotation(r Rotation) {
	c.Spec.Rotation = &r
}

// TLSCA returns TLS certificate authority
func (c *CertAuthorityV2) TLSCA() (*tlsca.CertAuthority, error) {
	if len(c.Spec.TLSKeyPairs) == 0 {
		return nil, trace.BadParameter("no TLS key pairs found for certificate authority")
	}
	return tlsca.New(c.Spec.TLSKeyPairs[0].Cert, c.Spec.TLSKeyPairs[0].Key)
}

// SetTLSPrivateKey sets TLS key pairs
func (c *CertAuthorityV2) SetTLSKeyPairs(pairs []TLSKeyPair) {
	c.Spec.TLSKeyPairs = pairs
}

// GetTLSPrivateKey returns TLS key pairs
func (c *CertAuthorityV2) GetTLSKeyPairs() []TLSKeyPair {
	return c.Spec.TLSKeyPairs
}

// GetMetadata returns object metadata
func (c *CertAuthorityV2) GetMetadata() Metadata {
	return c.Metadata
}

// SetExpiry sets expiry time for the object
func (c *CertAuthorityV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expires returns object expiry setting
func (c *CertAuthorityV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (c *CertAuthorityV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetResourceID returns resource ID
func (c *CertAuthorityV2) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *CertAuthorityV2) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// WithoutSecrets returns an instance of resource without secrets.
func (c *CertAuthorityV2) WithoutSecrets() Resource {
	c2 := c.Clone()
	RemoveCASecrets(c2)
	return c2
}

// V2 returns V2 version of the resouirce - itself
func (c *CertAuthorityV2) V2() *CertAuthorityV2 {
	return c
}

// String returns human readable version of the CertAuthorityV2.
func (c *CertAuthorityV2) String() string {
	return fmt.Sprintf("CA(name=%v, type=%v)", c.GetClusterName(), c.GetType())
}

// V1 returns V1 version of the object
func (c *CertAuthorityV2) V1() *CertAuthorityV1 {
	return &CertAuthorityV1{
		Type:         c.Spec.Type,
		DomainName:   c.Spec.ClusterName,
		CheckingKeys: c.Spec.CheckingKeys,
		SigningKeys:  c.Spec.SigningKeys,
	}
}

// AddRole adds a role to ca role list
func (ca *CertAuthorityV2) AddRole(name string) {
	for _, r := range ca.Spec.Roles {
		if r == name {
			return
		}
	}
	ca.Spec.Roles = append(ca.Spec.Roles, name)
}

// GetSigning keys returns signing keys
func (ca *CertAuthorityV2) GetSigningKeys() [][]byte {
	return ca.Spec.SigningKeys
}

// SetSigningKeys sets signing keys
func (ca *CertAuthorityV2) SetSigningKeys(keys [][]byte) error {
	ca.Spec.SigningKeys = keys
	return nil
}

// SetCheckingKeys sets SSH public keys
func (ca *CertAuthorityV2) SetCheckingKeys(keys [][]byte) error {
	ca.Spec.CheckingKeys = keys
	return nil
}

// GetID returns certificate authority ID -
// combined type and name
func (ca *CertAuthorityV2) GetID() CertAuthID {
	return CertAuthID{Type: ca.Spec.Type, DomainName: ca.Metadata.Name}
}

// SetName sets cert authority name
func (ca *CertAuthorityV2) SetName(name string) {
	ca.Metadata.SetName(name)
}

// GetName returns cert authority name
func (ca *CertAuthorityV2) GetName() string {
	return ca.Metadata.Name
}

// GetType returns user or host certificate authority
func (ca *CertAuthorityV2) GetType() CertAuthType {
	return ca.Spec.Type
}

// GetClusterName returns cluster name this cert authority
// is associated with.
func (ca *CertAuthorityV2) GetClusterName() string {
	return ca.Spec.ClusterName
}

// GetCheckingKeys returns public keys to check signature
func (ca *CertAuthorityV2) GetCheckingKeys() [][]byte {
	return ca.Spec.CheckingKeys
}

// GetRoles returns a list of roles assumed by users signed by this CA
func (ca *CertAuthorityV2) GetRoles() []string {
	return ca.Spec.Roles
}

// SetRoles sets assigned roles for this certificate authority
func (ca *CertAuthorityV2) SetRoles(roles []string) {
	ca.Spec.Roles = roles
}

// CombinedMapping is used to specify combined mapping from legacy property Roles
// and new property RoleMap
func (ca *CertAuthorityV2) CombinedMapping() RoleMap {
	if len(ca.Spec.Roles) != 0 {
		return RoleMap([]RoleMapping{{Remote: Wildcard, Local: ca.Spec.Roles}})
	}
	return RoleMap(ca.Spec.RoleMap)
}

// GetRoleMap returns role map property
func (ca *CertAuthorityV2) GetRoleMap() RoleMap {
	return RoleMap(ca.Spec.RoleMap)
}

// SetRoleMap sets role map
func (c *CertAuthorityV2) SetRoleMap(m RoleMap) {
	c.Spec.RoleMap = []RoleMapping(m)
}

// FirstSigningKey returns first signing key or returns error if it's not here
func (ca *CertAuthorityV2) FirstSigningKey() ([]byte, error) {
	if len(ca.Spec.SigningKeys) == 0 {
		return nil, trace.NotFound("%v has no signing keys", ca.Metadata.Name)
	}
	return ca.Spec.SigningKeys[0], nil
}

// ID returns id (consisting of domain name and type) that
// identifies the authority this key belongs to
func (ca *CertAuthorityV2) ID() *CertAuthID {
	return &CertAuthID{DomainName: ca.Spec.ClusterName, Type: ca.Spec.Type}
}

// Checkers returns public keys that can be used to check cert authorities
func (ca *CertAuthorityV2) Checkers() ([]ssh.PublicKey, error) {
	out := make([]ssh.PublicKey, 0, len(ca.Spec.CheckingKeys))
	for _, keyBytes := range ca.Spec.CheckingKeys {
		key, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
		if err != nil {
			return nil, trace.BadParameter("invalid authority public key (len=%d): %v", len(keyBytes), err)
		}
		out = append(out, key)
	}
	return out, nil
}

// Signers returns a list of signers that could be used to sign keys.
func (ca *CertAuthorityV2) Signers() ([]ssh.Signer, error) {
	out := make([]ssh.Signer, 0, len(ca.Spec.SigningKeys))
	for _, keyBytes := range ca.Spec.SigningKeys {
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		signer = sshutils.AlgSigner(signer, ca.GetSigningAlg())
		out = append(out, signer)
	}
	return out, nil
}

func (ca *CertAuthorityV2) GetSigningAlg() string {
	switch ca.Spec.SigningAlg {
	// UNKNOWN algorithm can come from a cluster that existed before SigningAlg
	// field was added. Default to RSA-SHA1 to match the implicit algorithm
	// used in those clusters.
	case CertAuthoritySpecV2_RSA_SHA1, CertAuthoritySpecV2_UNKNOWN:
		return ssh.SigAlgoRSA
	case CertAuthoritySpecV2_RSA_SHA2_256:
		return ssh.SigAlgoRSASHA2256
	case CertAuthoritySpecV2_RSA_SHA2_512:
		return ssh.SigAlgoRSASHA2512
	default:
		return ""
	}
}

func (ca *CertAuthorityV2) SetSigningAlg(alg string) {
	ca.Spec.SigningAlg = ParseSigningAlg(alg)
}

// Check checks if all passed parameters are valid
func (ca *CertAuthorityV2) Check() error {
	err := ca.ID().Check()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = ca.Checkers()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = ca.Signers()
	if err != nil {
		return trace.Wrap(err)
	}
	// This is to force users to migrate
	if len(ca.Spec.Roles) != 0 && len(ca.Spec.RoleMap) != 0 {
		return trace.BadParameter("should set either 'roles' or 'role_map', not both")
	}
	if err := RoleMap(ca.Spec.RoleMap).Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (ca *CertAuthorityV2) CheckAndSetDefaults() error {
	err := ca.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	err = ca.Check()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ParseSigningAlg converts the SSH signature algorithm strings to the
// corresponding proto enum value.
//
// alg should be one of ssh.SigAlgo* constants. If it's not one of those
// constants, CertAuthoritySpecV2_UNKNOWN is returned.
func ParseSigningAlg(alg string) CertAuthoritySpecV2_SigningAlgType {
	switch alg {
	case ssh.SigAlgoRSA:
		return CertAuthoritySpecV2_RSA_SHA1
	case ssh.SigAlgoRSASHA2256:
		return CertAuthoritySpecV2_RSA_SHA2_256
	case ssh.SigAlgoRSASHA2512:
		return CertAuthoritySpecV2_RSA_SHA2_512
	default:
		return CertAuthoritySpecV2_UNKNOWN
	}
}

// RemoveCASecrets removes secret values and keys
// from the certificate authority
func RemoveCASecrets(ca CertAuthority) {
	ca.SetSigningKeys(nil)
	keyPairs := ca.GetTLSKeyPairs()
	for i := range keyPairs {
		keyPairs[i].Key = nil
	}
	ca.SetTLSKeyPairs(keyPairs)
}

const (
	// RotationStateStandby is initial status of the rotation -
	// nothing is being rotated.
	RotationStateStandby = "standby"
	// RotationStateInProgress - that rotation is in progress.
	RotationStateInProgress = "in_progress"
	// RotationPhaseStandby is the initial phase of the rotation
	// it means no operations have started.
	RotationPhaseStandby = "standby"
	// RotationPhaseInit = is a phase of the rotation
	// when new certificate authoirty is issued, but not used
	// It is necessary for remote trusted clusters to fetch the
	// new certificate authority, otherwise the new clients
	// will reject it
	RotationPhaseInit = "init"
	// RotationPhaseUpdateClients is a phase of the rotation
	// when client credentials will have to be updated and reloaded
	// but servers will use and respond with old credentials
	// because clients have no idea about new credentials at first.
	RotationPhaseUpdateClients = "update_clients"
	// RotationPhaseUpdateServers is a phase of the rotation
	// when servers will have to reload and should start serving
	// TLS and SSH certificates signed by new CA.
	RotationPhaseUpdateServers = "update_servers"
	// RotationPhaseRollback means that rotation is rolling
	// back to the old certificate authority.
	RotationPhaseRollback = "rollback"
	// RotationModeManual is a manual rotation mode when all phases
	// are set by the operator.
	RotationModeManual = "manual"
	// RotationModeAuto is set to go through all phases by the schedule.
	RotationModeAuto = "auto"
)

// RotatePhases lists all supported rotation phases
var RotatePhases = []string{
	RotationPhaseInit,
	RotationPhaseStandby,
	RotationPhaseUpdateClients,
	RotationPhaseUpdateServers,
	RotationPhaseRollback,
}

// Matches returns true if this state rotation matches
// external rotation state, phase and rotation ID should match,
// notice that matches does not behave like Equals because it does not require
// all fields to be the same.
func (s *Rotation) Matches(rotation Rotation) bool {
	return s.CurrentID == rotation.CurrentID && s.State == rotation.State && s.Phase == rotation.Phase
}

// LastRotatedDescription returns human friendly description.
func (r *Rotation) LastRotatedDescription() string {
	if r.LastRotated.IsZero() {
		return "never updated"
	}
	return fmt.Sprintf("last rotated %v", r.LastRotated.Format(teleport.HumanDateFormatSeconds))
}

// PhaseDescription returns human friendly description of a current rotation phase.
func (r *Rotation) PhaseDescription() string {
	switch r.Phase {
	case RotationPhaseInit:
		return "initialized"
	case RotationPhaseStandby, "":
		return "on standby"
	case RotationPhaseUpdateClients:
		return "rotating clients"
	case RotationPhaseUpdateServers:
		return "rotating servers"
	case RotationPhaseRollback:
		return "rolling back"
	default:
		return fmt.Sprintf("unknown phase: %q", r.Phase)
	}
}

// String returns user friendly information about certificate authority.
func (r *Rotation) String() string {
	switch r.State {
	case "", RotationStateStandby:
		if r.LastRotated.IsZero() {
			return "never updated"
		}
		return fmt.Sprintf("rotated %v", r.LastRotated.Format(teleport.HumanDateFormatSeconds))
	case RotationStateInProgress:
		return fmt.Sprintf("%v (mode: %v, started: %v, ending: %v)",
			r.PhaseDescription(),
			r.Mode,
			r.Started.Format(teleport.HumanDateFormatSeconds),
			r.Started.Add(r.GracePeriod.Duration()).Format(teleport.HumanDateFormatSeconds),
		)
	default:
		return "unknown"
	}
}

// CheckAndSetDefaults checks and sets default rotation parameters.
func (r *Rotation) CheckAndSetDefaults(clock clockwork.Clock) error {
	switch r.Phase {
	case "", RotationPhaseRollback, RotationPhaseUpdateClients, RotationPhaseUpdateServers:
	default:
		return trace.BadParameter("unsupported phase: %q", r.Phase)
	}
	switch r.Mode {
	case "", RotationModeAuto, RotationModeManual:
	default:
		return trace.BadParameter("unsupported mode: %q", r.Mode)
	}
	switch r.State {
	case "":
		r.State = RotationStateStandby
	case RotationStateStandby:
	case RotationStateInProgress:
		if r.CurrentID == "" {
			return trace.BadParameter("set 'current_id' parameter for in progress rotation")
		}
		if r.Started.IsZero() {
			return trace.BadParameter("set 'started' parameter for in progress rotation")
		}
	default:
		return trace.BadParameter(
			"unsupported rotation 'state': %q, supported states are: %q, %q",
			r.State, RotationStateStandby, RotationStateInProgress)
	}
	return nil
}

// GenerateSchedule generates schedule based on the time period, using
// even time periods between rotation phases.
func GenerateSchedule(clock clockwork.Clock, gracePeriod time.Duration) (*RotationSchedule, error) {
	if gracePeriod <= 0 {
		return nil, trace.BadParameter("invalid grace period %q, provide value > 0", gracePeriod)
	}
	return &RotationSchedule{
		UpdateClients: clock.Now().UTC().Add(gracePeriod / 3).UTC(),
		UpdateServers: clock.Now().UTC().Add((gracePeriod * 2) / 3).UTC(),
		Standby:       clock.Now().UTC().Add(gracePeriod).UTC(),
	}, nil
}

// CheckAndSetDefaults checks and sets default values of the rotation schedule.
func (s *RotationSchedule) CheckAndSetDefaults(clock clockwork.Clock) error {
	if s.UpdateServers.IsZero() {
		return trace.BadParameter("phase %q has no time switch scheduled", RotationPhaseUpdateServers)
	}
	if s.Standby.IsZero() {
		return trace.BadParameter("phase %q has no time switch scheduled", RotationPhaseStandby)
	}
	if s.Standby.Before(s.UpdateServers) {
		return trace.BadParameter("phase %q can not be scheduled before %q", RotationPhaseStandby, RotationPhaseUpdateServers)
	}
	if s.UpdateServers.Before(clock.Now()) {
		return trace.BadParameter("phase %q can not be scheduled in the past", RotationPhaseUpdateServers)
	}
	if s.Standby.Before(clock.Now()) {
		return trace.BadParameter("phase %q can not be scheduled in the past", RotationPhaseStandby)
	}
	return nil
}

// CertAuthoritySpecV2Schema is JSON schema for cert authority V2
const CertAuthoritySpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["type", "cluster_name", "checking_keys"],
  "properties": {
    "type": {"type": "string"},
    "cluster_name": {"type": "string"},
    "checking_keys": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "signing_keys": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "roles": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "tls_key_pairs":  {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
           "cert": {"type": "string"},
           "key": {"type": "string"}
        }
      }
    },
    "signing_alg": {"type": "integer"},
    "rotation": %v,
    "role_map": %v
  }
}`

// RotationSchema is a JSON validation schema of the CA rotation state object.
const RotationSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "state": {"type": "string"},
    "phase": {"type": "string"},
    "mode": {"type": "string"},
    "current_id": {"type": "string"},
    "started": {"type": "string"},
    "grace_period": {"type": "string"},
    "last_rotated": {"type": "string"},
    "schedule": {
      "type": "object",
      "properties": {
        "update_clients": {"type": "string"},
        "update_servers": {"type": "string"},
        "standby": {"type": "string"}
      }
    }
  }
}`

// CertAuthorityV1 is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthorityV1 struct {
	// Type is either user or host certificate authority
	Type CertAuthType `json:"type"`
	// DomainName identifies domain name this authority serves,
	// for host authorities that means base hostname of all servers,
	// for user authorities that means organization name
	DomainName string `json:"domain_name"`
	// Checkers is a list of SSH public keys that can be used to check
	// certificate signatures
	CheckingKeys [][]byte `json:"checking_keys"`
	// SigningKeys is a list of private keys used for signing
	SigningKeys [][]byte `json:"signing_keys"`
	// AllowedLogins is a list of allowed logins for users within
	// this certificate authority
	AllowedLogins []string `json:"allowed_logins"`
}

// CombinedMapping is used to specify combined mapping from legacy property Roles
// and new property RoleMap
func (ca *CertAuthorityV1) CombinedMapping() RoleMap {
	return []RoleMapping{}
}

// GetRoleMap returns role map property
func (ca *CertAuthorityV1) GetRoleMap() RoleMap {
	return nil
}

// SetRoleMap sets role map
func (c *CertAuthorityV1) SetRoleMap(m RoleMap) {
}

// V1 returns V1 version of the resource
func (c *CertAuthorityV1) V1() *CertAuthorityV1 {
	return c
}

// V2 returns V2 version of the resource
func (c *CertAuthorityV1) V2() *CertAuthorityV2 {
	return &CertAuthorityV2{
		Kind:    KindCertAuthority,
		Version: V2,
		Metadata: Metadata{
			Name:      c.DomainName,
			Namespace: defaults.Namespace,
		},
		Spec: CertAuthoritySpecV2{
			Type:         c.Type,
			ClusterName:  c.DomainName,
			CheckingKeys: c.CheckingKeys,
			SigningKeys:  c.SigningKeys,
		},
	}
}

// String returns human readable version of the CertAuthorityV1.
func (c *CertAuthorityV1) String() string {
	return fmt.Sprintf("CA(name=%v, type=%v)", c.DomainName, c.Type)
}

var certAuthorityMarshaler CertAuthorityMarshaler = &TeleportCertAuthorityMarshaler{}

// SetCertAuthorityMarshaler sets global user marshaler
func SetCertAuthorityMarshaler(u CertAuthorityMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	certAuthorityMarshaler = u
}

// GetCertAuthorityMarshaler returns currently set user marshaler
func GetCertAuthorityMarshaler() CertAuthorityMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return certAuthorityMarshaler
}

// CertAuthorityMarshaler implements marshal/unmarshal of User implementations
// mostly adds support for extended versions
type CertAuthorityMarshaler interface {
	// UnmarshalCertAuthority unmarhsals cert authority from binary representation
	UnmarshalCertAuthority(bytes []byte, opts ...MarshalOption) (CertAuthority, error)
	// MarshalCertAuthority to binary representation
	MarshalCertAuthority(c CertAuthority, opts ...MarshalOption) ([]byte, error)
	// GenerateCertAuthority is used to generate new cert authority
	// based on standard teleport one and is used to add custom
	// parameters and extend it in extensions of teleport
	GenerateCertAuthority(CertAuthority) (CertAuthority, error)
}

// GetCertAuthoritySchema returns JSON Schema for cert authorities
func GetCertAuthoritySchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(CertAuthoritySpecV2Schema, RotationSchema, RoleMapSchema), DefaultDefinitions)
}

type TeleportCertAuthorityMarshaler struct{}

// GenerateCertAuthority is used to generate new cert authority
// based on standard teleport one and is used to add custom
// parameters and extend it in extensions of teleport
func (*TeleportCertAuthorityMarshaler) GenerateCertAuthority(ca CertAuthority) (CertAuthority, error) {
	return ca, nil
}

// UnmarshalCertAuthority unmarshals cert authority from JSON
func (*TeleportCertAuthorityMarshaler) UnmarshalCertAuthority(bytes []byte, opts ...MarshalOption) (CertAuthority, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	err = utils.FastUnmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var ca CertAuthorityV1
		err := json.Unmarshal(bytes, &ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ca.V2(), nil
	case V2:
		var ca CertAuthorityV2
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(bytes, &ca); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetCertAuthoritySchema(), &ca, bytes); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}
		if err := ca.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			ca.SetResourceID(cfg.ID)
		}
		return &ca, nil
	}

	return nil, trace.BadParameter("cert authority resource version %v is not supported", h.Version)
}

// MarshalCertAuthority marshalls cert authority into JSON
func (*TeleportCertAuthorityMarshaler) MarshalCertAuthority(ca CertAuthority, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type cav1 interface {
		V1() *CertAuthorityV1
	}

	type cav2 interface {
		V2() *CertAuthorityV2
	}
	version := cfg.GetVersion()
	switch version {
	case V1:
		v, ok := ca.(cav1)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V1)
		}
		return utils.FastMarshal(v.V1())
	case V2:
		v, ok := ca.(cav2)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V2)
		}
		v2 := v.V2()
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *v2
			copy.SetResourceID(0)
			v2 = &copy
		}
		return utils.FastMarshal(v2)
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}
