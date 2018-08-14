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
	GetSessionRecording() string

	// SetSessionRecording sets where the session is recorded.
	SetSessionRecording(string)

	// GetClusterID returns the unique cluster ID
	GetClusterID() string

	// SetClusterID sets the cluster ID
	SetClusterID(string)

	// GetProxyChecksHostKeys sets if the proxy will check host keys.
	GetProxyChecksHostKeys() string

	// SetProxyChecksHostKeys gets if the proxy will check host keys.
	SetProxyChecksHostKeys(string)

	// CheckAndSetDefaults checks and set default values for missing fields.
	CheckAndSetDefaults() error

	// GetAuditConfig returns audit settings
	GetAuditConfig() AuditConfig

	// SetAuditConfig sets audit config
	SetAuditConfig(AuditConfig)

	// GetClientIdleTimeout returns client idle timeout setting
	GetClientIdleTimeout() time.Duration

	// SetClientIdleTimeout sets client idle timeout setting
	SetClientIdleTimeout(t time.Duration)

	// GetDisconnectExpiredCert returns disconnect expired certificate setting
	GetDisconnectExpiredCert() bool

	// SetDisconnectExpiredCert sets disconnect client with expired certificate setting
	SetDisconnectExpiredCert(bool)

	// Copy creates a copy of the resource and returns it.
	Copy() ClusterConfig
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
			SessionRecording:    RecordAtNode,
			ProxyChecksHostKeys: HostKeyCheckYes,
		},
	}
}

// AuditConfig represents audit log settings in the cluster
type AuditConfig struct {
	// Type is audit backend type
	Type string `json:"type,omitempty"`
	// Region is a region setting for audit sessions used by cloud providers
	Region string `json:"region,omitempty"`
	// AuditSessionsURI is a parameter where to upload sessions
	AuditSessionsURI string `json:"audit_sessions_uri,omitempty"`
	// AuditEventsURI is a parameter with all supported outputs
	// for audit events
	AuditEventsURI utils.Strings `json:"audit_events_uri,omitempty"`
	// AuditTableName is a DB table name used for audits
	// Deprecated in favor of AuditEventsURI
	// DELETE IN (3.1.0)
	AuditTableName string `json:"audit_table_name,omitempty"`
}

// ShouldUploadSessions returns whether audit config
// instructs server to upload sessions
func (a AuditConfig) ShouldUploadSessions() bool {
	return a.AuditSessionsURI != ""
}

