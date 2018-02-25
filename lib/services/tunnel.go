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

// ReverseTunnelV2 is version 1 resource spec of the reverse tunnel
type ReverseTunnelV2 struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains user specification
	Spec ReverseTunnelSpecV2 `json:"spec"`
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

// ReverseTunnelSpecV2 is a specification for V2 reverse tunnel
type ReverseTunnelSpecV2 struct {
	// ClusterName is a domain name of remote cluster we are connecting to
	ClusterName string `json:"cluster_name"`
	// DialAddrs is a list of remote address to establish a connection to
	// it's always SSH over TCP
	DialAddrs []string `json:"dial_addrs,omitempty"`
}

// ReverseTunnelSpecV2Schema is JSON schema for reverse tunnel spec
const ReverseTunnelSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["cluster_name", "dial_addrs"],
  "properties": {
    "cluster_name": {"type": "string"},
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
func UnmarshalReverseTunnel(data []byte) (ReverseTunnel, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing tunnel data")
	}
	var h ResourceHeader
	err := json.Unmarshal(data, &h)
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
		return r.V2(), nil
	case V2:
		var r ReverseTunnelV2

		if err := utils.UnmarshalWithSchema(GetReverseTunnelSchema(), &r, data); err != nil {
			return nil, trace.BadParameter(err.Error())
		}

		if err := r.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
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
	UnmarshalReverseTunnel(bytes []byte) (ReverseTunnel, error)
	// MarshalReverseTunnel marshals reverse tunnel to binary representation
	MarshalReverseTunnel(ReverseTunnel, ...MarshalOption) ([]byte, error)
}

type TeleportTunnelMarshaler struct{}

// UnmarshalReverseTunnel unmarshals reverse tunnel from JSON or YAML
func (*TeleportTunnelMarshaler) UnmarshalReverseTunnel(bytes []byte) (ReverseTunnel, error) {
	return UnmarshalReverseTunnel(bytes)
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
		return json.Marshal(v.V2())
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}
