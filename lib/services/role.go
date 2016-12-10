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

const RoleSpecSchema = `{
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
    }
  }
}`

var RoleSchema = fmt.Sprintf(`{
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
}`, MetadataSchema, RoleSpecSchema)

const NodeSelectorSchema = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "properties": {
    "match_labels": {
      "type": "object",
      "default": {},
      "additionalProperties": false,
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "string" }
      }
    },
    "namespaces": {
      "type": "object",
      "default": {},
      "additionalProperties": false,
      "patternProperties": {
         ".*":  { "type": "boolean" }
      }
    }
  }
}`

// UnmarshalRoleResource unmarshals role from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalRoleResource(data []byte) (*RoleResource, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("empty input")
	}
	var role RoleResource
	if err := utils.UnmarshalWithSchema(RoleSchema, &role, data); err != nil {
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