// AuditConfigFromObject returns audit config from interface object
func AuditConfigFromObject(in interface{}) (*AuditConfig, error) {
	var cfg AuditConfig
	if in == nil {
		return &cfg, nil
	}
	if err := utils.ObjectToStruct(in, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cfg, nil
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

const (
	// RecordAtNode is the default. Sessions are recorded at Teleport nodes.
	RecordAtNode string = "node"

	// RecordAtProxy enables the recording proxy which intercepts and records
	// all sessions.
	RecordAtProxy string = "proxy"

	// RecordOff is used to disable session recording completely.
	RecordOff string = "off"
)

const (
	// HostKeyCheckYes is the default. The proxy will check the host key of the
	// target node it connects to.
	HostKeyCheckYes string = "yes"

	// HostKeyCheckNo is used to disable host key checking. This is a insecure
	// settings which makes MITM possible with no indications, use with caution.
	HostKeyCheckNo string = "no"
)

// ClusterConfigSpecV3 is the actual data we care about for ClusterConfig.
type ClusterConfigSpecV3 struct {
	// SessionRecording controls where (or if) the session is recorded.
	SessionRecording string `json:"session_recording"`

	// ClusterID is the unique cluster ID that is set once during the first auth
	// server startup.
	ClusterID string `json:"cluster_id"`

	// ProxyChecksHostKeys is used to control if the proxy will check host keys
	// when in recording mode.
	ProxyChecksHostKeys string `json:"proxy_checks_host_keys"`

	// Audit is a section with audit config
	Audit AuditConfig `json:"audit"`

	// ClientIdleTimeout sets global cluster default setting for client idle timeouts
	ClientIdleTimeout Duration `json:"client_idle_timeout"`

	// DisconnectExpiredCert provides disconnect expired certificate setting -
	// if true, connections with expired client certificates will get disconnected
	DisconnectExpiredCert Bool `json:"disconnect_expired_cert"`
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
func (c *ClusterConfigV3) GetSessionRecording() string {
	return c.Spec.SessionRecording
}

// SetClusterConfig sets the name of the cluster.
func (c *ClusterConfigV3) SetSessionRecording(s string) {
	c.Spec.SessionRecording = s
}

// GetClusterID returns the unique cluster ID
func (c *ClusterConfigV3) GetClusterID() string {
	return c.Spec.ClusterID
}

// SetClusterID sets the cluster ID
func (c *ClusterConfigV3) SetClusterID(id string) {
	c.Spec.ClusterID = id
}

// GetProxyChecksHostKeys sets if the proxy will check host keys.
func (c *ClusterConfigV3) GetProxyChecksHostKeys() string {
	return c.Spec.ProxyChecksHostKeys
}

// SetProxyChecksHostKeys sets if the proxy will check host keys.
func (c *ClusterConfigV3) SetProxyChecksHostKeys(t string) {
	c.Spec.ProxyChecksHostKeys = t
}

// GetAuditConfig returns audit settings
func (c *ClusterConfigV3) GetAuditConfig() AuditConfig {
	return c.Spec.Audit
}

// SetAuditConfig sets audit config
func (c *ClusterConfigV3) SetAuditConfig(cfg AuditConfig) {
	c.Spec.Audit = cfg
}

// GetClientIdleTimeout returns client idle timeout setting
func (c *ClusterConfigV3) GetClientIdleTimeout() time.Duration {
	return c.Spec.ClientIdleTimeout.Duration
}

// SetClientIdleTimeout sets client idle timeout setting
func (c *ClusterConfigV3) SetClientIdleTimeout(d time.Duration) {
	c.Spec.ClientIdleTimeout.Duration = d
}

// GetDisconnectExpiredCert returns disconnect expired certificate setting
func (c *ClusterConfigV3) GetDisconnectExpiredCert() bool {
	return c.Spec.DisconnectExpiredCert.bool
}

// SetDisconnectExpiredCert sets disconnect client with expired certificate setting
func (c *ClusterConfigV3) SetDisconnectExpiredCert(b bool) {
	c.Spec.DisconnectExpiredCert.bool = b
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
	if c.Spec.ProxyChecksHostKeys == "" {
		c.Spec.ProxyChecksHostKeys = HostKeyCheckYes
	}

	// check if the recording type is valid
	all := []string{RecordAtNode, RecordAtProxy, RecordOff}
	ok := utils.SliceContainsStr(all, c.Spec.SessionRecording)
	if !ok {
		return trace.BadParameter("session_recording must either be: %v", strings.Join(all, ","))
	}

	// check if host key checking mode is valid
	all = []string{HostKeyCheckYes, HostKeyCheckNo}
	ok = utils.SliceContainsStr(all, c.Spec.ProxyChecksHostKeys)
	if !ok {
		return trace.BadParameter("proxy_checks_host_keys must be one of: %v", strings.Join(all, ","))
	}

	return nil
}

// Copy creates a copy of the resource and returns it.
func (c *ClusterConfigV3) Copy() ClusterConfig {
	out := *c
	return &out
}

// String represents a human readable version of the cluster name.
func (c *ClusterConfigV3) String() string {
	return fmt.Sprintf("ClusterConfig(SessionRecording=%v, ClusterID=%v, ProxyChecksHostKeys=%v)",
		c.Spec.SessionRecording, c.Spec.ClusterID, c.Spec.ProxyChecksHostKeys)
}

// ClusterConfigSpecSchemaTemplate is a template for ClusterConfig schema.
const ClusterConfigSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "session_recording": {
      "type": "string"
    },
    "proxy_checks_host_keys": {
      "type": "string"
    },
    "cluster_id": {
      "type": "string"
    },
    "client_idle_timeout": {
      "type": "string"
    },
    "disconnect_expired_cert": {
      "anyOf": [{"type": "string"}, { "type": "boolean"}]
    },
    "audit": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "type": {
          "type": "string"
         }, 
        "region": {
          "type": "string"
         }, 
        "audit_sessions_uri": {
          "type": "string"
         }, 
        "audit_table_name": {
          "type": "string"
         }
      }
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
