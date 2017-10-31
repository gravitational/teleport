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
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// ClusterConfig defines cluster level configuration. This is a configuration
// resource, never create more than one instance of it.
type ClusterConfig interface {
	// Resource provides common resource properties.
	Resource

	// GetSessionRecording gets where the session is being recorded.
	GetSessionRecording() RecordingType

	// SetSessionRecording sets where the session is recorded.
	SetSessionRecording(RecordingType)

	// CheckAndSetDefaults checks and set default values for missing fields.
	CheckAndSetDefaults() error
}

// NewClusterConfig is a convenience wrapper to create a ClusterConfig resource.
func NewClusterConfig(spec ClusterConfigSpecV3) (ClusterConfig, error) {
	cc := ClusterConfigV3{
		Kind:    KindClusterConfig,
		Version: V3,
		Metadata: Metadata{
			Name:      MetaNameClusterConfig,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
	if err := cc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cc, nil
}

// DefaultClusterConfig is used as the default cluster configuration when
// one is not specified (record at node).
func DefaultClusterConfig() ClusterConfig {
	return &ClusterConfigV3{
		Kind:    KindClusterConfig,
		Version: V3,
		Metadata: Metadata{
			Name:      MetaNameClusterConfig,
			Namespace: defaults.Namespace,
		},
		Spec: ClusterConfigSpecV3{
			SessionRecording: RecordAtNode,
		},
	}
}

// ClusterConfigV3 implements the ClusterConfig interface.
type ClusterConfigV3 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Spec is the specification of the resource.
	Spec ClusterConfigSpecV3 `json:"spec"`
}

// RecordingType holds where the session will be recorded.
type RecordingType string

const (
	// RecordAtNode is the default. Sessions are recorded at Teleport nodes.
	RecordAtNode RecordingType = "node"

	// RecordAtProxy enabled the recording proxy which intercepts and records all sessions.
	RecordAtProxy RecordingType = "proxy"

	// RecordOff is used to disable session recording completely.
	RecordOff RecordingType = "off"
)

// ClusterConfigSpecV3 is the actual data we care about for ClusterConfig.
type ClusterConfigSpecV3 struct {
	// SessionRecording controls where (or if) the session is recorded.
	SessionRecording RecordingType `json:"session_recording"`
}

// GetName returns the name of the cluster.
func (c *ClusterConfigV3) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster.
func (c *ClusterConfigV3) SetName(e string) {
	c.Metadata.Name = e
}

// Expires retuns object expiry setting
func (c *ClusterConfigV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *ClusterConfigV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using realtime clock
func (c *ClusterConfigV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *ClusterConfigV3) GetMetadata() Metadata {
	return c.Metadata
}

// GetClusterConfig gets the name of the cluster.
func (c *ClusterConfigV3) GetSessionRecording() RecordingType {
	return c.Spec.SessionRecording
}

// SetClusterConfig sets the name of the cluster.
func (c *ClusterConfigV3) SetSessionRecording(s RecordingType) {
	c.Spec.SessionRecording = s
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *ClusterConfigV3) CheckAndSetDefaults() error {
	// make sure we have defaults for all metadata fields
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	if c.Spec.SessionRecording == "" {
		c.Spec.SessionRecording = RecordAtNode
	}

	// check if the recording type is valid
	all := []string{string(RecordAtNode), string(RecordAtProxy), string(RecordOff)}
	ok := utils.SliceContainsStr(all, string(c.Spec.SessionRecording))
	if !ok {
		return trace.BadParameter("session_recording must either be: %v", strings.Join(all, ","))
	}

	return nil
}

// String represents a human readable version of the cluster name.
func (c *ClusterConfigV3) String() string {
	return fmt.Sprintf("ClusterConfig(SessionRecording=%v)", c.Spec.SessionRecording)
}

// ClusterConfigSpecSchemaTemplate is a template for ClusterConfig schema.
const ClusterConfigSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "session_recording": {
      "type": "string"
    }%v
  }
}`

// GetClusterConfigSchema returns the schema with optionally injected
// schema for extensions.
func GetClusterConfigSchema(extensionSchema string) string {
	var clusterConfigSchema string
	if clusterConfigSchema == "" {
		clusterConfigSchema = fmt.Sprintf(ClusterConfigSpecSchemaTemplate, "")
	} else {
		clusterConfigSchema = fmt.Sprintf(ClusterConfigSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, clusterConfigSchema, DefaultDefinitions)
}

// ClusterConfigMarshaler implements marshal/unmarshal of ClusterConfig implementations
// mostly adds support for extended versions.
type ClusterConfigMarshaler interface {
	Marshal(c ClusterConfig, opts ...MarshalOption) ([]byte, error)
	Unmarshal(bytes []byte) (ClusterConfig, error)
}

var clusterConfigMarshaler ClusterConfigMarshaler = &TeleportClusterConfigMarshaler{}

// SetClusterConfigMarshaler sets the marshaler.
func SetClusterConfigMarshaler(m ClusterConfigMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	clusterConfigMarshaler = m
}

// GetClusterConfigMarshaler gets the marshaler.
func GetClusterConfigMarshaler() ClusterConfigMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return clusterConfigMarshaler
}

// TeleportClusterConfigMarshaler is used to marshal and unmarshal ClusterConfig.
type TeleportClusterConfigMarshaler struct{}

// Unmarshal unmarshals ClusterConfig from JSON.
func (t *TeleportClusterConfigMarshaler) Unmarshal(bytes []byte) (ClusterConfig, error) {
	var clusterConfig ClusterConfigV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	err := utils.UnmarshalWithSchema(GetClusterConfigSchema(""), &clusterConfig, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	err = clusterConfig.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &clusterConfig, nil
}

// Marshal marshals ClusterConfig to JSON.
func (t *TeleportClusterConfigMarshaler) Marshal(c ClusterConfig, opts ...MarshalOption) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return b, nil
}
