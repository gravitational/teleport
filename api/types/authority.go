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
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
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
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
	// SetSigningKeys sets signing keys
	SetSigningKeys([][]byte) error
	// SetCheckingKeys sets signing keys
	SetCheckingKeys([][]byte) error
	// AddRole adds a role to ca role list
	AddRole(name string)
	// String returns human readable version of the CertAuthority
	String() string
	// SetTLSKeyPairs sets TLS key pairs
	SetTLSKeyPairs(keyPairs []TLSKeyPair)
	// GetTLSKeyPairs returns first PEM encoded TLS cert
	GetTLSKeyPairs() []TLSKeyPair
	// GetJWTKeyPairs gets all JWT key pairs.
	GetJWTKeyPairs() []JWTKeyPair
	// SetJWTKeyPairs sets all JWT key pairs.
	SetJWTKeyPairs(keyPairs []JWTKeyPair)
	// GetRotation returns rotation state.
	GetRotation() Rotation
	// SetRotation sets rotation state.
	SetRotation(Rotation)
	// GetSigningAlg returns the signing algorithm used by signing keys.
	GetSigningAlg() CertAuthoritySpecV2_SigningAlgType
	// SetSigningAlg sets the signing algorithm used by signing keys.
	SetSigningAlg(CertAuthoritySpecV2_SigningAlgType)
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

// SetTLSKeyPairs sets TLS key pairs
func (ca *CertAuthorityV2) SetTLSKeyPairs(pairs []TLSKeyPair) {
	ca.Spec.TLSKeyPairs = pairs
}

// GetTLSKeyPairs returns TLS key pairs
func (ca *CertAuthorityV2) GetTLSKeyPairs() []TLSKeyPair {
	return ca.Spec.TLSKeyPairs
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

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (ca *CertAuthorityV2) SetTTL(clock Clock, ttl time.Duration) {
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

// GetSigningAlg returns the CA's signing algorithm type
func (ca *CertAuthorityV2) GetSigningAlg() CertAuthoritySpecV2_SigningAlgType {
	return ca.Spec.SigningAlg
}

// SetSigningAlg sets the CA's signing algorith type
func (ca *CertAuthorityV2) SetSigningAlg(alg CertAuthoritySpecV2_SigningAlgType) {
	ca.Spec.SigningAlg = alg
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (ca *CertAuthorityV2) CheckAndSetDefaults() error {
	err := ca.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	if err = ca.ID().Check(); err != nil {
		return trace.Wrap(err)
	}

	switch ca.GetType() {
	case UserCA, HostCA, JWTSigner:
		return nil
	default:
		return trace.BadParameter("invalid CA type %q", ca.GetType())
	}
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
	return fmt.Sprintf("last rotated %v", r.LastRotated.Format(constants.HumanDateFormatSeconds))
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
		return fmt.Sprintf("rotated %v", r.LastRotated.Format(constants.HumanDateFormatSeconds))
	case RotationStateInProgress:
		return fmt.Sprintf("%v (mode: %v, started: %v, ending: %v)",
			r.PhaseDescription(),
			r.Mode,
			r.Started.Format(constants.HumanDateFormatSeconds),
			r.Started.Add(r.GracePeriod.Duration()).Format(constants.HumanDateFormatSeconds),
		)
	default:
		return "unknown"
	}
}

// CheckAndSetDefaults checks and sets default rotation parameters.
func (r *Rotation) CheckAndSetDefaults() error {
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
func GenerateSchedule(now time.Time, gracePeriod time.Duration) (*RotationSchedule, error) {
	if gracePeriod <= 0 {
		return nil, trace.BadParameter("invalid grace period %q, provide value > 0", gracePeriod)
	}
	return &RotationSchedule{
		UpdateClients: now.UTC().Add(gracePeriod / 3),
		UpdateServers: now.UTC().Add((gracePeriod * 2) / 3),
		Standby:       now.UTC().Add(gracePeriod),
	}, nil
}

// CheckAndSetDefaults checks and sets default values of the rotation schedule.
func (s *RotationSchedule) CheckAndSetDefaults(now time.Time) error {
	if s.UpdateServers.IsZero() {
		return trace.BadParameter("phase %q has no time switch scheduled", RotationPhaseUpdateServers)
	}
	if s.Standby.IsZero() {
		return trace.BadParameter("phase %q has no time switch scheduled", RotationPhaseStandby)
	}
	if s.Standby.Before(s.UpdateServers) {
		return trace.BadParameter("phase %q can not be scheduled before %q", RotationPhaseStandby, RotationPhaseUpdateServers)
	}
	if s.UpdateServers.Before(now) {
		return trace.BadParameter("phase %q can not be scheduled in the past", RotationPhaseUpdateServers)
	}
	if s.Standby.Before(now) {
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
