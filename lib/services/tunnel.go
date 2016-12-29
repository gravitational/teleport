package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// ReverseTunnel is SSH reverse tunnel established between a local Proxy
// and a remote Proxy. It helps to bypass firewall restrictions, so local
// clusters don't need to have the cluster involved
type ReverseTunnel interface {
	// GetName returns tunnel object name
	GetName() string
	// GetClusterName returns name of the cluster
	GetClusterName() string
	// GetDialAddrs returns list of dial addresses for this cluster
	GetDialAddrs() []string
	// Check checks tunnel for errors
	Check() error
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

// GetName returns tunnel object name
func (r *ReverseTunnelV2) GetName() string {
	return r.Metadata.Name
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
	DialAddrs []string `json:"dial_addrs"`
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

// ReverseTunnelV0 is V0 version of reverse tunnel
type ReverseTunnelV0 struct {
	// DomainName is a domain name of remote cluster we are connecting to
	DomainName string `json:"domain_name"`
	// DialAddrs is a list of remote address to establish a connection to
	// it's always SSH over TCP
	DialAddrs []string `json:"dial_addrs"`
}

// V2 returns V2 version of reverse tunnel
func (r *ReverseTunnelV0) V2() *ReverseTunnelV2 {
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
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ReverseTunnelSpecV2Schema)
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
		var r ReverseTunnelV0
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
	MarshalReverseTunnel(ReverseTunnel) ([]byte, error)
}

type TeleportTunnelMarshaler struct{}

// UnmarshalReverseTunnel unmarshals reverse tunnel from JSON or YAML
func (*TeleportTunnelMarshaler) UnmarshalReverseTunnel(bytes []byte) (ReverseTunnel, error) {
	return UnmarshalReverseTunnel(bytes)
}

// MarshalRole marshalls role into JSON
func (*TeleportTunnelMarshaler) MarshalReverseTunnel(rt ReverseTunnel) ([]byte, error) {
	return json.Marshal(rt)
}
