/*
Copyright 2015 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

const (
	// DefaultAPIGroup is a default group of permissions API,
	// lets us to add different permission types
	DefaultAPIGroup = "gravitational.io/teleport"

	// ActionRead grants read access (get, list)
	ActionRead = "read"

	// ActionWrite allows to write (create, update, delete)
	ActionWrite = "write"

	// Wildcard is a special wildcard character matching everything
	Wildcard = "*"

	// DefaultNamespace is a default namespace of all resources
	DefaultNamespace = "default"

	// KindRole is a resource of kind role
	KindRole = "role"

	// V1 is our current version
	V1 = "v1"
)

// Access service manages roles and permissions
type Access interface {
	// GetRoles returns a list of roles
	GetRoles() ([]Role, error)

	// UpsertRole creates or updates role
	UpserRole(role Role) error

	// GetRole returns role by name
	GetRole(name string) (Role, error)

	// DeleteRole deletes role by name
	DeleteRole(name string) error
}

// Role contains a set of permissions or settings
type Role interface {
	// GetMetadata returns role metadata
	GetMetadata() Metadata
	// GetMaxSessionTTL is a maximum SSH or Web session TTL
	GetMaxSessionTTL() Duration
	// GetLogins returns a list of linux logins allowed for this role
	GetLogins() []string
	// GetNodeLabels returns a list of matchign nodes this role has access to
	GetNodeLabels() map[string]string
	// GetNamespaces returns a list of namespaces this role has access to
	GetNamespaces() []string
	// GetResources returns access to resources
	GetResources() map[string][]string
}

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string `json:"name"`
	// Namespace is object namespace
	Namespace string `json:"namespace"`
	// Description is object description
	Description string `json:"description"`
	// Labels is a set of labels
	Labels map[string]string `json:"labels,omitempty"`
}

// RoleResource represents role resource specification
type RoleResource struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains role specification
	Spec RoleSpec `json:"spec"`
}

// GetMetadata returns role metadata
func (r *RoleResource) GetMetadata() Metadata {
	return r.Metadata
}

// GetMaxSessionTTL is a maximum SSH or Web session TTL
func (r *RoleResource) GetMaxSessionTTL() Duration {
	return r.Spec.MaxSessionTTL
}

// GetLogins returns a list of linux logins allowed for this role
func (r *RoleResource) GetLogins() []string {
	return r.Spec.Logins
}

// GetNodeLabels returns a list of matchign nodes this role has access to
func (r *RoleResource) GetNodeLabels() map[string]string {
	return r.Spec.NodeLabels
}

// GetNamespaces returns a list of namespaces this role has access to
func (r *RoleResource) GetNamespaces() []string {
	return r.Spec.Namespaces
}

// GetResources returns access to resources
func (r *RoleResource) GetResources() map[string][]string {
	return r.Spec.Resources
}

// Check checks validity of all parameters and sets defaults
func (r *RoleResource) CheckAndSetDefaults() error {
	if r.Metadata.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if r.Spec.MaxSessionTTL.Duration == 0 {
		r.Spec.MaxSessionTTL.Duration = defaults.MaxCertDuration
	}
	if r.Spec.MaxSessionTTL.Duration < defaults.MinCertDuration {
		return trace.BadParameter("maximum session TTL can not be less than")
	}
	for key, val := range r.Spec.NodeLabels {
		if key == Wildcard && val != Wildcard {
			return trace.BadParameter("selector *:<val> is not supported")
		}
	}
	return nil
}

// RoleSpec is role specification
type RoleSpec struct {
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
	CheckAccessToServer(Server) error
	// CheckAccessToResourceAction check access to resource action
	CheckAccessToResourceAction(resourceNamespace, resourceName, accessType string) error
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

// CheckAccessToServer checks if role set has access to server based
// on combined role's selector
func (set RoleSet) CheckAccessToServer(s Server) error {
	for _, role := range set {
		if MatchNamespace(role.GetNamespaces(), s.GetNamespace()) && MatchLabels(role.GetNodeLabels(), s.Labels) {
			return nil
		}
	}
	return trace.AccessDenied("access to server is denied")
}

// CheckAccessToResourceAction checks if role set has access to this resource action
func (set RoleSet) CheckAccessToResourceAction(resourceNamespace, resourceName, accessType string) error {
	resourceNamespace = ProcessNamespace(resourceNamespace)
	for _, role := range set {
		if MatchNamespace(role.GetNamespaces(), resourceNamespace) && MatchResourceAction(role.GetResources(), resourceName, accessType) {
			return nil
		}
	}
	return trace.AccessDenied("%v access %v in namespace %v is denied")
}

// ProcessNamespace sets default namespace in case if namespace is empty
func ProcessNamespace(namespace string) string {
	if namespace == "" {
		return DefaultNamespace
	}
	return namespace
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

const MetadataSchema = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "required": ["name"],
  "properties": {
    "name": {"type": "string"},
    "namespace": {"type": "string", "default": "default"},
    "description": {"type": "string"},
    "labels": {
      "type": "object",
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "string" }
      }
    }
  }
}`

const RoleSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
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
    "resources": {
      "type": "object",
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "array", "items": {"type": "string"} }
       }
    },
    "extensions": %v
  }
}`

const RoleSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "required": ["kind", "spec", "metadata"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v1"},
    "metadata": %v,
    "spec": %v
  }
}`

// GetRoleSchema returns role schema with optionally injected
// schema for extensions
func GetRoleSchema(extensionSchema string) string {
	var roleSchema string
	if extensionSchema == "" {
		roleSchema = fmt.Sprintf(RoleSpecSchemaTemplate, `{"type": "object"}`)
	} else {
		roleSchema = fmt.Sprintf(RoleSpecSchemaTemplate, extensionSchema)
	}
	return fmt.Sprintf(RoleSchemaTemplate, MetadataSchema, roleSchema)
}

// UnmarshalRoleResource unmarshals role from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalRoleResource(data []byte) (*RoleResource, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("empty input")
	}
	var role RoleResource
	if err := utils.UnmarshalWithSchema(GetRoleSchema(""), &role, data); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return &role, nil
}

var roleMarshaler RoleMarshaler = &TeleportRoleMarshaler{}

func SetRoleMarshaler(u RoleMarshaler) {
	mtx.Lock()
	defer mtx.Unlock()
	roleMarshaler = u
}

func GetRoleMarshaler() RoleMarshaler {
	mtx.Lock()
	defer mtx.Unlock()
	return roleMarshaler
}

// RoleMarshaler implements marshal/unmarshal of Role implementations
// mostly adds support for extended versions
type RoleMarshaler interface {
	// UnmarshalRole from binary representation
	UnmarshalRole(bytes []byte) (Role, error)
	// MarshalRole to binary representation
	MarshalRole(u Role) ([]byte, error)
}

type TeleportRoleMarshaler struct{}

// UnmarshalRole unmarshals role from JSON
func (*TeleportRoleMarshaler) UnmarshalRole(bytes []byte) (Role, error) {
	return UnmarshalRoleResource(bytes)
}

// MarshalRole marshalls role into JSON
func (*TeleportRoleMarshaler) MarshalRole(u Role) ([]byte, error) {
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
	return s[i].GetMetadata().Name < s[j].GetMetadata().Name
}

// Swap swaps two roles in a list
func (s SortedRoles) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
