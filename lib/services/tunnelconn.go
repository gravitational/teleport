/*
Copyright 2015-2019 Gravitational, Inc.

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
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
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

// LatestTunnelConnection returns latest tunnel connection from the list
// of tunnel connections, if no connections found, returns NotFound error
func LatestTunnelConnection(conns []TunnelConnection) (TunnelConnection, error) {
	var lastConn TunnelConnection
	for i := range conns {
		conn := conns[i]
		if lastConn == nil || conn.GetLastHeartbeat().After(lastConn.GetLastHeartbeat()) {
			lastConn = conn
		}
	}
	if lastConn == nil {
		return nil, trace.NotFound("no connections found")
	}
	return lastConn, nil
}

// IsTunnelConnectionStatus returns tunnel connection status based on the last
// heartbeat time recorded for a connection
func TunnelConnectionStatus(clock clockwork.Clock, conn TunnelConnection) string {
	diff := clock.Now().Sub(conn.GetLastHeartbeat())
	if diff < defaults.ReverseTunnelOfflineThreshold {
		return teleport.RemoteClusterStatusOnline
	}
	return teleport.RemoteClusterStatusOffline
}

// MustCreateTunnelConnection returns new connection from V2 spec or panics if
// parameters are incorrect
func MustCreateTunnelConnection(name string, spec TunnelConnectionSpecV2) TunnelConnection {
	conn, err := NewTunnelConnection(name, spec)
	if err != nil {
		panic(err)
	}
	return conn
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

// Expires returns object expiry setting
func (r *TunnelConnectionV2) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (r *TunnelConnectionV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
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

func (r *TunnelConnectionV2) CheckAndSetDefaults() error {
	err := r.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	err = r.Check()
	if err != nil {
		return trace.Wrap(err)
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

// TunnelConnectionSpecV2Schema is JSON schema for reverse tunnel spec
const TunnelConnectionSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["cluster_name", "proxy_name", "last_heartbeat"],
  "properties": {
    "cluster_name": {"type": "string"},
    "proxy_name": {"type": "string"},
    "last_heartbeat": {"type": "string"},
    "type": {"type": "string"}
  }
}`

// GetTunnelConnectionSchema returns role schema with optionally injected
// schema for extensions
func GetTunnelConnectionSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, TunnelConnectionSpecV2Schema, DefaultDefinitions)
}

// UnmarshalTunnelConnection unmarshals reverse tunnel from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalTunnelConnection(data []byte, opts ...MarshalOption) (TunnelConnection, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing tunnel connection data")
	}
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	err = utils.FastUnmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V2:
		var r TunnelConnectionV2

		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetTunnelConnectionSchema(), &r, data); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}

		if err := r.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			r.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			r.SetExpiry(cfg.Expires)
		}
		return &r, nil
	}
	return nil, trace.BadParameter("reverse tunnel version %v is not supported", h.Version)
}

// MarshalTunnelConnection marshals tunnel connection
func MarshalTunnelConnection(rt TunnelConnection, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := rt.(type) {
	case *TunnelConnectionV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *resource
			copy.SetResourceID(0)
			resource = &copy
		}
		return utils.FastMarshal(resource)
	default:
		return nil, trace.BadParameter("unrecognized resource version %T", rt)
	}
}
