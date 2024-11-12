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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// RemoteCluster represents a remote cluster that has connected via reverse tunnel
// to this cluster
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

	// SetMetadata sets remote cluster metatada
	SetMetadata(Metadata)

	// Clone performs a deep copy.
	Clone() RemoteCluster

	// GetLabel retrieves the label with the provided key. If not found value
	// will be empty and ok will be false.
	GetLabel(key string) (value string, ok bool)

	// GetAllLabels returns all labels for the remote cluster
	GetAllLabels() map[string]string
}

// NewRemoteCluster is a convenience way to create a RemoteCluster resource.
func NewRemoteCluster(name string) (RemoteCluster, error) {
	c := &RemoteClusterV3{
		Metadata: Metadata{
			Name: name,
		},
	}
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
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

// GetRevision returns the revision
func (c *RemoteClusterV3) GetRevision() string {
	return c.Metadata.GetRevision()
}

// SetRevision sets the revision
func (c *RemoteClusterV3) SetRevision(rev string) {
	c.Metadata.SetRevision(rev)
}

// setStaticFields sets static resource header and metadata fields.
func (c *RemoteClusterV3) setStaticFields() {
	c.Kind = KindRemoteCluster
	c.Version = V3
}

// CheckAndSetDefaults checks and sets default values
func (c *RemoteClusterV3) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetLastHeartbeat returns last heartbeat of the cluster
func (c *RemoteClusterV3) GetLastHeartbeat() time.Time {
	return c.Status.LastHeartbeat
}

// SetLastHeartbeat sets last heartbeat of the cluster
func (c *RemoteClusterV3) SetLastHeartbeat(t time.Time) {
	c.Status.LastHeartbeat = t
}

// Clone performs a deep copy.
func (c *RemoteClusterV3) Clone() RemoteCluster {
	return utils.CloneProtoMsg(c)
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

// SetMetadata sets remote cluster metatada
func (c *RemoteClusterV3) SetMetadata(meta Metadata) {
	c.Metadata = meta
}

// SetExpiry sets expiry time for the object
func (c *RemoteClusterV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (c *RemoteClusterV3) Expiry() time.Time {
	return c.Metadata.Expiry()
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
func (c *RemoteClusterV3) String() string {
	return fmt.Sprintf("RemoteCluster(%v, %v)", c.Metadata.Name, c.Status.Connection)
}

// GetLabel retrieves the label with the provided key. If not found value
// will be empty and ok will be false.
func (c *RemoteClusterV3) GetLabel(key string) (value string, ok bool) {
	value, ok = c.Metadata.Labels[key]
	return value, ok
}

// GetAllLabels returns all labels for the remote cluster. Remote clusters only
// have static labels.
func (c *RemoteClusterV3) GetAllLabels() map[string]string {
	return c.Metadata.Labels
}
