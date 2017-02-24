/*
Copyright 2017 Gravitational, Inc.

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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// ClusterAuthPreference defines an interface to get and set
// authentication preferences for a cluster.
type ClusterAuthPreference interface {
	// GetClusterAuthPreference returns the authentication preferences for a cluster.
	GetClusterAuthPreference() (AuthPreference, error)

	// SetClusterAuthPreference sets the authentication preferences for a cluster.
	SetClusterAuthPreference(AuthPreference) error
}

// AuthPreference defines the authentication preferences for a specific
// cluster. It defines the type (local, oidc) and second factor (off, otp, oidc).
type AuthPreference interface {
	// GetType returns the type of authentication.
	GetType() string

	// SetType sets the type of authentication.
	SetType(string)

	// GetSecondFactor returns the type of second factor.
	GetSecondFactor() string

	// SetSecondFactor sets the type of second factor.
	SetSecondFactor(string)

	// CheckAndSetDefaults sets and default values and then
	// verifies the constraints for AuthPreference.
	CheckAndSetDefaults() error

	// String represents a human readable version of authentication settings.
	String() string
}

// NewAuthPreference is a convenience method to to create AuthPreferenceV2.
func NewAuthPreference(spec AuthPreferenceSpecV2) (AuthPreference, error) {
	return &AuthPreferenceV2{
		Kind:    KindClusterAuthPreference,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameClusterAuthPreference,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}, nil
}

// AuthPreferenceV2 implements AuthPreference.
type AuthPreferenceV2 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec AuthPreferenceSpecV2 `json:"spec"`
}

// AuthPreferenceSpecV2 is the actual data we care about for AuthPreferenceV2.
type AuthPreferenceSpecV2 struct {
	// Type is the type of authentication.
	Type string `json:"type"`

	// SecondFactor is the type of second factor.
	SecondFactor string `json:"second_factor"`
}

// GetType returns the type of authentication.
func (c *AuthPreferenceV2) GetType() string {
	return c.Spec.Type
}

// SetType sets the type of authentication.
func (c *AuthPreferenceV2) SetType(s string) {
	c.Spec.Type = s
}

// GetSecondFactor returns the type of second factor.
func (c *AuthPreferenceV2) GetSecondFactor() string {
	return c.Spec.SecondFactor
}

// SetSecondFactor sets the type of second factor.
func (c *AuthPreferenceV2) SetSecondFactor(s string) {
	c.Spec.SecondFactor = s
}

// CheckAndSetDefaults verifies the constraints for AuthPreference.
func (c *AuthPreferenceV2) CheckAndSetDefaults() error {
	// if nothing is passed in, set defaults
	if c.Spec.Type == "" {
		c.Spec.Type = teleport.Local
	}
	if c.Spec.SecondFactor == "" {
		c.Spec.SecondFactor = teleport.OTP
	}

	// make sure whatever was passed in was sane
	switch c.Spec.Type {
	case teleport.Local:
		if c.Spec.SecondFactor != teleport.OFF && c.Spec.SecondFactor != teleport.OTP && c.Spec.SecondFactor != teleport.U2F {
			return trace.BadParameter("second factor type %q not supported", c.Spec.SecondFactor)
		}
	case teleport.OIDC:
		if c.Spec.SecondFactor != "" {
			return trace.BadParameter("second factor not supported with oidc connector")
		}
	default:
		return trace.BadParameter("unsupported type %q", c.Spec.Type)
	}

	return nil
}

// String represents a human readable version of authentication settings.
func (c *AuthPreferenceV2) String() string {
	return fmt.Sprintf("AuthPreference(Type=%q,SecondFactor=%q)", c.Spec.Type, c.Spec.SecondFactor)
}

const AuthPreferenceSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "type": {"type": "string"},
    "second_factor": {"type": "string"}%v
  }
}`

// GetAuthPreferenceSchema returns the schema with optionally injected
// schema for extensions.
func GetAuthPreferenceSchema(extensionSchema string) string {
	var authPreferenceSchema string
	if authPreferenceSchema == "" {
		authPreferenceSchema = fmt.Sprintf(AuthPreferenceSpecSchemaTemplate, "")
	} else {
		authPreferenceSchema = fmt.Sprintf(AuthPreferenceSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, authPreferenceSchema)
}

// AuthPreferenceMarshaler implements marshal/unmarshal of AuthPreference implementations
// mostly adds support for extended versions.
type AuthPreferenceMarshaler interface {
	Marshal(c AuthPreference, opts ...MarshalOption) ([]byte, error)
	Unmarshal(bytes []byte) (AuthPreference, error)
}

var authPreferenceMarshaler AuthPreferenceMarshaler = &TeleportAuthPreferenceMarshaler{}

func SetAuthPreferenceMarshaler(m AuthPreferenceMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	authPreferenceMarshaler = m
}

func GetAuthPreferenceMarshaler() AuthPreferenceMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return authPreferenceMarshaler
}

type TeleportAuthPreferenceMarshaler struct{}

// Unmarshal unmarshals role from JSON or YAML.
func (t *TeleportAuthPreferenceMarshaler) Unmarshal(bytes []byte) (AuthPreference, error) {
	var authPreference AuthPreferenceV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	err := utils.UnmarshalWithSchema(GetAuthPreferenceSchema(""), &authPreference, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return &authPreference, nil
}

// Marshal marshals role to JSON or YAML.
func (t *TeleportAuthPreferenceMarshaler) Marshal(c AuthPreference, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(c)
}
