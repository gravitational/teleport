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
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
)

// CertAuthority is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthority interface {
	// ResourceWithSecrets sets common resource properties
	ResourceWithSecrets
	// SetMetadata sets CA metadata
	SetMetadata(meta Metadata)
	// GetID returns certificate authority ID -
	// combined type and name
	GetID() CertAuthID
	// GetType returns user or host certificate authority
	GetType() CertAuthType
	// GetClusterName returns cluster name this cert authority
	// is associated with
	GetClusterName() string

	GetActiveKeys() CAKeySet
	SetActiveKeys(CAKeySet) error
	GetAdditionalTrustedKeys() CAKeySet
	SetAdditionalTrustedKeys(CAKeySet) error

	GetTrustedSSHKeyPairs() []*SSHKeyPair
	GetTrustedTLSKeyPairs() []*TLSKeyPair
	GetTrustedJWTKeyPairs() []*JWTKeyPair

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
	// AddRole adds a role to ca role list
	AddRole(name string)
	// String returns human readable version of the CertAuthority
	String() string
	// GetRotation returns rotation state.
	GetRotation() Rotation
	// SetRotation sets rotation state.
	SetRotation(Rotation)
	// GetSigningAlg returns the signing algorithm used by signing keys.
	GetSigningAlg() CertAuthoritySpecV2_SigningAlgType
	// SetSigningAlg sets the signing algorithm used by signing keys.
	SetSigningAlg(CertAuthoritySpecV2_SigningAlgType)
	// AllKeyTypesMatch returns true if all keys in the CA are of the same type.
	AllKeyTypesMatch() bool
	// Clone returns a copy of the cert authority object.
	Clone() CertAuthority
}

