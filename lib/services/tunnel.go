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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
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
func (o *ReverseTunnelV2) SetSubKind(s string) {
	o.SubKind = s
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

// Expires returns object expiry setting
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

// V2 returns V2 version of the resource
func (r *ReverseTunnelV2) V2() *ReverseTunnelV2 {
	return r
}

// V1 returns V1 version of the resource
func (r *ReverseTunnelV2) V1() *ReverseTunnelV1 {
	return &ReverseTunnelV1{
		DomainName: r.Spec.ClusterName,
		DialAddrs:  r.Spec.DialAddrs,
	}
}

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
		_, err := utils.ParseAddr(addr)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
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

// ReverseTunnelV1 is V1 version of reverse tunnel
type ReverseTunnelV1 struct {
	// DomainName is a domain name of remote cluster we are connecting to
	DomainName string `json:"domain_name"`
	// DialAddrs is a list of remote address to establish a connection to
	// it's always SSH over TCP
	DialAddrs []string `json:"dial_addrs"`
}

// V1 returns V1 version of the resource
func (r *ReverseTunnelV1) V1() *ReverseTunnelV1 {
	return r
}

// V2 returns V2 version of reverse tunnel
func (r *ReverseTunnelV1) V2() *ReverseTunnelV2 {
	return &ReverseTunnelV2{
		Kind:    KindReverseTunnel,
		Version: V2,
		Metadata: Metadata{
			Name:      r.DomainName,
			Namespace: defaults.Namespace,
		},
		Spec: ReverseTunnelSpecV2{
			ClusterName: r.DomainName,
			Type:        ProxyTunnel,
			DialAddrs:   r.DialAddrs,
		},
	}
}

// GetReverseTunnelSchema returns role schema with optionally injected
// schema for extensions
func GetReverseTunnelSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ReverseTunnelSpecV2Schema, DefaultDefinitions)
}

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
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case "":
		var r ReverseTunnelV1
		err := json.Unmarshal(data, &r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		v2 := r.V2()
		if cfg.ID != 0 {
			v2.SetResourceID(cfg.ID)
		}
		return r.V2(), nil
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

var tunnelMarshaler ReverseTunnelMarshaler = &TeleportTunnelMarshaler{}

func SetReerseTunnelMarshaler(m ReverseTunnelMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	tunnelMarshaler = m
}

func GetReverseTunnelMarshaler() ReverseTunnelMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return tunnelMarshaler
}

// ReverseTunnelMarshaler implements marshal/unmarshal of reverse tunnel implementations
type ReverseTunnelMarshaler interface {
	// UnmarshalReverseTunnel unmarshals reverse tunnel from binary representation
	UnmarshalReverseTunnel(bytes []byte, opts ...MarshalOption) (ReverseTunnel, error)
	// MarshalReverseTunnel marshals reverse tunnel to binary representation
	MarshalReverseTunnel(ReverseTunnel, ...MarshalOption) ([]byte, error)
}

type TeleportTunnelMarshaler struct{}

// UnmarshalReverseTunnel unmarshals reverse tunnel from JSON or YAML
func (*TeleportTunnelMarshaler) UnmarshalReverseTunnel(bytes []byte, opts ...MarshalOption) (ReverseTunnel, error) {
	return UnmarshalReverseTunnel(bytes, opts...)
}

// MarshalRole marshalls role into JSON
func (*TeleportTunnelMarshaler) MarshalReverseTunnel(rt ReverseTunnel, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type tunv1 interface {
		V1() *ReverseTunnelV1
	}
	type tunv2 interface {
		V2() *ReverseTunnelV2
	}
	version := cfg.GetVersion()
	switch version {
	case V1:
		v, ok := rt.(tunv1)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V1)
		}
		return json.Marshal(v.V1())
	case V2:
		v, ok := rt.(tunv2)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V2)
		}
		v2 := v.V2()
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *v2
			copy.SetResourceID(0)
			v2 = &copy
		}
		return utils.FastMarshal(v2)
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}

const (
	// NodeTunnel is a tunnel where the node connects to the proxy (dial back).
	NodeTunnel TunnelType = "node"

	// ProxyTunnel is a tunnel where a proxy connects to the proxy (trusted cluster).
	ProxyTunnel TunnelType = "proxy"
)

// TunnelType is the type of tunnel. Either node or proxy.
type TunnelType string
