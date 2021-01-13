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

package types

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// ClusterName defines the name of the cluster. This is a configuration
// resource, never create more than one instance of it.
type ClusterName interface {
	// Resource provides common resource properties.
	Resource

	// SetClusterName sets the name of the cluster.
	SetClusterName(string)
	// GetClusterName gets the name of the cluster.
	GetClusterName() string

	// CheckAndSetDefaults checks and set default values for missing fields.
	CheckAndSetDefaults() error
}

// NewClusterName is a convenience wrapper to create a ClusterName resource.
func NewClusterName(spec ClusterNameSpecV2) (ClusterName, error) {
	cn := ClusterNameV2{
		Kind:    KindClusterName,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameClusterName,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
	if err := cn.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cn, nil
}

// GetVersion returns resource version
func (c *ClusterNameV2) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *ClusterNameV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *ClusterNameV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *ClusterNameV2) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetResourceID returns resource ID
func (c *ClusterNameV2) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *ClusterNameV2) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetName returns the name of the cluster.
func (c *ClusterNameV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster.
func (c *ClusterNameV2) SetName(e string) {
	c.Metadata.Name = e
}

// Expiry returns object expiry setting
func (c *ClusterNameV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *ClusterNameV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using realtime clock
func (c *ClusterNameV2) SetTTL(clock Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *ClusterNameV2) GetMetadata() Metadata {
	return c.Metadata
}

// SetClusterName sets the name of the cluster.
func (c *ClusterNameV2) SetClusterName(n string) {
	c.Spec.ClusterName = n
}

// GetClusterName gets the name of the cluster.
func (c *ClusterNameV2) GetClusterName() string {
	return c.Spec.ClusterName
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *ClusterNameV2) CheckAndSetDefaults() error {
	// make sure we have defaults for all metadata fields
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	if c.Spec.ClusterName == "" {
		return trace.BadParameter("cluster name is required")
	}

	return nil
}

// String represents a human readable version of the cluster name.
func (c *ClusterNameV2) String() string {
	return fmt.Sprintf("ClusterName(%v)", c.Spec.ClusterName)
}

// ClusterNameSpecSchemaTemplate is a template for ClusterName schema.
const ClusterNameSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "cluster_name": {
      "type": "string"
    }%v
  }
}`

// GetClusterNameSchema returns the schema with optionally injected
// schema for extensions.
func GetClusterNameSchema(extensionSchema string) string {
	var clusterNameSchema string
	if clusterNameSchema == "" {
		clusterNameSchema = fmt.Sprintf(ClusterNameSpecSchemaTemplate, "")
	} else {
		clusterNameSchema = fmt.Sprintf(ClusterNameSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, clusterNameSchema, DefaultDefinitions)
}

// ClusterNameMarshaler implements marshal/unmarshal of ClusterName implementations
// mostly adds support for extended versions.
type ClusterNameMarshaler interface {
	Marshal(c ClusterName, opts ...MarshalOption) ([]byte, error)
	Unmarshal(bytes []byte, opts ...MarshalOption) (ClusterName, error)
}

type teleportClusterNameMarshaler struct{}

// Unmarshal unmarshals ClusterName from JSON.
func (t *teleportClusterNameMarshaler) Unmarshal(bytes []byte, opts ...MarshalOption) (ClusterName, error) {
	var clusterName ClusterNameV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &clusterName); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err = utils.UnmarshalWithSchema(GetClusterNameSchema(""), &clusterName, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	err = clusterName.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		clusterName.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		clusterName.SetExpiry(cfg.Expires)
	}

	return &clusterName, nil
}

// Marshal marshals ClusterName to JSON.
func (t *teleportClusterNameMarshaler) Marshal(c ClusterName, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := c.(type) {
	case *ClusterNameV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *resource
			copy.SetResourceID(0)
			resource = &copy
		}
		return utils.FastMarshal(resource)
	default:
		return nil, trace.BadParameter("unrecognized resource version %T", c)
	}
}

var clusterNameMarshaler ClusterNameMarshaler = &teleportClusterNameMarshaler{}

// SetClusterNameMarshaler sets the marshaler.
func SetClusterNameMarshaler(m ClusterNameMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	clusterNameMarshaler = m
}

// GetClusterNameMarshaler gets the marshaler.
func GetClusterNameMarshaler() ClusterNameMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return clusterNameMarshaler
}