// NewCertAuthority returns new cert authority
func NewCertAuthority(spec CertAuthoritySpecV2) (CertAuthority, error) {
	ca := &CertAuthorityV2{Spec: spec}
	if err := ca.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return ca, nil
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
	return proto.Clone(ca).(*CertAuthorityV2)
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

// SetMetadata sets object metadata
func (ca *CertAuthorityV2) SetMetadata(meta Metadata) {
	ca.Metadata = meta
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
	ca2 := ca.Clone().(*CertAuthorityV2)
	RemoveCASecrets(ca2)
	return ca2
}

// RemoveCASecrets removes private (SSH, TLS, and JWT) keys from certificate
// authority.
func RemoveCASecrets(ca CertAuthority) {
	cav2, ok := ca.(*CertAuthorityV2)
	if !ok {
		return
	}
	cav2.Spec.SigningKeys = nil

	for i := range cav2.Spec.TLSKeyPairs {
		cav2.Spec.TLSKeyPairs[i].Key = nil
	}

	for i := range cav2.Spec.JWTKeyPairs {
		cav2.Spec.JWTKeyPairs[i].PrivateKey = nil
	}

	cav2.Spec.ActiveKeys = cav2.Spec.ActiveKeys.WithoutSecrets()
	cav2.Spec.AdditionalTrustedKeys = cav2.Spec.AdditionalTrustedKeys.WithoutSecrets()
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

func (ca *CertAuthorityV2) getOldKeySet(index int) (keySet CAKeySet) {
	// in the "old" CA schema, index 0 contains the active keys and index 1 the
	// additional trusted keys
	if index < 0 || index > 1 {
		return
	}
	if len(ca.Spec.CheckingKeys) > index {
		kp := &SSHKeyPair{
			PrivateKeyType: PrivateKeyType_RAW,
			PublicKey:      utils.CopyByteSlice(ca.Spec.CheckingKeys[index]),
		}
		if len(ca.Spec.SigningKeys) > index {
			kp.PrivateKey = utils.CopyByteSlice(ca.Spec.SigningKeys[index])
		}
		keySet.SSH = []*SSHKeyPair{kp}
	}
	if len(ca.Spec.TLSKeyPairs) > index {
		keySet.TLS = []*TLSKeyPair{ca.Spec.TLSKeyPairs[index].Clone()}
	}
	if len(ca.Spec.JWTKeyPairs) > index {
		keySet.JWT = []*JWTKeyPair{ca.Spec.JWTKeyPairs[index].Clone()}
	}
	return keySet
}

func (ca *CertAuthorityV2) GetActiveKeys() CAKeySet {
	haveNewCAKeys := len(ca.Spec.ActiveKeys.SSH) > 0 || len(ca.Spec.ActiveKeys.TLS) > 0 || len(ca.Spec.ActiveKeys.JWT) > 0
	if haveNewCAKeys {
		return ca.Spec.ActiveKeys
	}
	// fall back to old schema
	return ca.getOldKeySet(0)
}

func (ca *CertAuthorityV2) SetActiveKeys(ks CAKeySet) error {
	if err := ks.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	ca.Spec.ActiveKeys = ks
	return nil
}

func (ca *CertAuthorityV2) GetAdditionalTrustedKeys() CAKeySet {
	haveNewCAKeys := len(ca.Spec.AdditionalTrustedKeys.SSH) > 0 || len(ca.Spec.AdditionalTrustedKeys.TLS) > 0 || len(ca.Spec.AdditionalTrustedKeys.JWT) > 0
	if haveNewCAKeys {
		return ca.Spec.AdditionalTrustedKeys
	}
	// fall back to old schema
	return ca.getOldKeySet(1)
}

func (ca *CertAuthorityV2) SetAdditionalTrustedKeys(ks CAKeySet) error {
	if err := ks.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	ca.Spec.AdditionalTrustedKeys = ks
	return nil
}

func (ca *CertAuthorityV2) GetTrustedSSHKeyPairs() []*SSHKeyPair {
	var kps []*SSHKeyPair
	for _, k := range ca.GetActiveKeys().SSH {
		kps = append(kps, k.Clone())
	}
	for _, k := range ca.GetAdditionalTrustedKeys().SSH {
		kps = append(kps, k.Clone())
	}
	return kps
}

func (ca *CertAuthorityV2) GetTrustedTLSKeyPairs() []*TLSKeyPair {
	var kps []*TLSKeyPair
	for _, k := range ca.GetActiveKeys().TLS {
		kps = append(kps, k.Clone())
	}
	for _, k := range ca.GetAdditionalTrustedKeys().TLS {
		kps = append(kps, k.Clone())
	}
	return kps
}

func (ca *CertAuthorityV2) GetTrustedJWTKeyPairs() []*JWTKeyPair {
	var kps []*JWTKeyPair
	for _, k := range ca.GetActiveKeys().JWT {
		kps = append(kps, k.Clone())
	}
	for _, k := range ca.GetAdditionalTrustedKeys().JWT {
		kps = append(kps, k.Clone())
	}
	return kps
}

// setStaticFields sets static resource header and metadata fields.
func (ca *CertAuthorityV2) setStaticFields() {
	ca.Kind = KindCertAuthority
	ca.Version = V2
	// ca.Metadata.Name and ca.Spec.ClusterName should always be equal.
	if ca.Metadata.Name == "" {
		ca.Metadata.Name = ca.Spec.ClusterName
	} else {
		ca.Spec.ClusterName = ca.Metadata.Name
	}
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (ca *CertAuthorityV2) CheckAndSetDefaults() error {
	ca.setStaticFields()
	if err := ca.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if ca.SubKind == "" {
		ca.SubKind = string(ca.Spec.Type)
	}

	if err := ca.ID().Check(); err != nil {
		return trace.Wrap(err)
	}

	if err := ca.Spec.ActiveKeys.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := ca.Spec.AdditionalTrustedKeys.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := ca.Spec.Rotation.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := ca.GetType().Check(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// AllKeyTypesMatch returns true if all private keys in the given CA are of the same type.
func (ca *CertAuthorityV2) AllKeyTypesMatch() bool {
	keyTypes := make(map[PrivateKeyType]struct{})
	for _, keySet := range []CAKeySet{ca.Spec.ActiveKeys, ca.Spec.AdditionalTrustedKeys} {
		for _, keyPair := range keySet.SSH {
			keyTypes[keyPair.PrivateKeyType] = struct{}{}
		}
		for _, keyPair := range keySet.TLS {
			keyTypes[keyPair.KeyType] = struct{}{}
		}
		for _, keyPair := range keySet.JWT {
			keyTypes[keyPair.PrivateKeyType] = struct{}{}
		}
	}
	return len(keyTypes) == 1
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
	if r == nil {
		return nil
	}
	switch r.Phase {
	case "", RotationPhaseInit, RotationPhaseStandby, RotationPhaseRollback, RotationPhaseUpdateClients, RotationPhaseUpdateServers:
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

// Clone returns a deep copy of TLSKeyPair that can be mutated without
// modifying the original.
func (k *TLSKeyPair) Clone() *TLSKeyPair {
	return &TLSKeyPair{
		KeyType: k.KeyType,
		Key:     utils.CopyByteSlice(k.Key),
		Cert:    utils.CopyByteSlice(k.Cert),
	}
}

// Clone returns a deep copy of JWTKeyPair that can be mutated without
// modifying the original.
func (k *JWTKeyPair) Clone() *JWTKeyPair {
	return &JWTKeyPair{
		PrivateKeyType: k.PrivateKeyType,
		PrivateKey:     utils.CopyByteSlice(k.PrivateKey),
		PublicKey:      utils.CopyByteSlice(k.PublicKey),
	}
}

// Clone returns a deep copy of SSHKeyPair that can be mutated without
// modifying the original.
func (k *SSHKeyPair) Clone() *SSHKeyPair {
	return &SSHKeyPair{
		PrivateKeyType: k.PrivateKeyType,
		PrivateKey:     utils.CopyByteSlice(k.PrivateKey),
		PublicKey:      utils.CopyByteSlice(k.PublicKey),
	}
}

// Clone returns a deep copy of CAKeySet that can be mutated without modifying
// the original.
func (ks CAKeySet) Clone() CAKeySet {
	var out CAKeySet
	if len(ks.TLS) > 0 {
		out.TLS = make([]*TLSKeyPair, 0, len(ks.TLS))
		for _, k := range ks.TLS {
			out.TLS = append(out.TLS, k.Clone())
		}
	}
	if len(ks.JWT) > 0 {
		out.JWT = make([]*JWTKeyPair, 0, len(ks.JWT))
		for _, k := range ks.JWT {
			out.JWT = append(out.JWT, k.Clone())
		}
	}
	if len(ks.SSH) > 0 {
		out.SSH = make([]*SSHKeyPair, 0, len(ks.SSH))
		for _, k := range ks.SSH {
			out.SSH = append(out.SSH, k.Clone())
		}
	}
	return out
}

// WithoutSecrets returns a deep copy of CAKeySet with all secret fields
// (private keys) removed.
func (ks CAKeySet) WithoutSecrets() CAKeySet {
	ks = ks.Clone()
	for _, k := range ks.SSH {
		k.PrivateKey = nil
	}
	for _, k := range ks.TLS {
		k.Key = nil
	}
	for _, k := range ks.JWT {
		k.PrivateKey = nil
	}
	return ks
}

// CheckAndSetDefaults validates CAKeySet and sets defaults on any empty fields
// as needed.
func (ks CAKeySet) CheckAndSetDefaults() error {
	for _, kp := range ks.SSH {
		if err := kp.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, kp := range ks.TLS {
		if err := kp.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, kp := range ks.JWT {
		if err := kp.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Empty returns true if the CAKeySet holds no keys
func (ks *CAKeySet) Empty() bool {
	return len(ks.SSH) == 0 && len(ks.TLS) == 0 && len(ks.JWT) == 0
}

// CheckAndSetDefaults validates SSHKeyPair and sets defaults on any empty
// fields as needed.
func (k *SSHKeyPair) CheckAndSetDefaults() error {
	if len(k.PublicKey) == 0 {
		return trace.BadParameter("SSH key pair missing public key")
	}
	return nil
}

// CheckAndSetDefaults validates TLSKeyPair and sets defaults on any empty
// fields as needed.
func (k *TLSKeyPair) CheckAndSetDefaults() error {
	if len(k.Cert) == 0 {
		return trace.BadParameter("TLS key pair missing certificate")
	}
	return nil
}

// CheckAndSetDefaults validates JWTKeyPair and sets defaults on any empty
// fields as needed.
func (k *JWTKeyPair) CheckAndSetDefaults() error {
	if len(k.PublicKey) == 0 {
		return trace.BadParameter("JWT key pair missing public key")
	}
	return nil
}
