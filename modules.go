package teleport

import (
	"time"
)

const (
	// Component indicates a component of teleport, used for logging
	Component = "component"
	// ComponentFields stores component-specific fields
	ComponentFields = "fields"
	// ComponentReverseTunnel is reverse tunnel agent and server
	// that together establish a bi-directional SSH revers tunnel
	// to bypass firewall restrictions
	ComponentReverseTunnel = "reversetunnel"
	// ComponentNode is SSH node (SSH server serving requests)
	ComponentNode = "node"
	// ComponentProxy is SSH proxy (SSH server forwarding connections)
	ComponentProxy = "proxy"
	// ComponentTunClient is a tunnel client
	ComponentTunClient = "tunclient"
)

const (
	// DefaultServerTimeout sets read and wrie timeouts for SSH server ops
	DefaultServerTimeout time.Duration = 30 * time.Second
)
