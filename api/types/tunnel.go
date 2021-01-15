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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// ReverseTunnel is SSH reverse tunnel established between a local Proxy
// and a remote Proxy. It helps to bypass firewall restrictions, so local
// clusters don't need to have the cluster involved
type ReverseTunnel interface {
	// Resource provides common methods for resource objects
	Resource
	// GetClusterName returns name of the cluster
	GetClusterName() string
	// SetClusterName sets cluster name
	SetClusterName(name string)
	// GetType gets the type of ReverseTunnel.
	GetType() TunnelType
	// SetType sets the type of ReverseTunnel.
	SetType(TunnelType)
	// GetDialAddrs returns list of dial addresses for this cluster
	GetDialAddrs() []string
	// Check checks tunnel for errors
	Check() error
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
}

// NewReverseTunnel returns new version of reverse tunnel
func NewReverseTunnel(clusterName string, dialAddrs []string) ReverseTunnel {
	return &ReverseTunnelV2{
		Kind:    KindReverseTunnel,
		Version: V2,
		Metadata: Metadata{
			Name:      clusterName,
			Namespace: defaults.Namespace,
		},
		Spec: ReverseTunnelSpecV2{
			ClusterName: clusterName,
			DialAddrs:   dialAddrs,
		},
	}
}

// GetVersion returns resource version
func (r *ReverseTunnelV2) GetVersion() string {
	return r.Version
}

// GetKind returns resource kind
func (r *ReverseTunnelV2) GetKind() string {
	return r.Kind
}

// GetSubKind returns resource sub kind
func (r *ReverseTunnelV2) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *ReverseTunnelV2) SetSubKind(s string) {
	r.SubKind = s
}

// GetResourceID returns resource ID
func (r *ReverseTunnelV2) GetResourceID() int64 {
	return r.Metadata.ID
}

// SetResourceID sets resource ID
func (r *ReverseTunnelV2) SetResourceID(id int64) {
	r.Metadata.ID = id
}

// GetMetadata returns object metadata
func (r *ReverseTunnelV2) GetMetadata() Metadata {
	return r.Metadata
}

// SetExpiry sets expiry time for the object
func (r *ReverseTunnelV2) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (r *ReverseTunnelV2) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (r *ReverseTunnelV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// GetName returns the name of the User
func (r *ReverseTunnelV2) GetName() string {
	return r.Metadata.Name
}

// SetName sets the name of the User
func (r *ReverseTunnelV2) SetName(e string) {
	r.Metadata.Name = e
}

// CheckAndSetDefaults checks and sets defaults
func (r *ReverseTunnelV2) CheckAndSetDefaults() error {
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

// SetClusterName sets name of a cluster
func (r *ReverseTunnelV2) SetClusterName(name string) {
	r.Spec.ClusterName = name
}

// GetClusterName returns name of the cluster
func (r *ReverseTunnelV2) GetClusterName() string {
	return r.Spec.ClusterName
}

// GetType gets the type of ReverseTunnel.
func (r *ReverseTunnelV2) GetType() TunnelType {
	if string(r.Spec.Type) == "" {
		return ProxyTunnel
	}
	return r.Spec.Type
}

// SetType sets the type of ReverseTunnel.
func (r *ReverseTunnelV2) SetType(tt TunnelType) {
	r.Spec.Type = tt
}

// GetDialAddrs returns list of dial addresses for this cluster
func (r *ReverseTunnelV2) GetDialAddrs() []string {
	return r.Spec.DialAddrs
}

// Check returns nil if all parameters are good, error otherwise
func (r *ReverseTunnelV2) Check() error {
	if r.Version == "" {
		return trace.BadParameter("missing reverse tunnel version")
	}
	if strings.TrimSpace(r.Spec.ClusterName) == "" {
		return trace.BadParameter("Reverse tunnel validation error: empty cluster name")
	}

	if len(r.Spec.DialAddrs) == 0 {
		return trace.BadParameter("Invalid dial address for reverse tunnel '%v'", r.Spec.ClusterName)
	}

	for _, addr := range r.Spec.DialAddrs {
		if _, err := utils.ParseAddr(addr); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// TunnelType is the type of tunnel.
type TunnelType string

const (
	// NodeTunnel is a tunnel where the node connects to the proxy (dial back).
	NodeTunnel TunnelType = "node"

	// ProxyTunnel is a tunnel where a proxy connects to the proxy (trusted cluster).
	ProxyTunnel TunnelType = "proxy"

	// AppTunnel is a tunnel where the application proxy dials back to the proxy.
	AppTunnel TunnelType = "app"

	// KubeTunnel is a tunnel where the kubernetes service dials back to the proxy.
	KubeTunnel TunnelType = "kube"

	// DatabaseTunnel is a tunnel where a database proxy dials back to the proxy.
	DatabaseTunnel TunnelType = "db"
)

// GetReverseTunnelSchema returns role schema with optionally injected
// schema for extensions
func GetReverseTunnelSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ReverseTunnelSpecV2Schema, DefaultDefinitions)
}

// ReverseTunnelSpecV2Schema is JSON schema for reverse tunnel spec
const ReverseTunnelSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["cluster_name", "dial_addrs"],
  "properties": {
    "cluster_name": {"type": "string"},
    "type": {"type": "string"},
    "dial_addrs": {
      "type": "array",
      "items": {
        "type": "string"
      }
    }
  }
}`

// UnmarshalReverseTunnel unmarshals reverse tunnel from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalReverseTunnel(data []byte, opts ...MarshalOption) (ReverseTunnel, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing tunnel data")
	}
	var h ResourceHeader
	err := json.Unmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case V2:
		var r ReverseTunnelV2
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetReverseTunnelSchema(), &r, data); err != nil {
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

// ReverseTunnelMarshaler implements marshal/unmarshal of reverse tunnel implementations
type ReverseTunnelMarshaler interface {
	// UnmarshalReverseTunnel unmarshals reverse tunnel from binary representation
	UnmarshalReverseTunnel(bytes []byte, opts ...MarshalOption) (ReverseTunnel, error)
	// MarshalReverseTunnel marshals reverse tunnel to binary representation
	MarshalReverseTunnel(ReverseTunnel, ...MarshalOption) ([]byte, error)
}

// teleportTunnelMarshaler implements ReverseTunnelMarshaler
type teleportTunnelMarshaler struct{}

// UnmarshalReverseTunnel unmarshals reverse tunnel from JSON or YAML
func (*teleportTunnelMarshaler) UnmarshalReverseTunnel(bytes []byte, opts ...MarshalOption) (ReverseTunnel, error) {
	return UnmarshalReverseTunnel(bytes, opts...)
}

// MarshalReverseTunnel marshalls role into JSON
func (*teleportTunnelMarshaler) MarshalReverseTunnel(rt ReverseTunnel, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch reverseTunnel := rt.(type) {
	case *ReverseTunnelV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *reverseTunnel
			copy.SetResourceID(0)
			reverseTunnel = &copy
		}
		return utils.FastMarshal(reverseTunnel)
	default:
		return nil, trace.BadParameter("unrecognized reversetunnel version %T", rt)
	}
}

var tunnelMarshaler ReverseTunnelMarshaler = &teleportTunnelMarshaler{}

// SetReverseTunnelMarshaler sets global ReverseTunnelMarshaler
func SetReverseTunnelMarshaler(m ReverseTunnelMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	tunnelMarshaler = m
}

// GetReverseTunnelMarshaler returns currently set ReverseTunnelMarshaler
func GetReverseTunnelMarshaler() ReverseTunnelMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return tunnelMarshaler
}
