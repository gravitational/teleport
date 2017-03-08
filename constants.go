package teleport

import (
	"time"
)

// ForeverTTL means that object TTL will not expire unless deleted
const ForeverTTL time.Duration = 0

const (
	// SSHAuthSock is the environment variable pointing to the
	// Unix socket the SSH agent is running on.
	SSHAuthSock = "SSH_AUTH_SOCK"
	// SSHAgentPID is the environment variable pointing to the agent
	// process ID
	SSHAgentPID = "SSH_AGENT_PID"
)

const (
	// TOTPValidityPeriod is the number of seconds a TOTP token is valid.
	TOTPValidityPeriod uint = 30

	// TOTPSkew adds that many periods before and after to the validity window.
	TOTPSkew uint = 1
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

	// ComponentAuth is the cluster CA node (auth server API)
	ComponentAuth = "auth"

	// ComponentNode is SSH node (SSH server serving requests)
	ComponentNode = "node"

	// ComponentProxy is SSH proxy (SSH server forwarding connections)
	ComponentProxy = "proxy"

	// ComponentTunClient is a tunnel client
	ComponentTunClient = "tunclient"

	// DebugEnvVar tells tests to use verbose debug output
	DebugEnvVar = "DEBUG"

	// VerboseLogEnvVar forces all logs to be verbose (down to DEBUG level)
	VerboseLogsEnvVar = "TELEPORT_DEBUG"

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

	// DataDirParameterName is the name of the data dir configuration parameter passed
	// to all backends during initialization
	DataDirParameterName = "data_dir"

	// SSH request type to keep the connection alive. A client and a server keep
	// pining each other with it:
	KeepAliveReqType = "keepalive@openssh.com"

	// OTP means One-time Password Algorithm for Two-Factor Authentication.
	OTP = "otp"

	// TOTP means Time-based One-time Password Algorithm. for Two-Factor Authentication.
	TOTP = "totp"

	// HOTP means HMAC-based One-time Password Algorithm.for Two-Factor Authentication.
	HOTP = "hotp"

	// U2F means Universal 2nd Factor.for Two-Factor Authentication.
	U2F = "u2f"

	// OFF means no second factor.for Two-Factor Authentication.
	OFF = "off"

	// Local means authentication will happen locally within the Teleport cluster.
	Local = "local"

	// OIDC means authentication will happen remotly using an OIDC connector.
	OIDC = "oidc"
)
