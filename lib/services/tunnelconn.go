package services

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
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
	// GetClusterName returns name of the cluster
	// this connection is for
	GetClusterName() string
	// GetProxyName returns the proxy name this connection is established to
	GetProxyName() string
	// GetLastHeartbeat returns time of the last heartbeat received from
	// the tunnel over the connection
	GetLastHeartbeat() time.Time
	// SetLastHeartbeat sets last heartbeat time
	SetLastHeartbeat(time.Time)
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

// TunnelConnectionV2 is version 1 resource spec of the reverse tunnel
type TunnelConnectionV2 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains user specification
	Spec TunnelConnectionSpecV2 `json:"spec"`
}

// Clone returns a copy of this tunnel connection
func (r *TunnelConnectionV2) Clone() TunnelConnection {
	out := *r
	return &out
}

// String returns user-friendly description of this connection
func (r *TunnelConnectionV2) String() string {
	return fmt.Sprintf("TunnelConnection(name=%v, cluster=%v, proxy=%v)", r.Metadata.Name, r.Spec.ClusterName, r.Spec.ProxyName)
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

// TunnelConnectionSpecV2 is a specification for V2 tunnel connection
type TunnelConnectionSpecV2 struct {
	// ClusterName is a name of the cluster
	ClusterName string `json:"cluster_name"`
	// ProxyName is the name of the proxy server
	ProxyName string `json:"proxy_name"`
	// LastHeartbeat is a time of the last heartbeat
	LastHeartbeat time.Time `json:"last_heartbeat,omitempty"`
}

// TunnelConnectionSpecV2Schema is JSON schema for reverse tunnel spec
const TunnelConnectionSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["cluster_name", "proxy_name", "last_heartbeat"],
  "properties": {
    "cluster_name": {"type": "string"},
    "proxy_name": {"type": "string"},
    "last_heartbeat": {"type": "string"}
  }
}`

// GetTunnelConnectionSchema returns role schema with optionally injected
// schema for extensions
func GetTunnelConnectionSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, TunnelConnectionSpecV2Schema, DefaultDefinitions)
}

// UnmarshalTunnelConnection unmarshals reverse tunnel from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalTunnelConnection(data []byte) (TunnelConnection, error) {
	if len(data) == 0 {
		debug.PrintStack()
		return nil, trace.BadParameter("missing tunnel connection data")
	}
	var h ResourceHeader
	err := json.Unmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V2:
		var r TunnelConnectionV2

		if err := utils.UnmarshalWithSchema(GetTunnelConnectionSchema(), &r, data); err != nil {
			return nil, trace.BadParameter(err.Error())
		}

		if err := r.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return &r, nil
	}
	return nil, trace.BadParameter("reverse tunnel version %v is not supported", h.Version)
}

// MarshalTunnelConnection marshals tunnel connection
func MarshalTunnelConnection(rt TunnelConnection, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(rt)
}
