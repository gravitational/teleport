package services

// ReverseTunnel is SSH reverse tunnel established between a local Proxy
// and a remote Proxy. It helps to bypass firewall restrictions, so local
// clusters don't need to have the cluster involved
type ReverseTunnel interface {
	// GetClusterName returns name of the cluster
	GetClusterName() string
	// GetDialAddrs returns list of dial addresses for this cluster
	GetDialAddrs()
	// Check checks tunnel for errors
	Check() error
}

// ReverseTunnelV1 is version 1 resource spec of the reverse tunnel
type ReverseTunnelV1 struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains user specification
	Spec ReverseTunnelSpecV1 `json:"spec"`
}

// Check returns nil if all parameters are good, error otherwise
func (r *ReverseTunnelV1) Check() error {
	if strings.TrimSpace(r.DomainName) == "" {
		return trace.BadParameter("Reverse tunnel validation error: empty domain name")
	}

	if len(r.DialAddrs) == 0 {
		return trace.BadParameter("Invalid dial address for reverse tunnel '%v'", r.DomainName)
	}

	for _, addr := range r.DialAddrs {
		_, err := utils.ParseAddr(addr)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// ReverseTunnelSpecV1 is a specification for V1 reverse tunnel
type ReverseTunnelSpecV1 struct {
	// ClusterName is a domain name of remote cluster we are connecting to
	ClusterName string `json:"cluster_name"`
	// DialAddrs is a list of remote address to establish a connection to
	// it's always SSH over TCP
	DialAddrs []string `json:"dial_addrs"`
}

// ReverseTunnelSpecV1Schema is JSON schema for reverse tunnel spec
const ReverseTunnelSpecV1Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["cluster_name", "dial_addrs"],
  "properties": {
    "cluster_name": {"type": "string"},
    "dial_addrs": {
      "type": "array"
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

// V1 returns V1 version of reverse tunnel
func (r *ReverseTunnelV0) V1() *ReverseTunnelV1 {
	return &ReverseTunnelV1{
		Kind:    KindReverseTunnel,
		Version: V1,
		Metadata: Metadata{
			Name: r.DomainName,
		},
		Spec: UserSpecV1{
			ClusterName: r.DomainName,
			DialAddrs:   r.DialAddrs,
		},
	}
}
