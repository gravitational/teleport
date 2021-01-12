/*
Copyright 2020 Gravitational, Inc.

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

package types

import (
	"crypto"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

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
	// String returns human readable version of the CertAuthority
	String() string
	// TLSCA returns first TLS certificate authority from the list of key pairs
	TLSCA() (*tlsca.CertAuthority, error)
	// SetTLSKeyPairs sets TLS key pairs
	SetTLSKeyPairs(keyPairs []TLSKeyPair)
	// GetTLSKeyPairs returns first PEM encoded TLS cert
	GetTLSKeyPairs() []TLSKeyPair
	// JWTSigner returns the active JWT key used to sign tokens.
	JWTSigner(jwt.Config) (*jwt.Key, error)
	// GetJWTKeyPairs gets all JWT key pairs.
	GetJWTKeyPairs() []JWTKeyPair
	// SetJWTKeyPairs sets all JWT key pairs.
	SetJWTKeyPairs(keyPairs []JWTKeyPair)
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

// NewCertAuthority returns new cert authority
func NewCertAuthority(spec CertAuthoritySpecV2) CertAuthority {
	return &CertAuthorityV2{
		Kind:    KindCertAuthority,
		Version: V2,
		SubKind: string(spec.Type),
		Metadata: Metadata{
			Name:      spec.ClusterName,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// NewJWTAuthority creates and returns a services.CertAuthority with a new
// key pair.
func NewJWTAuthority(clusterName string) (CertAuthority, error) {
	publicKey, privateKey, err := jwt.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CertAuthorityV2{
		Kind:    KindCertAuthority,
		Version: V2,
		Metadata: Metadata{
			Name:      clusterName,
			Namespace: defaults.Namespace,
		},
		Spec: CertAuthoritySpecV2{
			ClusterName: clusterName,
			Type:        JWTSigner,
			JWTKeyPairs: []JWTKeyPair{
				{
					PublicKey:  publicKey,
					PrivateKey: privateKey,
				},
			},
		},
	}, nil
}

// GetVersion returns resource version
func (ca *CertAuthorityV2) GetVersion() string {
	return ca.Version
}

// GetKind returns resource kind
func (ca *CertAuthorityV2) GetKind() string {
	return ca.Kind
}

// GetSubKind returns resource sub kind
func (ca *CertAuthorityV2) GetSubKind() string {
	return ca.SubKind
}

// SetSubKind sets resource subkind
func (ca *CertAuthorityV2) SetSubKind(s string) {
	ca.SubKind = s
}

// Clone returns a copy of the cert authority object.
func (ca *CertAuthorityV2) Clone() CertAuthority {
	out := *ca
	out.Spec.CheckingKeys = utils.CopyByteSlices(ca.Spec.CheckingKeys)
	out.Spec.SigningKeys = utils.CopyByteSlices(ca.Spec.SigningKeys)
	for i, kp := range ca.Spec.TLSKeyPairs {
		out.Spec.TLSKeyPairs[i] = TLSKeyPair{
			Key:  utils.CopyByteSlice(kp.Key),
			Cert: utils.CopyByteSlice(kp.Cert),
		}
	}
	for i, kp := range ca.Spec.JWTKeyPairs {
		out.Spec.JWTKeyPairs[i] = JWTKeyPair{
			PublicKey:  utils.CopyByteSlice(kp.PublicKey),
			PrivateKey: utils.CopyByteSlice(kp.PrivateKey),
		}
	}
	out.Spec.Roles = utils.CopyStrings(ca.Spec.Roles)
	return &out
}

// GetRotation returns rotation state.
func (ca *CertAuthorityV2) GetRotation() Rotation {
	if ca.Spec.Rotation == nil {
		return Rotation{}
	}
	return *ca.Spec.Rotation
}

// SetRotation sets rotation state.
func (ca *CertAuthorityV2) SetRotation(r Rotation) {
	ca.Spec.Rotation = &r
}

// TLSCA returns TLS certificate authority
func (ca *CertAuthorityV2) TLSCA() (*tlsca.CertAuthority, error) {
	if len(ca.Spec.TLSKeyPairs) == 0 {
		return nil, trace.BadParameter("no TLS key pairs found for certificate authority")
	}
	return tlsca.New(ca.Spec.TLSKeyPairs[0].Cert, ca.Spec.TLSKeyPairs[0].Key)
}

// SetTLSKeyPairs sets TLS key pairs
func (ca *CertAuthorityV2) SetTLSKeyPairs(pairs []TLSKeyPair) {
	ca.Spec.TLSKeyPairs = pairs
}

// GetTLSKeyPairs returns TLS key pairs
func (ca *CertAuthorityV2) GetTLSKeyPairs() []TLSKeyPair {
	return ca.Spec.TLSKeyPairs
}

// JWTSigner returns the active JWT key used to sign tokens.
func (ca *CertAuthorityV2) JWTSigner(config jwt.Config) (*jwt.Key, error) {
	if len(ca.Spec.JWTKeyPairs) == 0 {
		return nil, trace.BadParameter("no JWT keypairs found")
	}
	privateKey, err := utils.ParsePrivateKey(ca.Spec.JWTKeyPairs[0].PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.Algorithm = defaults.ApplicationTokenAlgorithm
	config.ClusterName = ca.Spec.ClusterName
	config.PrivateKey = privateKey
	key, err := jwt.New(&config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return key, nil
}

// GetJWTKeyPairs gets all JWT keypairs used to sign a JWT.
func (ca *CertAuthorityV2) GetJWTKeyPairs() []JWTKeyPair {
	return ca.Spec.JWTKeyPairs
}

// SetJWTKeyPairs sets all JWT keypairs used to sign a JWT.
func (ca *CertAuthorityV2) SetJWTKeyPairs(keyPairs []JWTKeyPair) {
	ca.Spec.JWTKeyPairs = keyPairs
}

// GetMetadata returns object metadata
func (ca *CertAuthorityV2) GetMetadata() Metadata {
	return ca.Metadata
}

// SetExpiry sets expiry time for the object
func (ca *CertAuthorityV2) SetExpiry(expires time.Time) {
	ca.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (ca *CertAuthorityV2) Expiry() time.Time {
	return ca.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (ca *CertAuthorityV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	ca.Metadata.SetTTL(clock, ttl)
}

// GetResourceID returns resource ID
func (ca *CertAuthorityV2) GetResourceID() int64 {
	return ca.Metadata.ID
}

// SetResourceID sets resource ID
func (ca *CertAuthorityV2) SetResourceID(id int64) {
	ca.Metadata.ID = id
}

// WithoutSecrets returns an instance of resource without secrets.
func (ca *CertAuthorityV2) WithoutSecrets() Resource {
	ca2 := ca.Clone()
	RemoveCASecrets(ca2)
	return ca2
}

// RemoveCASecrets removes private (SSH, TLS, and JWT) keys from certificate
// authority.
func RemoveCASecrets(ca CertAuthority) {
	ca.SetSigningKeys(nil)

	tlsKeyPairs := ca.GetTLSKeyPairs()
	for i := range tlsKeyPairs {
		tlsKeyPairs[i].Key = nil
	}
	ca.SetTLSKeyPairs(tlsKeyPairs)

	jwtKeyPairs := ca.GetJWTKeyPairs()
	for i := range jwtKeyPairs {
		jwtKeyPairs[i].PrivateKey = nil
	}
	ca.SetJWTKeyPairs(jwtKeyPairs)
}

// String returns human readable version of the CertAuthorityV2.
func (ca *CertAuthorityV2) String() string {
	return fmt.Sprintf("CA(name=%v, type=%v)", ca.GetClusterName(), ca.GetType())
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

// GetSigningKeys returns signing keys
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
func (ca *CertAuthorityV2) SetRoleMap(m RoleMap) {
	ca.Spec.RoleMap = []RoleMapping(m)
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

// GetSigningAlg returns the CA's signing algorithm type
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

// ParseSigningAlg converts the SSH signature algorithm strings to the
// corresponding proto enum value.
//
// alg should be one of ssh.SigAlgo*  If it's not one of those
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

// SetSigningAlg sets the CA's signing algorith type
func (ca *CertAuthorityV2) SetSigningAlg(alg string) {
	ca.Spec.SigningAlg = ParseSigningAlg(alg)
}

// Check checks if all passed parameters are valid
func (ca *CertAuthorityV2) Check() error {
	err := ca.ID().Check()
	if err != nil {
		return trace.Wrap(err)
	}

	switch ca.GetType() {
	case UserCA, HostCA:
		err = ca.checkUserOrHostCA()
	case JWTSigner:
		err = ca.checkJWTKeys()
	default:
		err = trace.BadParameter("invalid CA type %q", ca.GetType())
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (ca *CertAuthorityV2) checkUserOrHostCA() error {
	if len(ca.Spec.CheckingKeys) == 0 {
		return trace.BadParameter("certificate authority missing SSH public keys")
	}
	if len(ca.Spec.TLSKeyPairs) == 0 {
		return trace.BadParameter("certificate authority missing TLS key pairs")
	}

	_, err := ca.Checkers()
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

func (ca *CertAuthorityV2) checkJWTKeys() error {
	// Check that some JWT keys have been set on the CA.
	if len(ca.Spec.JWTKeyPairs) == 0 {
		return trace.BadParameter("missing JWT CA")
	}

	var err error
	var privateKey crypto.Signer

	// Check that the JWT keys set are valid.
	for _, pair := range ca.Spec.JWTKeyPairs {
		if len(pair.PrivateKey) > 0 {
			privateKey, err = utils.ParsePrivateKey(pair.PrivateKey)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		publicKey, err := utils.ParsePublicKey(pair.PublicKey)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = jwt.New(&jwt.Config{
			Algorithm:   defaults.ApplicationTokenAlgorithm,
			ClusterName: ca.Spec.ClusterName,
			PrivateKey:  privateKey,
			PublicKey:   publicKey,
		})
		if err != nil {
			return trace.Wrap(err)
		}
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
	// when new certificate authority is issued, but not used
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
func (r *Rotation) Matches(rotation Rotation) bool {
	return r.CurrentID == rotation.CurrentID && r.State == rotation.State && r.Phase == rotation.Phase
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

// Merge overwrites r from src and
// is part of support for cloning Server values
// using proto.Clone.
//
// Note: this does not implement the full Merger interface,
// specifically, it assumes that r is zero value.
// See https://github.com/gogo/protobuf/blob/v1.3.1/proto/clone.go#L58-L60
//
// Implements proto.Merger
func (r *Rotation) Merge(src proto.Message) {
	s, ok := src.(*Rotation)
	if !ok {
		return
	}
	*r = *s
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

// CertAuthoritySpecV2Schema is JSON schema for cert authority V2
const CertAuthoritySpecV2Schema = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["type", "cluster_name"],
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
		"jwt_key_pairs":  {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"properties": {
					"public_key": {"type": "string"},
					"private_key": {"type": "string"}
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

// GetCertAuthoritySchema returns JSON Schema for cert authorities
func GetCertAuthoritySchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(CertAuthoritySpecV2Schema, RotationSchema, RoleMapSchema), DefaultDefinitions)
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

type teleportCertAuthorityMarshaler struct{}

// GenerateCertAuthority is used to generate new cert authority
// based on standard teleport one and is used to add custom
// parameters and extend it in extensions of teleport
func (*teleportCertAuthorityMarshaler) GenerateCertAuthority(ca CertAuthority) (CertAuthority, error) {
	return ca, nil
}

// UnmarshalCertAuthority unmarshals cert authority from JSON
func (*teleportCertAuthorityMarshaler) UnmarshalCertAuthority(bytes []byte, opts ...MarshalOption) (CertAuthority, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	err = utils.FastUnmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
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
func (*teleportCertAuthorityMarshaler) MarshalCertAuthority(ca CertAuthority, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch authority := ca.(type) {
	case *CertAuthorityV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *authority
			copy.SetResourceID(0)
			authority = &copy
		}
		return utils.FastMarshal(authority)
	default:
		return nil, trace.BadParameter("unrecognized certificate authority version %T", ca)
	}
}

var certAuthorityMarshaler CertAuthorityMarshaler = &teleportCertAuthorityMarshaler{}

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
