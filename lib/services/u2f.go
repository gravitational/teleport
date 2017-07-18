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

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// UniversalSecondFactorSettings defines the interface to get and set
// Universal Second Factor settings.
type UniversalSecondFactorSettings interface {
	// GetUniversalSecondFactor returns universal second factor settings.
	GetUniversalSecondFactor() (UniversalSecondFactor, error)

	// SetUniversalSecondFactor sets universal second factor settings.
	SetUniversalSecondFactor(UniversalSecondFactor) error
}

// UniversalSecondFactor defines settings for Universal Second Factor
// like the AppID and Facets.
type UniversalSecondFactor interface {
	// GetAppID returns the application ID for universal second factor.
	GetAppID() string

	// SetAppID sets the application ID for universal second factor.
	SetAppID(string)

	// GetFacets returns the facets for universal second factor.
	GetFacets() []string

	// SetFacets sets the facets for universal second factor.
	SetFacets([]string)

	// String represents a human readable version of U2F settings.
	String() string
}

// NewUniversalSecondFactor is a convenience method to to create UniversalSecondFactorV2.
func NewUniversalSecondFactor(spec UniversalSecondFactorSpecV2) (UniversalSecondFactor, error) {
	return &UniversalSecondFactorV2{
		Kind:    KindUniversalSecondFactor,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameUniversalSecondFactor,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}, nil
}

// UniversalSecondFactorV2 implements UniversalSecondFactor.
type UniversalSecondFactorV2 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec UniversalSecondFactorSpecV2 `json:"spec"`
}

// UniversalSecondFactorSpecV2 is the actual data we care about for UniversalSecondFactorV2.
type UniversalSecondFactorSpecV2 struct {
	// AppID is the application ID for universal second factor.
	AppID string `json:"app_id"`

	// Facets are the facets for universal second factor.
	Facets []string `json:"facets"`
}

// GetAppID returns the application ID for universal second factor.
func (c *UniversalSecondFactorV2) GetAppID() string {
	return c.Spec.AppID
}

// SetAppID sets the application ID for universal second factor.
func (c *UniversalSecondFactorV2) SetAppID(s string) {
	c.Spec.AppID = s
}

// GetFacets returns the facets for universal second factor.
func (c *UniversalSecondFactorV2) GetFacets() []string {
	return c.Spec.Facets
}

// SetFacets sets the facets for universal second factor.
func (c *UniversalSecondFactorV2) SetFacets(s []string) {
	c.Spec.Facets = s
}

// String represents a human readable version of U2F settings.
func (c *UniversalSecondFactorV2) String() string {
	return fmt.Sprintf("UniversalSecondFactor(AppID=%q,Facets=%q)", c.Spec.AppID, c.Spec.Facets)
}

const UniversalSecondFactorSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "app_id": {"type": "string"},
	"facets": {
      "type": "array",
      "items": {
        "type": "string"
      }
    }%v
  }
}`

// GetUniversalSecondFactorSchema returns the schema with optionally injected
// schema for extensions.
func GetUniversalSecondFactorSchema(extensionSchema string) string {
	var authPreferenceSchema string
	if authPreferenceSchema == "" {
		authPreferenceSchema = fmt.Sprintf(UniversalSecondFactorSpecSchemaTemplate, "")
	} else {
		authPreferenceSchema = fmt.Sprintf(UniversalSecondFactorSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, authPreferenceSchema, DefaultDefinitions)
}

// UniversalSecondFactorMarshaler implements marshal/unmarshal of UniversalSecondFactor implementations
// mostly adds support for extended versions.
type UniversalSecondFactorMarshaler interface {
	Marshal(c UniversalSecondFactor, opts ...MarshalOption) ([]byte, error)
	Unmarshal(bytes []byte) (UniversalSecondFactor, error)
}

var universalSecondFactorMarshaler UniversalSecondFactorMarshaler = &TeleportUniversalSecondFactorMarshaler{}

func SetUniversalSecondFactorMarshaler(m UniversalSecondFactorMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	universalSecondFactorMarshaler = m
}

func GetUniversalSecondFactorMarshaler() UniversalSecondFactorMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return universalSecondFactorMarshaler
}

type TeleportUniversalSecondFactorMarshaler struct{}

// Unmarshal unmarshals role from JSON or YAML.
func (t *TeleportUniversalSecondFactorMarshaler) Unmarshal(bytes []byte) (UniversalSecondFactor, error) {
	var authPreference UniversalSecondFactorV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	err := utils.UnmarshalWithSchema(GetUniversalSecondFactorSchema(""), &authPreference, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return &authPreference, nil
}

// Marshal marshals role to JSON or YAML.
func (t *TeleportUniversalSecondFactorMarshaler) Marshal(c UniversalSecondFactor, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(c)
}
