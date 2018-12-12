/*
Copyright 2017-2019 Gravitational, Inc.

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
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// RemoteCluster represents a remote cluster that has connected via reverse tunnel
// to this lcuster
type RemoteCluster interface {
	// Resource provides common resource properties
	Resource
	// GetConnectionStatus returns connection status
	GetConnectionStatus() string
	// SetConnectionStatus sets connection  status
	SetConnectionStatus(string)

	// GetLastHeartbeat returns last heartbeat of the cluster
	GetLastHeartbeat() time.Time
	// SetLastHeartbeat sets last heartbeat of the cluster
	SetLastHeartbeat(t time.Time)

	// CheckAndSetDefaults checks and sets default values
	CheckAndSetDefaults() error
}

// NewRemoteCluster is a convenience wa to create a RemoteCluster resource.
func NewRemoteCluster(name string) (RemoteCluster, error) {
	return &RemoteClusterV3{
		Kind:    KindRemoteCluster,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
	}, nil
}

// RemoteClusterV3 implements RemoteCluster.
type RemoteClusterV3 struct {
	// Kind is a resource kind - always resource.
	Kind string `json:"kind"`

	// SubKind is a resource sub kind
	SubKind string `json:"sub_kind,omitempty"`

	// Version is a resource version.
	Version string `json:"version"`

	// Metadata is metadata about the resource.
	Metadata Metadata `json:"metadata"`

	// Sstatus is read only status of the remote cluster
	Status RemoteClusterStatusV3 `json:"status"`
}

// RemoteClusterSpecV3 represents status of the remote cluster
type RemoteClusterStatusV3 struct {
	// Connection represents connection status, online or offline
	Connection string `json:"connection"`
	// LastHeartbeat records last heartbeat of the cluster
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// GetVersion returns resource version
func (c *RemoteClusterV3) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *RemoteClusterV3) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *RemoteClusterV3) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *RemoteClusterV3) SetSubKind(s string) {
	c.SubKind = s
}

// GetResourceID returns resource ID
func (c *RemoteClusterV3) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *RemoteClusterV3) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// CheckAndSetDefaults checks and sets default values
func (c *RemoteClusterV3) CheckAndSetDefaults() error {
	return c.Metadata.CheckAndSetDefaults()
}

// GetLastHeartbeat returns last heartbeat of the cluster
func (c *RemoteClusterV3) GetLastHeartbeat() time.Time {
	return c.Status.LastHeartbeat
}

// SetLastHeartbeat sets last heartbeat of the cluster
func (c *RemoteClusterV3) SetLastHeartbeat(t time.Time) {
	c.Status.LastHeartbeat = t
}

// GetConnectionStatus returns connection status
func (c *RemoteClusterV3) GetConnectionStatus() string {
	return c.Status.Connection
}

// SetConnectionStatus sets connection  status
func (c *RemoteClusterV3) SetConnectionStatus(status string) {
	c.Status.Connection = status
}

// GetMetadata returns object metadata
func (c *RemoteClusterV3) GetMetadata() Metadata {
	return c.Metadata
}

// SetExpiry sets expiry time for the object
func (c *RemoteClusterV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expires returns object expiry setting
func (c *RemoteClusterV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (c *RemoteClusterV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetName returns the name of the RemoteCluster.
func (c *RemoteClusterV3) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the RemoteCluster.
func (c *RemoteClusterV3) SetName(e string) {
	c.Metadata.Name = e
}

// String represents a human readable version of remote cluster settings.
func (r *RemoteClusterV3) String() string {
	return fmt.Sprintf("RemoteCluster(%v, %v)", r.Metadata.Name, r.Status.Connection)
}

// RemoteClusterSchemaTemplate is a template JSON Schema for V3 style objects
const RemoteClusterV3SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "metadata", "version"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v3"},
    "metadata": %v,
    "status": %v
  }
}`

// RemoteClusterV3StatusSchema is a template for remote
const RemoteClusterV3StatusSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["connection", "last_heartbeat"],
  "properties": {
    "connection": {"type": "string"},
    "last_heartbeat": {"type": "string"}
  }
}`

// GetRemoteClusterSchema returns the schema for remote cluster
func GetRemoteClusterSchema() string {
	return fmt.Sprintf(RemoteClusterV3SchemaTemplate, MetadataSchema, RemoteClusterV3StatusSchema)
}

// UnmarshalRemoteCluster unmarshals remote cluster from JSON or YAML.
func UnmarshalRemoteCluster(bytes []byte, opts ...MarshalOption) (RemoteCluster, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cluster RemoteClusterV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	if cfg.SkipValidation {
		err := utils.FastUnmarshal(bytes, &cluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		err = utils.UnmarshalWithSchema(GetRemoteClusterSchema(), &cluster, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	err = cluster.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &cluster, nil
}

// MarshalRemoteCluster marshals remote cluster to JSON.
func MarshalRemoteCluster(c RemoteCluster, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := c.(type) {
	case *RemoteClusterV3:
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
