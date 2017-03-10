/*
Copyright 2016 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
)

// RoleNameForUser returns role name associated with user
func RoleNameForUser(name string) string {
	return "user:" + name
}

// RoleNameForCertAuthority returns role name associated with cert authority
func RoleNameForCertAuthority(name string) string {
	return "ca:" + name
}

// RoleForUser creates role using AllowedLogins parameter
func RoleForUser(u User) Role {
	return &RoleV2{
		Kind:    KindRole,
		Version: V2,
		Metadata: Metadata{
			Name:      RoleNameForUser(u.GetName()),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV2{
			MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			NodeLabels:    map[string]string{Wildcard: Wildcard},
			Namespaces:    []string{defaults.Namespace},
			Resources: map[string][]string{
				KindSession:       RO(),
				KindRole:          RO(),
				KindNode:          RO(),
				KindAuthServer:    RO(),
				KindReverseTunnel: RO(),
				KindCertAuthority: RO(),
			},
		},
	}
}

// RoleForCertauthority creates role using AllowedLogins parameter
func RoleForCertAuthority(ca CertAuthority) Role {
	return &RoleV2{
		Kind:    KindRole,
		Version: V2,
		Metadata: Metadata{
			Name:      RoleNameForCertAuthority(ca.GetClusterName()),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV2{
			MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			NodeLabels:    map[string]string{Wildcard: Wildcard},
			Namespaces:    []string{defaults.Namespace},
			Resources: map[string][]string{
				KindRole:          RO(),
				KindSession:       RO(),
				KindNode:          RO(),
				KindAuthServer:    RO(),
				KindReverseTunnel: RO(),
				KindCertAuthority: RO(),
			},
		},
	}
}

// ConvertV1CertAuthority converts V1 cert authority for new CA and Role
func ConvertV1CertAuthority(v1 *CertAuthorityV1) (CertAuthority, Role) {
	ca := v1.V2()
	role := RoleForCertAuthority(ca)
	role.SetLogins(v1.AllowedLogins)
	ca.AddRole(role.GetName())
	return ca, role
}

// Access service manages roles and permissions
type Access interface {
	// GetRoles returns a list of roles
	GetRoles() ([]Role, error)

	// UpsertRole creates or updates role
	UpsertRole(role Role) error

	// GetRole returns role by name
	GetRole(name string) (Role, error)

	// DeleteRole deletes role by name
	DeleteRole(name string) error
}

// Role contains a set of permissions or settings
type Role interface {
	// GetMetadata returns role metadata
	GetMetadata() Metadata
	// GetName returns role name and is a shortcut for GetMetadata().Name
	GetName() string
	// GetMaxSessionTTL is a maximum SSH or Web session TTL
	GetMaxSessionTTL() Duration
	// SetLogins sets logins for role
	SetLogins(logins []string)
	// GetLogins returns a list of linux logins allowed for this role
	GetLogins() []string
	// GetNodeLabels returns a list of matching nodes this role has access to
	GetNodeLabels() map[string]string
	// GetNamespaces returns a list of namespaces this role has access to
	GetNamespaces() []string
	// GetResources returns access to resources
	GetResources() map[string][]string
	// SetResource sets resource rule
	SetResource(kind string, actions []string)
	// SetNodeLabels sets node labels for this rule
	SetNodeLabels(labels map[string]string)
	// SetMaxSessionTTL sets a maximum TTL for SSH or Web session
	SetMaxSessionTTL(duration time.Duration)
	// SetNamespaces sets a list of namespaces this role has access to
	SetNamespaces(namespaces []string)
}

// RoleV2 represents role resource specification
type RoleV2 struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains role specification
	Spec RoleSpecV2 `json:"spec"`
}

// SetResource sets resource rule
func (r *RoleV2) SetResource(kind string, actions []string) {
	if r.Spec.Resources == nil {
		r.Spec.Resources = make(map[string][]string)
	}
	r.Spec.Resources[kind] = actions
}

// SetLogins sets logins for role
func (r *RoleV2) SetLogins(logins []string) {
	r.Spec.Logins = logins
}

// SetNodeLabels sets node labels for role
func (r *RoleV2) SetNodeLabels(labels map[string]string) {
	r.Spec.NodeLabels = labels
}

// SetMaxSessionTTL sets a maximum TTL for SSH or Web session
func (r *RoleV2) SetMaxSessionTTL(duration time.Duration) {
	r.Spec.MaxSessionTTL.Duration = duration
}

// GetName returns role name and is a shortcut for GetMetadata().Name
func (r *RoleV2) GetName() string {
	return r.Metadata.Name
}

// GetMetadata returns role metadata
func (r *RoleV2) GetMetadata() Metadata {
	return r.Metadata
}

// GetMaxSessionTTL is a maximum SSH or Web session TTL
func (r *RoleV2) GetMaxSessionTTL() Duration {
	return r.Spec.MaxSessionTTL
}

// GetLogins returns a list of linux logins allowed for this role
func (r *RoleV2) GetLogins() []string {
	return r.Spec.Logins
}

// GetNodeLabels returns a list of matchign nodes this role has access to
func (r *RoleV2) GetNodeLabels() map[string]string {
	return r.Spec.NodeLabels
}

// GetNamespaces returns a list of namespaces this role has access to
func (r *RoleV2) GetNamespaces() []string {
	return r.Spec.Namespaces
}

// SetNamespaces sets a list of namespaces this role has access to
func (r *RoleV2) SetNamespaces(namespaces []string) {
	r.Spec.Namespaces = namespaces
}

// GetResources returns access to resources
func (r *RoleV2) GetResources() map[string][]string {
	return r.Spec.Resources
}

// Check checks validity of all parameters and sets defaults
func (r *RoleV2) CheckAndSetDefaults() error {
	if r.Metadata.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if r.Spec.MaxSessionTTL.Duration == 0 {
		r.Spec.MaxSessionTTL.Duration = defaults.MaxCertDuration
	}
	if r.Spec.MaxSessionTTL.Duration < defaults.MinCertDuration {
		return trace.BadParameter("maximum session TTL can not be less than")
	}
	for _, login := range r.Spec.Logins {
		if login == Wildcard {
			return trace.BadParameter("wilcard matcher is not allowed in logins")
		}
		if !cstrings.IsValidUnixUser(login) {
			return trace.BadParameter("'%v' is not a valid user name", login)
		}
	}
	for key, val := range r.Spec.NodeLabels {
		if key == Wildcard && val != Wildcard {
			return trace.BadParameter("selector *:<val> is not supported")
		}
	}
	return nil
}

func (r *RoleV2) String() string {
	return fmt.Sprintf("Role(Name=%v,MaxSessionTTL=%v,Logins=%v,NodeLabels=%v,Namespaces=%v,Resources=%v)",
		r.GetName(), r.GetMaxSessionTTL(), r.GetLogins(), r.GetNodeLabels(), r.GetNamespaces(), r.GetResources())
}

// RoleSpecV2 is role specification for RoleV2
type RoleSpecV2 struct {
	// MaxSessionTTL is a maximum SSH or Web session TTL
	MaxSessionTTL Duration `json:"max_session_ttl"`
	// Logins is a list of linux logins allowed for this role
	Logins []string `json:"logins,omitempty"`
	// NodeLabels is a set of matching labels that users of this role
	// will be allowed to access
	NodeLabels map[string]string `json:"node_labels,omitempty"`
	// Namespaces is a list of namespaces, guarding accesss to resources
	Namespaces []string `json:"namespaces,omitempty"`
	// Resources limits access to resources
	Resources map[string][]string `json:"resources,omitempty"`
}

// AccessChecker interface implements access checks for given role
type AccessChecker interface {
	// CheckAccessToServer checks access to server
	CheckAccessToServer(login string, server Server) error
	// CheckResourceAction check access to resource action
	CheckResourceAction(resourceNamespace, resourceName, accessType string) error
	// CheckLogins checks if role set can login up to given duration
	// and returns a combined list of allowed logins
	CheckLogins(ttl time.Duration) ([]string, error)
	// AdjustSessionTTL will reduce the requested ttl to lowes max allowed TTL
	// for this role set, otherwise it returns ttl unchanges
	AdjustSessionTTL(ttl time.Duration) time.Duration
}

// FromSpec returns new RoleSet created from spec
func FromSpec(name string, spec RoleSpecV2) (RoleSet, error) {
	role, err := NewRole(name, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewRoleSet(role), nil
}

// RW returns read write action list
func RW() []string {
	return []string{ActionRead, ActionWrite}
}

// RO returns read only action list
func RO() []string {
	return []string{ActionRead}
}

// NewRole constructs new standard role
func NewRole(name string, spec RoleSpecV2) (Role, error) {
	role := RoleV2{
		Kind:    KindRole,
		Version: V2,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
	if err := role.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &role, nil
}

// RoleGetter is an interface that defines GetRole method
type RoleGetter interface {
	// GetRole returns role by name
	GetRole(name string) (Role, error)
}

// FetchRoles fetches roles by their names and returns role set
func FetchRoles(roleNames []string, access RoleGetter) (RoleSet, error) {
	var roles RoleSet
	for _, roleName := range roleNames {
		role, err := access.GetRole(roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// NewRoleSet returns new RoleSet based on the roles
func NewRoleSet(roles ...Role) RoleSet {
	return roles
}

// RoleSet is a set of roles that implements access control functionality
type RoleSet []Role

// MatchResourceAction tests if selector matches required resource action in a given namespace
func MatchResourceAction(selector map[string][]string, resourceName, resourceAction string) bool {
	// empty selector matches nothing
	if len(selector) == 0 {
		return false
	}

	// check for wildcard resource matcher
	for _, action := range selector[Wildcard] {
		if action == Wildcard || action == resourceAction {
			return true
		}
	}

	// check for matching resource by name
	for _, action := range selector[resourceName] {
		if action == Wildcard || action == resourceAction {
			return true
		}
	}
	return false
}

// MatchLogin returns true if attempted login matches any of the logins
func MatchLogin(logins []string, login string) bool {
	for _, l := range logins {
		if l == login {
			return true
		}
	}
	return false
}

// MatchNamespace returns true if given list of namespace matches
// target namespace, wildcard matches everything
func MatchNamespace(selector []string, namespace string) bool {
	for _, n := range selector {
		if n == namespace || n == Wildcard {
			return true
		}
	}
	return false
}

// MatchLabels matches selector against target
func MatchLabels(selector map[string]string, target map[string]string) bool {
	// empty selector matches nothing
	if len(selector) == 0 {
		return false
	}
	// *: * matches everything even empty target set
	if selector[Wildcard] == Wildcard {
		return true
	}
	for key, val := range selector {
		if targetVal, ok := target[key]; !ok || (val != targetVal && val != Wildcard) {
			return false
		}
	}
	return true
}

// AdjustSessionTTL will reduce the requested ttl to lowest max allowed TTL
// for this role set, otherwise it returns ttl unchanges
func (set RoleSet) AdjustSessionTTL(ttl time.Duration) time.Duration {
	for _, role := range set {
		if ttl > role.GetMaxSessionTTL().Duration {
			ttl = role.GetMaxSessionTTL().Duration
		}
	}
	return ttl
}

// CheckLogins checks if role set can login up to given duration
// and returns a combined list of allowed logins
func (set RoleSet) CheckLogins(ttl time.Duration) ([]string, error) {
	logins := make(map[string]bool)
	var matchedTTL bool
	for _, role := range set {
		if ttl <= role.GetMaxSessionTTL().Duration && role.GetMaxSessionTTL().Duration != 0 {
			matchedTTL = true
		}
		for _, login := range role.GetLogins() {
			logins[login] = true
		}
	}
	if !matchedTTL {
		return nil, trace.AccessDenied("this user cannot request a certificate for %v", ttl)
	}
	if len(logins) == 0 {
		return nil, trace.AccessDenied("this user cannot create SSH sessions, has no logins")
	}
	out := make([]string, 0, len(logins))
	for login := range logins {
		out = append(out, login)
	}
	return out, nil
}

// CheckAccessToServer checks if role set has access to server based
// on combined role's selector and attempted login
func (set RoleSet) CheckAccessToServer(login string, s Server) error {
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(), s.GetNamespace())
		matchLabels := MatchLabels(role.GetNodeLabels(), s.GetAllLabels())
		matchLogin := MatchLogin(role.GetLogins(), login)
		if matchNamespace && matchLabels && matchLogin {
			return nil
		}
	}
	return trace.AccessDenied("access to server is denied for %v", set)
}

func (set RoleSet) String() string {
	if len(set) == 0 {
		return "user without assigned roles"
	}
	roleNames := make([]string, len(set))
	for i, role := range set {
		roleNames[i] = role.GetName()
	}
	return fmt.Sprintf("roles %v", strings.Join(roleNames, ","))
}

// CheckResourceAction checks if role set has access to this resource action
func (set RoleSet) CheckResourceAction(resourceNamespace, resourceName, accessType string) error {
	resourceNamespace = ProcessNamespace(resourceNamespace)
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(), resourceNamespace)
		matchResourceAction := MatchResourceAction(role.GetResources(), resourceName, accessType)
		if matchNamespace && matchResourceAction {
			return nil
		}
	}
	return trace.AccessDenied("%v access to %v in namespace %v is denied for %v", accessType, resourceName, resourceNamespace, set)
}

// ProcessNamespace sets default namespace in case if namespace is empty
func ProcessNamespace(namespace string) string {
	if namespace == "" {
		return defaults.Namespace
	}
	return namespace
}

// MaxDuration returns maximum duration that is possible
func MaxDuration() Duration {
	return NewDuration(1<<63 - 1)
}

// NewDuration returns Duration struct based on time.Duration
func NewDuration(d time.Duration) Duration {
	return Duration{Duration: d}
}

// Duration is a wrapper around duration to set up custom marshal/unmarshal
type Duration struct {
	time.Duration
}

// MarshalJSON marshals Duration to string
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%v", d.Duration))
}

// UnmarshalJSON marshals Duration to string
func (d *Duration) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var stringVar string
	if err := json.Unmarshal(data, &stringVar); err != nil {
		return trace.Wrap(err)
	}
	out, err := time.ParseDuration(stringVar)
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	d.Duration = out
	return nil
}

const RoleSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "max_session_ttl": {"type": "string"},
    "node_labels": {
      "type": "object",
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "string" }
      }
    },
    "namespaces": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "logins": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "resources": {
      "type": "object",
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "array", "items": {"type": "string"} }
       }
    }%v
  }
}`

// GetRoleSchema returns role schema with optionally injected
// schema for extensions
func GetRoleSchema(extensionSchema string) string {
	var roleSchema string
	if extensionSchema == "" {
		roleSchema = fmt.Sprintf(RoleSpecSchemaTemplate, ``)
	} else {
		roleSchema = fmt.Sprintf(RoleSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, roleSchema)
}

// UnmarshalRole unmarshals role from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalRole(data []byte) (*RoleV2, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}
	var role RoleV2
	if err := utils.UnmarshalWithSchema(GetRoleSchema(""), &role, data); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return &role, nil
}

var roleMarshaler RoleMarshaler = &TeleportRoleMarshaler{}

func SetRoleMarshaler(m RoleMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	roleMarshaler = m
}

func GetRoleMarshaler() RoleMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return roleMarshaler
}

// RoleMarshaler implements marshal/unmarshal of Role implementations
// mostly adds support for extended versions
type RoleMarshaler interface {
	// UnmarshalRole from binary representation
	UnmarshalRole(bytes []byte) (Role, error)
	// MarshalRole to binary representation
	MarshalRole(u Role, opts ...MarshalOption) ([]byte, error)
}

type TeleportRoleMarshaler struct{}

// UnmarshalRole unmarshals role from JSON
func (*TeleportRoleMarshaler) UnmarshalRole(bytes []byte) (Role, error) {
	return UnmarshalRole(bytes)
}

// MarshalRole marshalls role into JSON
func (*TeleportRoleMarshaler) MarshalRole(u Role, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(u)
}

// SortedRoles sorts roles by name
type SortedRoles []Role

// Len returns length of a role list
func (s SortedRoles) Len() int {
	return len(s)
}

// Less compares roles by name
func (s SortedRoles) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

// Swap swaps two roles in a list
func (s SortedRoles) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
