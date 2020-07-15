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

	// GetSessionControlTimeout gets the session control timeout.
	GetSessionControlTimeout() time.Duration

	// SetSessionControlTimeout sets the session control timeout.
	SetSessionControlTimeout(t time.Duration)

	// GetDisconnectExpiredCert returns disconnect expired certificate setting
	GetDisconnectExpiredCert() bool

	// SetDisconnectExpiredCert sets disconnect client with expired certificate setting
	SetDisconnectExpiredCert(bool)

	// GetKeepAliveInterval gets the keep-alive interval for server to client
	// connections.
	GetKeepAliveInterval() time.Duration

	// SetKeepAliveInterval sets the keep-alive interval for server to client
	// connections.
	SetKeepAliveInterval(t time.Duration)

	// GetKeepAliveCountMax gets the number of missed keep-alive messages before
	// the server disconnects the client.
	GetKeepAliveCountMax() int64

	// SetKeepAliveCountMax sets the number of missed keep-alive messages before
	// the server disconnects the client.
	SetKeepAliveCountMax(c int64)

	// GetLocalAuth gets if local authentication is allowed.
	GetLocalAuth() bool

	// SetLocalAuth sets if local authentication is allowed.
	SetLocalAuth(bool)

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
			KeepAliveInterval:   NewDuration(defaults.KeepAliveInterval),
			KeepAliveCountMax:   int64(defaults.KeepAliveCountMax),
			LocalAuth:           NewBool(true),
		},
	}
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

const (
	// RecordAtNode is the default. Sessions are recorded at Teleport nodes.
	RecordAtNode string = "node"

	// RecordAtProxy enables the recording proxy which intercepts and records
	// all sessions.
	RecordAtProxy string = "proxy"

	// RecordOff is used to disable session recording completely.
	RecordOff string = "off"

	// RecordAtNodeSync enables the nodes to stream sessions in sync mode
	// to the auth server
	RecordAtNodeSync string = "node-sync"

	// RecordAtProxySync enables the recording proxy which intercepts and records
	// all sessions, streams the records synchronously
	RecordAtProxySync string = "proxy-sync"
)

// IsRecordAtProxy returns true if recording is sync or async at proxy
func IsRecordAtProxy(mode string) bool {
	return mode == RecordAtProxy || mode == RecordAtProxySync
}

// IsRecordSync returns true if recording is sync or async for proxy or node
func IsRecordSync(mode string) bool {
	return mode == RecordAtProxySync || mode == RecordAtNodeSync
}

const (
	// HostKeyCheckYes is the default. The proxy will check the host key of the
	// target node it connects to.
	HostKeyCheckYes string = "yes"

	// HostKeyCheckNo is used to disable host key checking. This is a insecure
	// settings which makes MITM possible with no indications, use with caution.
	HostKeyCheckNo string = "no"
)

// GetVersion returns resource version
func (c *ClusterConfigV3) GetVersion() string {
	return c.Version
}

// GetSubKind returns resource subkind
func (c *ClusterConfigV3) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *ClusterConfigV3) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetKind returns resource kind
func (c *ClusterConfigV3) GetKind() string {
	return c.Kind
}

// GetResourceID returns resource ID
func (c *ClusterConfigV3) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *ClusterConfigV3) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetName returns the name of the cluster.
func (c *ClusterConfigV3) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster.
func (c *ClusterConfigV3) SetName(e string) {
	c.Metadata.Name = e
}

// Expires returns object expiry setting
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
	return c.Spec.ClientIdleTimeout.Duration()
}

// SetClientIdleTimeout sets client idle timeout setting
func (c *ClusterConfigV3) SetClientIdleTimeout(d time.Duration) {
	c.Spec.ClientIdleTimeout = Duration(d)
}

// GetSessionControlTimeout gets the session control timeout.
func (c *ClusterConfigV3) GetSessionControlTimeout() time.Duration {
	return c.Spec.SessionControlTimeout.Duration()
}

// SetSessionControlTimeout sets the session control timeout.
func (c *ClusterConfigV3) SetSessionControlTimeout(d time.Duration) {
	c.Spec.SessionControlTimeout = Duration(d)
}

// GetDisconnectExpiredCert returns disconnect expired certificate setting
func (c *ClusterConfigV3) GetDisconnectExpiredCert() bool {
	return c.Spec.DisconnectExpiredCert.Value()
}

// SetDisconnectExpiredCert sets disconnect client with expired certificate setting
func (c *ClusterConfigV3) SetDisconnectExpiredCert(b bool) {
	c.Spec.DisconnectExpiredCert = NewBool(b)
}

// GetKeepAliveInterval gets the keep-alive interval.
func (c *ClusterConfigV3) GetKeepAliveInterval() time.Duration {
	return c.Spec.KeepAliveInterval.Duration()
}

// SetKeepAliveInterval sets the keep-alive interval.
func (c *ClusterConfigV3) SetKeepAliveInterval(t time.Duration) {
	c.Spec.KeepAliveInterval = Duration(t)
}

// GetKeepAliveCountMax gets the number of missed keep-alive messages before
// the server disconnects the client.
func (c *ClusterConfigV3) GetKeepAliveCountMax() int64 {
	return c.Spec.KeepAliveCountMax
}

// SetKeepAliveCountMax sets the number of missed keep-alive messages before
// the server disconnects the client.
func (c *ClusterConfigV3) SetKeepAliveCountMax(m int64) {
	c.Spec.KeepAliveCountMax = m
}

// GetLocalAuth gets if local authentication is allowed.
func (c *ClusterConfigV3) GetLocalAuth() bool {
	return c.Spec.LocalAuth.Value()
}

// SetLocalAuth gets if local authentication is allowed.
func (c *ClusterConfigV3) SetLocalAuth(b bool) {
	c.Spec.LocalAuth = NewBool(b)
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
	all := []string{RecordAtNode, RecordAtProxy, RecordAtNodeSync, RecordAtProxySync, RecordOff}
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

	// Set the keep-alive interval and max missed keep-alives before the
	// client is disconnected.
	if c.Spec.KeepAliveInterval.Duration() == 0 {
		c.Spec.KeepAliveInterval = NewDuration(defaults.KeepAliveInterval)
	}
	if c.Spec.KeepAliveCountMax == 0 {
		c.Spec.KeepAliveCountMax = int64(defaults.KeepAliveCountMax)
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
    "session_control_timeout": {
      "type": "string"
    },
    "disconnect_expired_cert": {
      "anyOf": [{"type": "string"}, { "type": "boolean"}]
    },
    "keep_alive_interval": {
      "type": "string"
    },
    "keep_alive_count_max": {
      "type": "number"
    },
    "local_auth": {
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
        "audit_events_uri": {
          "anyOf": [
            {"type": "string"},
            {"type": "array",
             "items": {
               "type": "string"
             }
            }
          ]
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
	Unmarshal(bytes []byte, opts ...MarshalOption) (ClusterConfig, error)
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
func (t *TeleportClusterConfigMarshaler) Unmarshal(bytes []byte, opts ...MarshalOption) (ClusterConfig, error) {
	var clusterConfig ClusterConfigV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &clusterConfig); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err = utils.UnmarshalWithSchema(GetClusterConfigSchema(""), &clusterConfig, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	err = clusterConfig.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		clusterConfig.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		clusterConfig.SetExpiry(cfg.Expires)
	}
	return &clusterConfig, nil
}

// Marshal marshals ClusterConfig to JSON.
func (t *TeleportClusterConfigMarshaler) Marshal(c ClusterConfig, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := c.(type) {
	case *ClusterConfigV3:
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
