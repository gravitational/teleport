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
	"strings"
	"time"

	"github.com/gravitational/teleport/api/defaults"

	"github.com/gravitational/trace"
)

// TunnelConnection is SSH reverse tunnel connection
// established to reverse tunnel proxy
type TunnelConnection interface {
	// Resource provides common methods for resource objects
	Resource
	// GetClusterName returns name of the cluster this connection is for.
	GetClusterName() string
	// GetProxyName returns the proxy name this connection is established to
	GetProxyName() string
	// GetLastHeartbeat returns time of the last heartbeat received from
	// the tunnel over the connection
	GetLastHeartbeat() time.Time
	// SetLastHeartbeat sets last heartbeat time
	SetLastHeartbeat(time.Time)
	// GetType gets the type of ReverseTunnel.
	GetType() TunnelType
	// SetType sets the type of ReverseTunnel.
	SetType(TunnelType)
	// Check checks tunnel for errors
	Check() error
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
	// String returns user friendly representation of this connection
	String() string
	// Clone returns a copy of this tunnel connection
	Clone() TunnelConnection
}

// NewTunnelConnection returns new connection from V2 spec
func NewTunnelConnection(name string, spec TunnelConnectionSpecV2) (TunnelConnection, error) {
	conn := &TunnelConnectionV2{
		Kind:    KindTunnelConnection,
		SubKind: spec.ClusterName,
		Version: V2,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
	if err := conn.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// GetVersion returns resource version
func (r *TunnelConnectionV2) GetVersion() string {
	return r.Version
}

// GetKind returns resource kind
func (r *TunnelConnectionV2) GetKind() string {
	return r.Kind
}

// GetSubKind returns resource sub kind
func (r *TunnelConnectionV2) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *TunnelConnectionV2) SetSubKind(s string) {
	r.SubKind = s
}

// GetResourceID returns resource ID
func (r *TunnelConnectionV2) GetResourceID() int64 {
	return r.Metadata.ID
}

// SetResourceID sets resource ID
func (r *TunnelConnectionV2) SetResourceID(id int64) {
	r.Metadata.ID = id
}

// Clone returns a copy of this tunnel connection
func (r *TunnelConnectionV2) Clone() TunnelConnection {
	out := *r
	return &out
}

// String returns user-friendly description of this connection
func (r *TunnelConnectionV2) String() string {
	return fmt.Sprintf("TunnelConnection(name=%v, type=%v, cluster=%v, proxy=%v)",
		r.Metadata.Name, r.Spec.Type, r.Spec.ClusterName, r.Spec.ProxyName)
}

// GetMetadata returns object metadata
func (r *TunnelConnectionV2) GetMetadata() Metadata {
	return r.Metadata
}

// SetExpiry sets expiry time for the object
func (r *TunnelConnectionV2) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (r *TunnelConnectionV2) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (r *TunnelConnectionV2) SetTTL(clock Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// GetName returns the name of the User
func (r *TunnelConnectionV2) GetName() string {
	return r.Metadata.Name
}

// SetName sets the name of the User
func (r *TunnelConnectionV2) SetName(e string) {
	r.Metadata.Name = e
}

// V2 returns V2 version of the resource
func (r *TunnelConnectionV2) V2() *TunnelConnectionV2 {
	return r
}

// CheckAndSetDefaults checks and sets default values
func (r *TunnelConnectionV2) CheckAndSetDefaults() error {
	err := r.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	err = r.Check()
	if err != nil {
		return trace.Wrap(err)
	}

	if r.Expiry().IsZero() {
		// calculate an appropriate expiry if one was not provided.
		// tunnel connection resources are ephemeral and trigger
		// allocations in proxies, so it is important that they expire
		// in a timely manner.
		from := r.GetLastHeartbeat()
		if from.IsZero() {
			from = time.Now()
		}
		r.SetExpiry(from.UTC().Add(defaults.ServerAnnounceTTL))
	}

	return nil
}

// GetClusterName returns name of the cluster
func (r *TunnelConnectionV2) GetClusterName() string {
	return r.Spec.ClusterName
}

// GetProxyName returns the name of the proxy
func (r *TunnelConnectionV2) GetProxyName() string {
	return r.Spec.ProxyName
}

// GetLastHeartbeat returns last heartbeat
func (r *TunnelConnectionV2) GetLastHeartbeat() time.Time {
	return r.Spec.LastHeartbeat
}

// SetLastHeartbeat sets last heartbeat time
func (r *TunnelConnectionV2) SetLastHeartbeat(tm time.Time) {
	r.Spec.LastHeartbeat = tm
}

// GetType gets the type of ReverseTunnel.
func (r *TunnelConnectionV2) GetType() TunnelType {
	if string(r.Spec.Type) == "" {
		return ProxyTunnel
	}
	return r.Spec.Type
}

// SetType sets the type of ReverseTunnel.
func (r *TunnelConnectionV2) SetType(tt TunnelType) {
	r.Spec.Type = tt
}

// Check returns nil if all parameters are good, error otherwise
func (r *TunnelConnectionV2) Check() error {
	if r.Version == "" {
		return trace.BadParameter("missing version")
	}
	if strings.TrimSpace(r.Spec.ClusterName) == "" {
		return trace.BadParameter("empty cluster name")
	}

	if len(r.Spec.ProxyName) == 0 {
		return trace.BadParameter("missing parameter proxy name")
	}

	return nil
}
