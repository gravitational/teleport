package teleport

import (
	"time"
)

// ForeverTTL means that object TTL will not expire unless deleted
const ForeverTTL time.Duration = 0

const (
	// BoltBackendType is a BoltDB backend
	BoltBackendType = "bolt"

	// ETCDBackendType is etcd backend
	ETCDBackendType = "etcd"

	// Component indicates a component of teleport, used for logging
	Component = "component"

	// ComponentFields stores component-specific fields
	ComponentFields = "fields"

	// ComponentReverseTunnel is reverse tunnel agent and server
	// that together establish a bi-directional SSH revers tunnel
	// to bypass firewall restrictions
	ComponentReverseTunnel = "reversetunnel"

	// ComponentAuth is the cluster CA node (auth server API)
	ComponentAuth = "auth"

	// ComponentNode is SSH node (SSH server serving requests)
	ComponentNode = "node"

	// ComponentProxy is SSH proxy (SSH server forwarding connections)
	ComponentProxy = "proxy"

	// ComponentTunClient is a tunnel client
	ComponentTunClient = "tunclient"

	// DefaultTimeout sets read and wrie timeouts for SSH server ops
	DefaultTimeout time.Duration = 30 * time.Second

	// DebugOutputEnvVar tells tests to use verbose debug output
	DebugOutputEnvVar = "TELEPORT_DEBUG"

	// DefaultTerminalWidth defines the default width of a server-side allocated
	// pseudo TTY
	DefaultTerminalWidth = 80

	// DefaultTerminalHeight defines the default height of a server-side allocated
	// pseudo TTY
	DefaultTerminalHeight = 25

	// SafeTerminalType is the fall-back TTY type to fall back to (when $TERM
	// is not defined)
	SafeTerminalType = "xterm"

	// ConnectorOIDC means connector type OIDC
	ConnectorOIDC = "oidc"
)
