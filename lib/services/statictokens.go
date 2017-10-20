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
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// StaticTokens define a list of static []ProvisionToken used to provision a
// node. StaticTokens is a configuration resource, never create more than one instance
// of it.
type StaticTokens interface {
	// Resource provides common resource properties.
	Resource

	// SetStaticTokens sets the list of static tokens used to provision nodes.
	SetStaticTokens([]ProvisionToken)
	// GetStaticTokens gets the list of static tokens used to provision nodes.
	GetStaticTokens() []ProvisionToken

	// CheckAndSetDefaults checks and set default values for missing fields.
	CheckAndSetDefaults() error
}

// NewStaticTokens is a convenience wrapper to create a StaticTokens resource.
func NewStaticTokens(spec StaticTokensSpecV2) (StaticTokens, error) {
	st := StaticTokensV2{
		Kind:    KindStaticTokens,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameStaticTokens,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
	if err := st.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &st, nil
}

// DefaultStaticTokens is used to get the default static tokens (empty list)
// when nothing is specified in file configuration.
func DefaultStaticTokens() StaticTokens {
	return &StaticTokensV2{
		Kind:    KindStaticTokens,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameStaticTokens,
			Namespace: defaults.Namespace,
		},
		Spec: StaticTokensSpecV2{
			StaticTokens: []ProvisionToken{},
		},
	}
}

// StaticTokensV2 implements the StaticTokens interface.
type StaticTokensV2 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec StaticTokensSpecV2 `json:"spec"`
}

// StaticTokensSpecV2 is the actual data we care about for StaticTokensSpecV2.
type StaticTokensSpecV2 struct {
	// StaticTokens is a list of tokens that can be used to add nodes to the
	// cluster.
	StaticTokens []ProvisionToken `json:"static_tokens"`
}

// GetName returns the name of the StaticTokens resource.
func (c *StaticTokensV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the StaticTokens resource.
func (c *StaticTokensV2) SetName(e string) {
	c.Metadata.Name = e
}

// Expires returns object expiry setting
func (c *StaticTokensV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *StaticTokensV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using realtime clock
func (c *StaticTokensV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *StaticTokensV2) GetMetadata() Metadata {
	return c.Metadata
}

// SetStaticTokens sets the list of static tokens used to provision nodes.
func (c *StaticTokensV2) SetStaticTokens(s []ProvisionToken) {
	c.Spec.StaticTokens = s
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (c *StaticTokensV2) GetStaticTokens() []ProvisionToken {
	return c.Spec.StaticTokens
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *StaticTokensV2) CheckAndSetDefaults() error {
	// make sure we have defaults for all metadata fields
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// String represents a human readable version of static provisioning tokens.
func (c *StaticTokensV2) String() string {
	return fmt.Sprintf("StaticTokens(%v)", c.Spec.StaticTokens)
}

// StaticTokensSpecSchemaTemplate is a template for StaticTokens schema.
const StaticTokensSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
	"static_tokens": {
		"type": "array",
		"items": {
			"type": "object",
			"additionalProperties": false,
			"properties": {
				"expires": {
					"type": "string"
				},
				"roles": {
					"type": "array",
					"items": {
						"type": "string"
					}
				},
				"token": {
					"type": "string"
				}
			}
		}
	}%v
  }
}`

// GetStaticTokensSchema returns the schema with optionally injected
// schema for extensions.
func GetStaticTokensSchema(extensionSchema string) string {
	var staticTokensSchema string
	if staticTokensSchema == "" {
		staticTokensSchema = fmt.Sprintf(StaticTokensSpecSchemaTemplate, "")
	} else {
		staticTokensSchema = fmt.Sprintf(StaticTokensSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, staticTokensSchema, DefaultDefinitions)
}

// StaticTokensMarshaler implements marshal/unmarshal of StaticTokens implementations
// mostly adds support for extended versions.
type StaticTokensMarshaler interface {
	Marshal(c StaticTokens, opts ...MarshalOption) ([]byte, error)
	Unmarshal(bytes []byte) (StaticTokens, error)
}

var staticTokensMarshaler StaticTokensMarshaler = &TeleportStaticTokensMarshaler{}

// SetStaticTokensMarshaler sets the marshaler.
func SetStaticTokensMarshaler(m StaticTokensMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	staticTokensMarshaler = m
}

// GetStaticTokensMarshaler gets the marshaler.
func GetStaticTokensMarshaler() StaticTokensMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return staticTokensMarshaler
}

// TeleportStaticTokensMarshaler is used to marshal and unmarshal StaticTokens.
type TeleportStaticTokensMarshaler struct{}

// Unmarshal unmarshals StaticTokens from JSON.
func (t *TeleportStaticTokensMarshaler) Unmarshal(bytes []byte) (StaticTokens, error) {
	var staticTokens StaticTokensV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	err := utils.UnmarshalWithSchema(GetStaticTokensSchema(""), &staticTokens, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	err = staticTokens.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &staticTokens, nil
}

// Marshal marshals StaticTokens to JSON.
func (t *TeleportStaticTokensMarshaler) Marshal(c StaticTokens, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(c)
}
