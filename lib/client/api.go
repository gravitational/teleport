/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/net/http2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/touchid"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/authz"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/devicetrust"
	dtauthn "github.com/gravitational/teleport/lib/devicetrust/authn"
	dtenroll "github.com/gravitational/teleport/lib/devicetrust/enroll"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/shell"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/agentconn"
)

const (
	AddKeysToAgentAuto = "auto"
	AddKeysToAgentNo   = "no"
	AddKeysToAgentYes  = "yes"
	AddKeysToAgentOnly = "only"
)

var AllAddKeysOptions = []string{AddKeysToAgentAuto, AddKeysToAgentNo, AddKeysToAgentYes, AddKeysToAgentOnly}

// ValidateAgentKeyOption validates that a string is a valid option for the AddKeysToAgent parameter.
func ValidateAgentKeyOption(supplied string) error {
	for _, option := range AllAddKeysOptions {
		if supplied == option {
			return nil
		}
	}

	return trace.BadParameter("invalid value %q, must be one of %v", supplied, AllAddKeysOptions)
}

// AgentForwardingMode  describes how the user key agent will be forwarded
// to a remote machine, if at all.
type AgentForwardingMode int

const (
	ForwardAgentNo AgentForwardingMode = iota
	ForwardAgentYes
	ForwardAgentLocal
)

const remoteForwardUnsupportedMessage = "ssh: tcpip-forward request denied by peer"

var log = logrus.WithFields(logrus.Fields{
	teleport.ComponentKey: teleport.ComponentClient,
})

// ForwardedPort specifies local tunnel to remote
// destination managed by the client, is equivalent
// of ssh -L src:host:dst command
type ForwardedPort struct {
	SrcIP    string
	SrcPort  int
	DestPort int
	DestHost string
}

// ForwardedPorts contains an array of forwarded port structs
type ForwardedPorts []ForwardedPort

// ToString returns a string representation of a forwarded port spec, compatible
// with OpenSSH's -L  flag, i.e. "src_host:src_port:dest_host:dest_port".
func (p *ForwardedPort) ToString() string {
	sport := strconv.Itoa(p.SrcPort)
	dport := strconv.Itoa(p.DestPort)
	if utils.IsLocalhost(p.SrcIP) {
		return sport + ":" + net.JoinHostPort(p.DestHost, dport)
	}
	return net.JoinHostPort(p.SrcIP, sport) + ":" + net.JoinHostPort(p.DestHost, dport)
}

// DynamicForwardedPort local port for dynamic application-level port
// forwarding. Whenever a connection is made to this port, SOCKS5 protocol
// is used to determine the address of the remote host. More or less
// equivalent to OpenSSH's -D flag.
type DynamicForwardedPort struct {
	// SrcIP is the IP address to listen on locally.
	SrcIP string

	// SrcPort is the port to listen on locally.
	SrcPort int
}

// DynamicForwardedPorts is a slice of locally forwarded dynamic ports (SOCKS5).
type DynamicForwardedPorts []DynamicForwardedPort

// ToString returns a string representation of a dynamic port spec, compatible
// with OpenSSH's -D flag, i.e. "src_host:src_port".
func (p *DynamicForwardedPort) ToString() string {
	sport := strconv.Itoa(p.SrcPort)
	if utils.IsLocalhost(p.SrcIP) {
		return sport
	}
	return net.JoinHostPort(p.SrcIP, sport)
}

// HostKeyCallback is called by SSH client when it needs to check
// remote host key or certificate validity
type HostKeyCallback func(host string, ip net.Addr, key ssh.PublicKey) error

// Config is a client config
type Config struct {
	// Username is the Teleport account username (for logging into Teleport proxies)
	Username string
	// ExplicitUsername is true if Username was initially set by the end-user
	// (for example, using command-line flags).
	ExplicitUsername bool

	// Remote host to connect
	Host string

	// SearchKeywords host to connect
	SearchKeywords []string

	// PredicateExpression host to connect
	PredicateExpression string

	// UseSearchAsRoles modifies the behavior of resource loading to
	// use search as roles feature.
	UseSearchAsRoles bool

	// Labels represent host Labels
	Labels map[string]string

	// Namespace is nodes namespace
	Namespace string

	// HostLogin is a user login on a remote host
	HostLogin string

	// HostPort is a remote host port to connect to. This is used for **explicit**
	// port setting via -p flag, otherwise '0' is passed which means "use server default"
	HostPort int

	// JumpHosts if specified are interpreted in a similar way
	// as -J flag in ssh - used to dial through
	JumpHosts []utils.JumpHost

	// WebProxyAddr is the host:port the web proxy can be accessed at.
	WebProxyAddr string

	// SSHProxyAddr is the host:port the SSH proxy can be accessed at.
	SSHProxyAddr string

	// KubeProxyAddr is the host:port the Kubernetes proxy can be accessed at.
	KubeProxyAddr string

	// PostgresProxyAddr is the host:port the Postgres proxy can be accessed at.
	PostgresProxyAddr string

	// MongoProxyAddr is the host:port the Mongo proxy can be accessed at.
	MongoProxyAddr string

	// MySQLProxyAddr is the host:port the MySQL proxy can be accessed at.
	MySQLProxyAddr string

	// KeyTTL is a time to live for the temporary SSH keypair to remain valid:
	KeyTTL time.Duration

	// InsecureSkipVerify is an option to skip HTTPS cert check
	InsecureSkipVerify bool

	// NonInteractive tells the client not to trigger interactive features for non-interactive commands,
	// such as prompting user to re-login on credential errors. This is used by external programs linking
	// against Teleport client and obtaining credentials from elsewhere. e.g. from an identity file.
	NonInteractive bool

	// Agent is an SSH agent to use for local Agent procedures. Defaults to in-memory agent keyring.
	Agent agent.ExtendedAgent

	ClientStore *Store

	// ForwardAgent is used by the client to request agent forwarding from the server.
	ForwardAgent AgentForwardingMode

	// EnableX11Forwarding specifies whether X11 forwarding should be enabled.
	EnableX11Forwarding bool

	// X11ForwardingTimeout can be set to set a X11 forwarding timeout in seconds,
	// after which any X11 forwarding requests in that session will be rejected.
	X11ForwardingTimeout time.Duration

	// X11ForwardingTrusted specifies the X11 forwarding security mode.
	X11ForwardingTrusted bool

	// AuthMethods are used to login into the cluster. If specified, the client will
	// use them in addition to certs stored in the client store.
	AuthMethods []ssh.AuthMethod

	// TLSConfig is TLS configuration, if specified, the client
	// will use this TLS configuration to access API endpoints
	TLS *tls.Config

	// ProxySSHPrincipal determines the SSH username (principal) the client should be using
	// when connecting to auth/proxy servers. By default, the SSH username is pulled from
	// the user's certificate, but the web-based terminal provides this username explicitly.
	ProxySSHPrincipal string

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	// ExitStatus carries the returned value (exit status) of the remote
	// process execution (via SSH exec)
	ExitStatus int

	// SiteName specifies site to execute operation,
	// if omitted, first available site will be selected
	SiteName string

	// KubernetesCluster specifies the kubernetes cluster for any relevant
	// operations. If empty, the auth server will choose one using stable (same
	// cluster every time) but unspecified logic.
	KubernetesCluster string

	// DatabaseService specifies name of the database proxy server to issue
	// certificate for.
	DatabaseService string

	// LocalForwardPorts are the local ports tsh listens on for port forwarding
	// (parameters to -L ssh flag).
	LocalForwardPorts ForwardedPorts

	// DynamicForwardedPorts are the list of ports tsh listens on for dynamic
	// port forwarding (parameters to -D ssh flag).
	DynamicForwardedPorts DynamicForwardedPorts

	// RemoteForwardPorts are the list of ports the remote connection listens on
	// for remote port forwarding (parameters to -R ssh flag).
	RemoteForwardPorts ForwardedPorts

	// HostKeyCallback will be called to check host keys of the remote
	// node, if not specified will be using CheckHostSignature function
	// that uses local cache to validate hosts
	HostKeyCallback ssh.HostKeyCallback

	// KeyDir defines where temporary session keys will be stored.
	// if empty, they'll go to ~/.tsh
	KeysDir string

	// SessionID is a session ID to use when opening a new session.
	SessionID string

	// extraEnvs contains additional environment variables that will be added
	// to SSH session.
	extraEnvs map[string]string

	// InteractiveCommand tells tsh to launch a remote exec command in interactive mode,
	// i.e. attaching the terminal to it.
	InteractiveCommand bool

	// ClientAddr (if set) specifies the true client IP. Usually it's not needed (since the server
	// can look at the connecting address to determine client's IP) but for cases when the
	// client is web-based, this must be set to HTTP's remote addr
	ClientAddr string

	// CachePolicy defines local caching policy in case if discovery goes down
	// by default does not use caching
	CachePolicy *CachePolicy

	// CertificateFormat is the format of the SSH certificate.
	CertificateFormat string

	// AuthConnector is the name of the authentication connector to use.
	AuthConnector string

	// AuthenticatorAttachment is the desired authenticator attachment.
	AuthenticatorAttachment wancli.AuthenticatorAttachment

	// PreferOTP prefers OTP in favor of other MFA methods.
	// Useful in constrained environments without access to USB or platform
	// authenticators, such as remote hosts or virtual machines.
	PreferOTP bool

	// CheckVersions will check that client version is compatible
	// with auth server version when connecting.
	CheckVersions bool

	// BindAddr is an optional host:port to bind to for SSO redirect flows.
	BindAddr string
	// CallbackAddr is the optional base URL to give to the user when performing
	// SSO redirect flows.
	CallbackAddr string

	// NoRemoteExec will not execute a remote command after connecting to a host,
	// will block instead. Useful when port forwarding. Equivalent of -N for OpenSSH.
	NoRemoteExec bool

	// Browser can be used to pass the name of a browser to override the system default
	// (not currently implemented), or set to 'none' to suppress browser opening entirely.
	Browser string

	// AddKeysToAgent specifies how the client handles keys.
	//	auto - will attempt to add keys to agent if the agent supports it
	//	only - attempt to load keys into agent but don't write them to disk
	//	on - attempt to load keys into agent
	//	off - do not attempt to load keys into agent
	AddKeysToAgent string

	// EnableEscapeSequences will scan Stdin for SSH escape sequences during
	// command/shell execution. This also requires Stdin to be an interactive
	// terminal.
	EnableEscapeSequences bool

	// MockSSOLogin is used in tests for mocking the SSO login response.
	MockSSOLogin SSOLoginFunc

	// MockHeadlessLogin is used in tests for mocking the Headless login response.
	MockHeadlessLogin SSHLoginFunc

	// OverrideMySQLOptionFilePath overrides the MySQL option file path to use.
	// Useful in parallel tests so they don't all use the default path in the
	// user home dir.
	OverrideMySQLOptionFilePath string

	// OverridePostgresServiceFilePath overrides the Postgres service file path.
	// Useful in parallel tests so they don't all use the default path in the
	// user home dir.
	OverridePostgresServiceFilePath string

	// HomePath is where tsh stores profiles
	HomePath string

	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool

	// TLSRoutingConnUpgradeRequired indicates that ALPN connection upgrades
	// are required for making TLS routing requests.
	//
	// Note that this is applicable to the Proxy's Web port regardless of
	// whether the Proxy is in single-port or multi-port configuration.
	TLSRoutingConnUpgradeRequired bool

	// Reason is a reason attached to started sessions meant to describe their intent.
	Reason string

	// Invited is a list of people invited to a session.
	Invited []string

	// DisplayParticipantRequirements is set if debug information about participants requirements
	// should be printed in moderated sessions.
	DisplayParticipantRequirements bool

	// ExtraProxyHeaders is a collection of http headers to be included in requests to the WebProxy.
	ExtraProxyHeaders map[string]string

	// AllowStdinHijack allows stdin hijack during MFA prompts.
	// Stdin hijack provides a better login UX, but it can be difficult to reason
	// about and is often a source of bugs.
	// Do not set this options unless you deeply understand what you are doing.
	AllowStdinHijack bool

	// Tracer is the tracer to create spans with
	Tracer oteltrace.Tracer

	// PrivateKeyPolicy is a key policy that this client will try to follow during login.
	PrivateKeyPolicy keys.PrivateKeyPolicy

	// PIVSlot specifies a specific PIV slot to use with hardware key support.
	PIVSlot keys.PIVSlot

	// LoadAllCAs indicates that tsh should load the CAs of all clusters
	// instead of just the current cluster.
	LoadAllCAs bool

	// AllowHeadless determines whether headless login can be used. Currently, only
	// the ssh, scp, and ls commands can use headless login. Other commands will ignore
	// headless auth connector and default to local instead.
	AllowHeadless bool

	// DialOpts used by the api.client.proxy.Client when establishing a connection to
	// the proxy server. Used by tests.
	DialOpts []grpc.DialOption

	// PROXYSigner is used to sign PROXY headers for securely propagating client IP address
	PROXYSigner multiplexer.PROXYHeaderSigner

	// DTAuthnRunCeremony allows tests to override the default device
	// authentication function.
	// Defaults to "dtauthn.NewCeremony().Run()".
	DTAuthnRunCeremony DTAuthnRunCeremonyFunc

	// dtAttemptLoginIgnorePing and dtAutoEnrollIgnorePing allow Device Trust
	// tests to ignore Ping responses.
	// Useful to force flows that only typically happen on Teleport Enterprise.
	dtAttemptLoginIgnorePing, dtAutoEnrollIgnorePing bool

	// dtAutoEnroll allows tests to override the default device auto-enroll
	// function.
	// Defaults to [dtenroll.AutoEnroll].
	dtAutoEnroll dtAutoEnrollFunc

	// WebauthnLogin allows tests to override the Webauthn Login func.
	// Defaults to [wancli.Login].
	WebauthnLogin WebauthnLoginFunc

	// SSHLogDir is the directory to log the output of multiple SSH commands to.
	// If not set, no logs will be created.
	SSHLogDir string

	// MFAPromptConstructor is a custom MFA prompt constructor to use when prompting for MFA.
	MFAPromptConstructor func(cfg *libmfa.PromptConfig) mfa.Prompt

	// DisableSSHResumption disables transparent SSH connection resumption.
	DisableSSHResumption bool
}

// CachePolicy defines cache policy for local clients
type CachePolicy struct {
	// CacheTTL defines cache TTL
	CacheTTL time.Duration
	// NeverExpire never expires local cache information
	NeverExpires bool
}

// MakeDefaultConfig returns default client config
func MakeDefaultConfig() *Config {
	return &Config{
		Stdout:                os.Stdout,
		Stderr:                os.Stderr,
		Stdin:                 os.Stdin,
		AddKeysToAgent:        AddKeysToAgentAuto,
		EnableEscapeSequences: true,
		Tracer:                tracing.NoopProvider().Tracer("TeleportClient"),
	}
}

// VirtualPathKind is the suffix component for env vars denoting the type of
// file that will be loaded.
type VirtualPathKind string

const (
	// VirtualPathEnvPrefix is the env var name prefix shared by all virtual
	// path vars.
	VirtualPathEnvPrefix = "TSH_VIRTUAL_PATH"

	VirtualPathKey        VirtualPathKind = "KEY"
	VirtualPathCA         VirtualPathKind = "CA"
	VirtualPathDatabase   VirtualPathKind = "DB"
	VirtualPathApp        VirtualPathKind = "APP"
	VirtualPathKubernetes VirtualPathKind = "KUBE"
)

// VirtualPathParams are an ordered list of additional optional parameters
// for a virtual path. They can be used to specify a more exact resource name
// if multiple might be available. Simpler integrations can instead only
// specify the kind and it will apply wherever a more specific env var isn't
// found.
type VirtualPathParams []string

// VirtualPathCAParams returns parameters for selecting CA certificates.
func VirtualPathCAParams(caType types.CertAuthType) VirtualPathParams {
	return VirtualPathParams{
		strings.ToUpper(string(caType)),
	}
}

// VirtualPathDatabaseParams returns parameters for selecting specific database
// certificates.
func VirtualPathDatabaseParams(databaseName string) VirtualPathParams {
	return VirtualPathParams{databaseName}
}

// VirtualPathAppParams returns parameters for selecting specific apps by name.
func VirtualPathAppParams(appName string) VirtualPathParams {
	return VirtualPathParams{appName}
}

// VirtualPathKubernetesParams returns parameters for selecting k8s clusters by
// name.
func VirtualPathKubernetesParams(k8sCluster string) VirtualPathParams {
	return VirtualPathParams{k8sCluster}
}

// VirtualPathEnvName formats a single virtual path environment variable name.
func VirtualPathEnvName(kind VirtualPathKind, params VirtualPathParams) string {
	components := append([]string{
		VirtualPathEnvPrefix,
		string(kind),
	}, params...)

	return strings.ToUpper(strings.Join(components, "_"))
}

// VirtualPathEnvNames determines an ordered list of environment variables that
// should be checked to resolve an env var override. Params may be nil to
// indicate no additional arguments are to be specified or accepted.
func VirtualPathEnvNames(kind VirtualPathKind, params VirtualPathParams) []string {
	// Bail out early if there are no parameters.
	if len(params) == 0 {
		return []string{VirtualPathEnvName(kind, VirtualPathParams{})}
	}

	var vars []string
	for i := len(params); i >= 0; i-- {
		vars = append(vars, VirtualPathEnvName(kind, params[0:i]))
	}

	return vars
}

// RetryWithRelogin is a helper error handling method, attempts to relogin and
// retry the function once.
func RetryWithRelogin(ctx context.Context, tc *TeleportClient, fn func() error, opts ...RetryWithReloginOption) error {
	fnErr := fn()
	switch {
	case fnErr == nil:
		return nil
	case utils.IsPredicateError(fnErr):
		return trace.Wrap(utils.PredicateError{Err: fnErr})
	case tc.NonInteractive:
		return trace.Wrap(fnErr)
	case !IsErrorResolvableWithRelogin(fnErr):
		return trace.Wrap(fnErr)
	}
	opt := retryWithReloginOptions{}
	for _, o := range opts {
		o(&opt)
	}
	log.Debugf("Activating relogin on error=%q (type=%T)", fnErr, trace.Unwrap(fnErr))

	if keys.IsPrivateKeyPolicyError(fnErr) {
		privateKeyPolicy, err := keys.ParsePrivateKeyPolicyError(fnErr)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := tc.updatePrivateKeyPolicy(privateKeyPolicy); err != nil {
			return trace.Wrap(err)
		}
	}

	if opt.beforeLoginHook != nil {
		if err := opt.beforeLoginHook(); err != nil {
			return trace.Wrap(err)
		}
	}
	key, err := tc.Login(ctx)
	if err != nil {
		if errors.Is(err, prompt.ErrNotTerminal) {
			log.WithError(err).Debugf("Relogin is not available in this environment")
			return trace.Wrap(fnErr)
		}
		if trace.IsTrustError(err) {
			return trace.Wrap(err, "refusing to connect to untrusted proxy %v without --insecure flag\n", tc.SSHProxyAddr)
		}
		return trace.Wrap(err)
	}

	proxyClient, rootAuthClient, err := tc.ConnectToRootCluster(ctx, key)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		rootAuthClient.Close()
		proxyClient.Close()
	}()

	// Attempt device login. This activates a fresh key if successful.
	if err := tc.AttemptDeviceLogin(ctx, key, rootAuthClient); err != nil {
		return trace.Wrap(err)
	}

	// Save profile to record proxy credentials
	if err := tc.SaveProfile(true); err != nil {
		log.Warningf("Failed to save profile: %v", err)
		return trace.Wrap(err)
	}

	return fn()
}

// RetryWithReloginOption is a functional option for configuring the
// RetryWithRelogin helper.
type RetryWithReloginOption func(*retryWithReloginOptions)

// retryWithReloginOptions is a struct for configuring the RetryWithRelogin
type retryWithReloginOptions struct {
	// beforeLoginHook is a function that will be called before the login attempt.
	beforeLoginHook func() error
}

// WithBeforeLogin is a functional option for configuring a function that will
// be called before the login attempt.
func WithBeforeLoginHook(fn func() error) RetryWithReloginOption {
	return func(o *retryWithReloginOptions) {
		o.beforeLoginHook = fn
	}
}

// IsErrorResolvableWithRelogin returns true if relogin is attempted on `err`.
func IsErrorResolvableWithRelogin(err error) bool {
	// Private key policy errors indicate that the user must login with an
	// unexpected private key policy requirement satisfied. This can occur
	// in the following cases:
	// - User is logging in for the first time, and their strictest private
	//   key policy requirement is specified in a role.
	// - User is assuming a role with a stricter private key policy requirement
	//   than the user's given roles.
	// - The private key policy in the user's roles or the cluster auth
	//   preference have been upgraded since the user last logged in, making
	//   their current login session invalid.
	if keys.IsPrivateKeyPolicyError(err) {
		return true
	}

	// Ignore any failures resulting from RPCs.
	// These were all materialized as status.Error here before
	// https://github.com/gravitational/teleport/pull/30578.
	var remoteErr *interceptors.RemoteError
	if errors.As(err, &remoteErr) {
		// Exception for the two "retryable" errors that come from RPCs.
		//
		// Since Connect no longer checks the user cert before making an RPC,
		// it has to be able to properly recognize "expired certs" errors
		// that come from the server (to show a re-login dialog).
		//
		// TODO(gzdunek): These manual checks should be replaced with retryable
		// errors returned explicitly, as described below by codingllama.
		isClientCredentialsHaveExpired := errors.Is(err, client.ErrClientCredentialsHaveExpired)
		isTLSExpiredCertificate := strings.Contains(err.Error(), "tls: expired certificate")
		return isClientCredentialsHaveExpired || isTLSExpiredCertificate
	}

	// TODO(codingllama): Retrying BadParameter is a terrible idea.
	//  We should fix this and remove the RemoteError condition above as well.
	//  Any retriable error should be explicitly marked as such.
	return trace.IsBadParameter(err) ||
		trace.IsTrustError(err) ||
		utils.IsCertExpiredError(err) ||
		// Assume that failed handshake is a result of expired credentials.
		utils.IsHandshakeFailedError(err) ||
		IsNoCredentialsError(err)
}

// GetProfile gets the profile for the specified proxy address, or
// the current profile if no proxy is specified.
func (c *Config) GetProfile(ps ProfileStore, proxyAddr string) (*profile.Profile, error) {
	var proxyHost string
	var err error
	if proxyAddr == "" {
		proxyHost, err = ps.CurrentProfile()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		proxyHost, err = utils.Host(proxyAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	profile, err := ps.GetProfile(proxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return profile, nil
}

// LoadProfile populates Config with the values stored in the given
// profiles directory. If profileDir is an empty string, the default profile
// directory ~/.tsh is used.
func (c *Config) LoadProfile(ps ProfileStore, proxyAddr string) error {
	profile, err := c.GetProfile(ps, proxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	c.Username = profile.Username
	c.SiteName = profile.SiteName
	c.KubeProxyAddr = profile.KubeProxyAddr
	c.WebProxyAddr = profile.WebProxyAddr
	c.SSHProxyAddr = profile.SSHProxyAddr
	c.PostgresProxyAddr = profile.PostgresProxyAddr
	c.MySQLProxyAddr = profile.MySQLProxyAddr
	c.MongoProxyAddr = profile.MongoProxyAddr
	c.TLSRoutingEnabled = profile.TLSRoutingEnabled
	c.TLSRoutingConnUpgradeRequired = profile.TLSRoutingConnUpgradeRequired
	c.KeysDir = profile.Dir
	c.AuthConnector = profile.AuthConnector
	c.LoadAllCAs = profile.LoadAllCAs
	c.PrivateKeyPolicy = profile.PrivateKeyPolicy
	c.PIVSlot = profile.PIVSlot
	c.AuthenticatorAttachment, err = parseMFAMode(profile.MFAMode)
	if err != nil {
		return trace.BadParameter("unable to parse mfa mode in user profile: %v.", err)
	}

	c.DynamicForwardedPorts, err = ParseDynamicPortForwardSpec(profile.DynamicForwardedPorts)
	if err != nil {
		log.Warnf("Unable to parse dynamic port forwarding in user profile: %v.", err)
	}

	if required, ok := client.OverwriteALPNConnUpgradeRequirementByEnv(c.WebProxyAddr); ok {
		c.TLSRoutingConnUpgradeRequired = required
	}
	log.Infof("ALPN connection upgrade required for %q: %v.", c.WebProxyAddr, c.TLSRoutingConnUpgradeRequired)
	return nil
}

// SaveProfile updates the given profiles directory with the current configuration
// If profileDir is an empty string, the default ~/.tsh is used
func (c *Config) SaveProfile(makeCurrent bool) error {
	if c.WebProxyAddr == "" {
		return nil
	}

	if err := c.ClientStore.SaveProfile(c.Profile(), makeCurrent); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Profile converts Config to *profile.Profile.
func (c *Config) Profile() *profile.Profile {
	return &profile.Profile{
		Username:                      c.Username,
		WebProxyAddr:                  c.WebProxyAddr,
		SSHProxyAddr:                  c.SSHProxyAddr,
		KubeProxyAddr:                 c.KubeProxyAddr,
		PostgresProxyAddr:             c.PostgresProxyAddr,
		MySQLProxyAddr:                c.MySQLProxyAddr,
		MongoProxyAddr:                c.MongoProxyAddr,
		SiteName:                      c.SiteName,
		TLSRoutingEnabled:             c.TLSRoutingEnabled,
		TLSRoutingConnUpgradeRequired: c.TLSRoutingConnUpgradeRequired,
		AuthConnector:                 c.AuthConnector,
		MFAMode:                       c.AuthenticatorAttachment.String(),
		LoadAllCAs:                    c.LoadAllCAs,
		PrivateKeyPolicy:              c.PrivateKeyPolicy,
		PIVSlot:                       c.PIVSlot,
	}
}

// ParsedProxyHost holds the hostname and Web & SSH proxy addresses
// parsed out of a WebProxyAddress string.
type ParsedProxyHost struct {
	Host string

	// UsingDefaultWebProxyPort means that the port in WebProxyAddr was
	// supplied by ParseProxyHost function rather than ProxyHost string
	// itself.
	UsingDefaultWebProxyPort bool
	WebProxyAddr             string
	SSHProxyAddr             string
}

// ParseProxyHost parses a ProxyHost string of the format <hostname>:<proxy_web_port>,<proxy_ssh_port>
// and returns the parsed components.
//
// There are several "default" ports that the Web Proxy service may use, and if the port is not
// specified in the supplied proxyHost string
//
// If a definitive answer is not possible (e.g.  no proxy port is specified in
// the supplied string), ParseProxyHost() will supply default versions and flag
// that a default value is being used in the returned `ParsedProxyHost`
func ParseProxyHost(proxyHost string) (*ParsedProxyHost, error) {
	host, port, err := net.SplitHostPort(proxyHost)
	if err != nil {
		host = proxyHost
		port = ""
	}

	// set the default values of the port strings. One, both, or neither may
	// be overridden by the port string parsing below.
	usingDefaultWebProxyPort := true
	webPort := strconv.Itoa(defaults.HTTPListenPort)
	sshPort := strconv.Itoa(defaults.SSHProxyListenPort)

	// Split the port string out into at most two parts, the proxy port and
	// ssh port. Any more that 2 parts will be considered an error.
	parts := strings.Split(port, ",")

	switch {
	// Default ports for both the SSH and Web proxy.
	case len(parts) == 0:
		break

	// User defined HTTP proxy port, default SSH proxy port.
	case len(parts) == 1:
		if text := strings.TrimSpace(parts[0]); len(text) > 0 {
			webPort = text
			usingDefaultWebProxyPort = false
		}

	// User defined HTTP and SSH proxy ports.
	case len(parts) == 2:
		if text := strings.TrimSpace(parts[0]); len(text) > 0 {
			webPort = text
			usingDefaultWebProxyPort = false
		}
		if text := strings.TrimSpace(parts[1]); len(text) > 0 {
			sshPort = text
		}

	default:
		return nil, trace.BadParameter("unable to parse port: %v", port)
	}

	result := &ParsedProxyHost{
		Host:                     host,
		UsingDefaultWebProxyPort: usingDefaultWebProxyPort,
		WebProxyAddr:             net.JoinHostPort(host, webPort),
		SSHProxyAddr:             net.JoinHostPort(host, sshPort),
	}
	return result, nil
}

// ParseProxyHost parses the proxyHost string and updates the config.
//
// Format of proxyHost string:
//
//	proxy_web_addr:<proxy_web_port>,<proxy_ssh_port>
func (c *Config) ParseProxyHost(proxyHost string) error {
	parsedAddrs, err := ParseProxyHost(proxyHost)
	if err != nil {
		return trace.Wrap(err)
	}
	c.WebProxyAddr = parsedAddrs.WebProxyAddr
	c.SSHProxyAddr = parsedAddrs.SSHProxyAddr
	return nil
}

// KubeProxyHostPort returns the host and port of the Kubernetes proxy.
func (c *Config) KubeProxyHostPort() (string, int) {
	if c.KubeProxyAddr != "" {
		addr, err := utils.ParseAddr(c.KubeProxyAddr)
		if err == nil {
			return addr.Host(), addr.Port(defaults.KubeListenPort)
		}
	}

	webProxyHost, _ := c.WebProxyHostPort()
	return webProxyHost, defaults.KubeListenPort
}

// KubeClusterAddr returns a public HTTPS address of the proxy for use by
// Kubernetes client.
func (c *Config) KubeClusterAddr() string {
	host, port := c.KubeProxyHostPort()
	return fmt.Sprintf("https://%s:%d", host, port)
}

// WebProxyHostPort returns the host and port of the web proxy.
func (c *Config) WebProxyHostPort() (string, int) {
	if c.WebProxyAddr != "" {
		addr, err := utils.ParseAddr(c.WebProxyAddr)
		if err == nil {
			return addr.Host(), addr.Port(defaults.HTTPListenPort)
		}
	}
	return "unknown", defaults.HTTPListenPort
}

// WebProxyHost returns the web proxy host without the port number.
func (c *Config) WebProxyHost() string {
	host, _ := c.WebProxyHostPort()
	return host
}

// WebProxyPort returns the port of the web proxy.
func (c *Config) WebProxyPort() int {
	_, port := c.WebProxyHostPort()
	return port
}

// SSHProxyHostPort returns the host and port of the SSH proxy.
func (c *Config) SSHProxyHostPort() (string, int) {
	if c.SSHProxyAddr != "" {
		addr, err := utils.ParseAddr(c.SSHProxyAddr)
		if err == nil {
			return addr.Host(), addr.Port(defaults.SSHProxyListenPort)
		}
	}

	webProxyHost, _ := c.WebProxyHostPort()
	return webProxyHost, defaults.SSHProxyListenPort
}

// PostgresProxyHostPort returns the host and port of Postgres proxy.
func (c *Config) PostgresProxyHostPort() (string, int) {
	if c.PostgresProxyAddr != "" {
		addr, err := utils.ParseAddr(c.PostgresProxyAddr)
		if err == nil {
			return addr.Host(), addr.Port(c.WebProxyPort())
		}
	}
	return c.WebProxyHostPort()
}

// MongoProxyHostPort returns the host and port of Mongo proxy.
func (c *Config) MongoProxyHostPort() (string, int) {
	if c.MongoProxyAddr != "" {
		addr, err := utils.ParseAddr(c.MongoProxyAddr)
		if err == nil {
			return addr.Host(), addr.Port(defaults.MongoListenPort)
		}
	}
	return c.WebProxyHostPort()
}

// MySQLProxyHostPort returns the host and port of MySQL proxy.
func (c *Config) MySQLProxyHostPort() (string, int) {
	if c.MySQLProxyAddr != "" {
		addr, err := utils.ParseAddr(c.MySQLProxyAddr)
		if err == nil {
			return addr.Host(), addr.Port(defaults.MySQLListenPort)
		}
	}
	webProxyHost, _ := c.WebProxyHostPort()
	return webProxyHost, defaults.MySQLListenPort
}

// DatabaseProxyHostPort returns proxy connection endpoint for the database.
func (c *Config) DatabaseProxyHostPort(db tlsca.RouteToDatabase) (string, int) {
	switch db.Protocol {
	case defaults.ProtocolPostgres, defaults.ProtocolCockroachDB:
		return c.PostgresProxyHostPort()
	case defaults.ProtocolMySQL:
		return c.MySQLProxyHostPort()
	case defaults.ProtocolMongoDB:
		return c.MongoProxyHostPort()
	}
	return c.WebProxyHostPort()
}

// DoesDatabaseUseWebProxyHostPort returns true if database is using web port.
//
// This is useful for deciding whether local proxy is required when web port is
// behind a load balancer.
func (c *Config) DoesDatabaseUseWebProxyHostPort(db tlsca.RouteToDatabase) bool {
	dbHost, dbPort := c.DatabaseProxyHostPort(db)
	webHost, webPort := c.WebProxyHostPort()
	return dbHost == webHost && dbPort == webPort
}

// GetKubeTLSServerName returns k8s server name used in KUBECONFIG to leverage TLS Routing.
func GetKubeTLSServerName(k8host string) string {
	isIPFormat := net.ParseIP(k8host) != nil

	if k8host == "" || k8host == string(teleport.PrincipalLocalhost) || isIPFormat {
		// If proxy is configured without public_addr set the ServerName to the 'kube.teleport.cluster.local' value.
		// The k8s server name needs to be a valid hostname but when public_addr is missing from proxy settings
		// the web_listen_addr is used thus webHost will contain local proxy IP address like: 0.0.0.0 or 127.0.0.1
		return addSubdomainPrefix(constants.APIDomain, constants.KubeTeleportProxyALPNPrefix)
	}
	return addSubdomainPrefix(k8host, constants.KubeTeleportProxyALPNPrefix)
}

func addSubdomainPrefix(domain, prefix string) string {
	return fmt.Sprintf("%s%s", prefix, domain)
}

// ProxyHost returns the hostname of the proxy server (without any port numbers)
func ProxyHost(proxyHost string) string {
	host, _, err := net.SplitHostPort(proxyHost)
	if err != nil {
		return proxyHost
	}
	return host
}

// ProxySpecified returns true if proxy has been specified.
func (c *Config) ProxySpecified() bool {
	return c.WebProxyAddr != ""
}

// ResourceFilter returns the default list resource request for the
// provided resource kind.
func (c *Config) ResourceFilter(kind string) *proto.ListResourcesRequest {
	return &proto.ListResourcesRequest{
		ResourceType:        kind,
		Namespace:           c.Namespace,
		Labels:              c.Labels,
		SearchKeywords:      c.SearchKeywords,
		PredicateExpression: c.PredicateExpression,
		UseSearchAsRoles:    c.UseSearchAsRoles,
	}
}

// DTAuthnRunCeremonyFunc matches the signature of [dtauthn.Ceremony.Run].
type DTAuthnRunCeremonyFunc func(context.Context, devicepb.DeviceTrustServiceClient, *devicepb.UserCertificates) (*devicepb.UserCertificates, error)

// dtAutoEnrollFunc matches the signature of [dtenroll.AutoEnroll].
type dtAutoEnrollFunc func(context.Context, devicepb.DeviceTrustServiceClient) (*devicepb.Device, error)

// TeleportClient is a wrapper around SSH client with teleport specific
// workflow built in.
// TeleportClient is NOT safe for concurrent use.
type TeleportClient struct {
	Config
	localAgent *LocalKeyAgent

	// OnShellCreated gets called when the shell is created. It's
	// safe to keep it nil.
	OnShellCreated ShellCreatedCallback

	// eventsCh is a channel used to inform clients about events have that
	// occurred during the session.
	eventsCh chan events.EventFields

	// Note: there's no mutex guarding this or localAgent, making
	// TeleportClient NOT safe for concurrent use.
	lastPing *webclient.PingResponse
}

// ShellCreatedCallback can be supplied for every teleport client. It will
// be called right after the remote shell is created, but the session
// hasn't begun yet.
//
// It allows clients to cancel SSH action
type ShellCreatedCallback func(s *tracessh.Session, c *tracessh.Client, terminal io.ReadWriteCloser) (exit bool, err error)

// NewClient creates a TeleportClient object and fully configures it
func NewClient(c *Config) (tc *TeleportClient, err error) {
	if len(c.JumpHosts) > 1 {
		return nil, trace.BadParameter("only one jump host is supported, got %v", len(c.JumpHosts))
	}
	// validate configuration
	if c.Username == "" {
		c.Username, err = Username()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.Infof("No teleport login given. defaulting to %s", c.Username)
	}
	if c.WebProxyAddr == "" {
		return nil, trace.BadParameter("No proxy address specified, missed --proxy flag?")
	}
	if c.HostLogin == "" {
		c.HostLogin, err = Username()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.Infof("no host login given. defaulting to %s", c.HostLogin)
	}

	c.Namespace = types.ProcessNamespace(c.Namespace)

	if c.Tracer == nil {
		c.Tracer = tracing.NoopProvider().Tracer(teleport.ComponentTeleport)
	}

	tc = &TeleportClient{
		Config: *c,
	}

	if tc.Stdout == nil {
		tc.Stdout = os.Stdout
	}
	if tc.Stderr == nil {
		tc.Stderr = os.Stderr
	}
	if tc.Stdin == nil {
		tc.Stdin = os.Stdin
	}

	if tc.ClientStore == nil {
		if tc.TLS != nil || tc.AuthMethods != nil {
			// Client will use static auth methods instead of client store.
			// Initialize empty client store to prevent panics.
			tc.ClientStore = NewMemClientStore()
		} else {
			tc.ClientStore = NewFSClientStore(c.KeysDir)
			if c.AddKeysToAgent == AddKeysToAgentOnly {
				// Store client keys in memory, but still save trusted certs and profile to disk.
				tc.ClientStore.KeyStore = NewMemKeyStore()
			}
		}
	}

	// Create a buffered channel to hold events that occurred during this session.
	// This channel must be buffered because the SSH connection directly feeds
	// into it. Delays in pulling messages off the global SSH request channel
	// could lead to the connection hanging.
	tc.eventsCh = make(chan events.EventFields, 1024)

	localAgentCfg := LocalAgentConfig{
		ClientStore: tc.ClientStore,
		Agent:       c.Agent,
		ProxyHost:   tc.WebProxyHost(),
		Username:    c.Username,
		KeysOption:  c.AddKeysToAgent,
		Insecure:    c.InsecureSkipVerify,
		Site:        tc.SiteName,
		LoadAllCAs:  tc.LoadAllCAs,
	}

	// initialize the local agent (auth agent which uses local SSH keys signed by the CA):
	tc.localAgent, err = NewLocalAgent(localAgentCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tc.HostKeyCallback == nil {
		tc.HostKeyCallback = tc.localAgent.HostKeyCallback
	}

	return tc, nil
}

func (tc *TeleportClient) ProfileStatus() (*ProfileStatus, error) {
	status, err := tc.ClientStore.ReadProfileStatus(tc.WebProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// If the profile has a different username than the current client, don't return
	// the profile. This is used for login and logout logic.
	if status.Username != tc.Username {
		return nil, trace.NotFound("no profile for proxy %v and user %v found", tc.WebProxyAddr, tc.Username)
	}
	return status, nil
}

// LoadKeyForCluster fetches a cluster-specific SSH key and loads it into the
// SSH agent.
func (tc *TeleportClient) LoadKeyForCluster(ctx context.Context, clusterName string) error {
	_, span := tc.Tracer.Start(
		ctx,
		"teleportClient/LoadKeyForCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("cluster", clusterName)),
	)
	defer span.End()
	if tc.localAgent == nil {
		return trace.BadParameter("TeleportClient.LoadKeyForCluster called on a client without localAgent")
	}
	if err := tc.localAgent.LoadKeyForCluster(clusterName); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// LoadKeyForClusterWithReissue fetches a cluster-specific SSH key and loads it into the
// SSH agent.  If the key is not found, it is requested to be reissued.
func (tc *TeleportClient) LoadKeyForClusterWithReissue(ctx context.Context, clusterName string) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/LoadKeyForClusterWithReissue",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("cluster", clusterName)),
	)
	defer span.End()

	err := tc.LoadKeyForCluster(ctx, clusterName)
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// Reissuing also loads the new key.
	err = tc.ReissueUserCerts(ctx, CertCacheKeep, ReissueParams{RouteToCluster: clusterName})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SignersForClusterWithReissue fetches cluster-specific signers from stored certificates.
// If the cluster certificates are not found, it is requested to be reissued.
func (tc *TeleportClient) SignersForClusterWithReissue(ctx context.Context, clusterName string) ([]ssh.Signer, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/LoadKeyForClusterWithReissue",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("cluster", clusterName)),
	)
	defer span.End()

	signers, err := tc.localAgent.signersForCluster(clusterName)
	if err == nil {
		return signers, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if err := tc.WithoutJumpHosts(func(tc *TeleportClient) error {
		return tc.ReissueUserCerts(ctx, CertCacheKeep, ReissueParams{RouteToCluster: clusterName})
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	signers, err = tc.localAgent.signersForCluster(clusterName)
	if err != nil {
		log.WithError(err).Warnf("Failed to load/reissue certificates for cluster %q.", clusterName)
		return nil, trace.Wrap(err)
	}
	return signers, nil
}

// LocalAgent is a getter function for the client's local agent
func (tc *TeleportClient) LocalAgent() *LocalKeyAgent {
	return tc.localAgent
}

// RootClusterName returns root cluster name.
func (tc *TeleportClient) RootClusterName(ctx context.Context) (string, error) {
	_, span := tc.Tracer.Start(
		ctx,
		"teleportClient/RootClusterName",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if tc.TLS != nil {
		if len(tc.TLS.Certificates) == 0 || len(tc.TLS.Certificates[0].Certificate) == 0 {
			return "", trace.BadParameter("missing tc.TLS.Certificates")
		}

		cert, err := x509.ParseCertificate(tc.TLS.Certificates[0].Certificate[0])
		if err != nil {
			return "", trace.Wrap(err)
		}

		clusterName := cert.Issuer.CommonName
		if clusterName == "" {
			return "", trace.NotFound("failed to extract root cluster name from Teleport TLS cert")
		}
		return clusterName, nil
	}

	key, err := tc.LocalAgent().GetCoreKey()
	if err != nil {
		return "", trace.Wrap(err)
	}
	name, err := key.RootClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return name, nil
}

type targetNode struct {
	hostname string
	addr     string
}

// getTargetNodes returns a list of node addresses this SSH command needs to
// operate on.
func (tc *TeleportClient) getTargetNodes(ctx context.Context, clt client.GetResourcesClient, options SSHOptions) ([]targetNode, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/getTargetNodes",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if options.HostAddress != "" {
		return []targetNode{
			{
				hostname: options.HostAddress,
				addr:     options.HostAddress,
			},
		}, nil
	}

	// use the target node that was explicitly provided if valid
	if len(tc.Labels) == 0 {
		// detect the common error when users use host:port address format
		_, port, err := net.SplitHostPort(tc.Host)
		// client has used host:port notation
		if err == nil {
			return nil, trace.BadParameter("please use ssh subcommand with '--port=%v' flag instead of semicolon", port)
		}

		addr := net.JoinHostPort(tc.Host, strconv.Itoa(tc.HostPort))
		return []targetNode{
			{
				hostname: tc.Host,
				addr:     addr,
			},
		}, nil
	}

	// find the nodes matching the labels that were provided
	nodes, err := client.GetAllResources[types.Server](ctx, clt, tc.ResourceFilter(types.KindNode))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	retval := make([]targetNode, 0, len(nodes))
	for _, resource := range nodes {
		// always dial nodes by UUID
		retval = append(retval, targetNode{
			hostname: resource.GetHostname(),
			addr:     fmt.Sprintf("%s:0", resource.GetName()),
		})
	}

	return retval, nil
}

// ReissueUserCerts issues new user certs based on params and stores them in
// the local key agent (usually on disk in ~/.tsh).
func (tc *TeleportClient) ReissueUserCerts(ctx context.Context, cachePolicy CertCachePolicy, params ReissueParams) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ReissueUserCerts",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	err = RetryWithRelogin(ctx, tc, func() error {
		return trace.Wrap(clusterClient.ReissueUserCerts(ctx, cachePolicy, params))
	})
	return trace.Wrap(err)
}

// IssueUserCertsWithMFA issues a single-use SSH or TLS certificate for
// connecting to a target (node/k8s/db/app) specified in params with an MFA
// check. A user has to be logged in, there should be a valid login cert
// available.
//
// If access to this target does not require per-connection MFA checks
// (according to RBAC), IssueCertsWithMFA will:
// - for SSH certs, return the existing Key from the keystore.
// - for TLS certs, fall back to ReissueUserCerts.
func (tc *TeleportClient) IssueUserCertsWithMFA(ctx context.Context, params ReissueParams, mfaPromptOpts ...mfa.PromptOpt) (*Key, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/IssueUserCertsWithMFA",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	key, _, err := clusterClient.IssueUserCertsWithMFA(ctx, params, tc.NewMFAPrompt(mfaPromptOpts...))
	return key, trace.Wrap(err)
}

// CreateAccessRequestV2 registers a new access request with the auth server.
func (tc *TeleportClient) CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/CreateAccessRequestV2",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("request", req.GetName())),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	created, err := clusterClient.AuthClient.CreateAccessRequestV2(ctx, req)
	return created, trace.Wrap(err)
}

// GetAccessRequests loads all access requests matching the supplied filter.
func (tc *TeleportClient) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/GetAccessRequests",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("id", filter.ID),
			attribute.String("user", filter.User),
		),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	reqs, err := clusterClient.AuthClient.GetAccessRequests(ctx, filter)
	return reqs, trace.Wrap(err)
}

// GetRole loads a role resource by name.
func (tc *TeleportClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/GetRole",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("role", name),
		),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	role, err := clusterClient.AuthClient.GetRole(ctx, name)
	return role, trace.Wrap(err)
}

// watchCloser is a wrapper around a services.Watcher
// which holds a closer that must be called after the watcher
// is closed.
type watchCloser struct {
	types.Watcher
	io.Closer
}

func (w watchCloser) Close() error {
	return trace.NewAggregate(w.Watcher.Close(), w.Closer.Close())
}

// NewWatcher sets up a new event watcher.
func (tc *TeleportClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/NewWatcher",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("name", watch.Name),
		),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	watcher, err := clusterClient.AuthClient.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.NewAggregate(err, clusterClient.Close())
	}

	return watchCloser{
		Watcher: watcher,
		Closer:  clusterClient,
	}, nil
}

// WithRootClusterClient provides a functional interface for making calls
// against the root cluster's auth server.
// Deprecated: Prefer reusing auth clients instead of creating at worst two
// clients for a single function.
func (tc *TeleportClient) WithRootClusterClient(ctx context.Context, do func(clt auth.ClientI) error) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/WithRootClusterClient",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	clt, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clt.Close()

	return trace.Wrap(do(clt))
}

// NewTracingClient provides a tracing client that will forward spans on to
// the current clusters auth server. The auth server will then forward along to the configured
// telemetry backend.
func (tc *TeleportClient) NewTracingClient(ctx context.Context) (*apitracing.Client, error) {
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := clusterClient.ProxyClient.ClientConfig(ctx, clusterClient.ClusterName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tracingClient, err := client.NewTracingClient(ctx, cfg)
	return tracingClient, trace.Wrap(err)
}

// SSHOptions allow overriding configuration
// used when connecting to a host via [TeleportClient.SSH].
type SSHOptions struct {
	// HostAddress is the address of the target host. If specified it
	// will be used instead of the target provided when `tsh ssh` was invoked.
	HostAddress string
}

// WithHostAddress returns a SSHOptions which overrides the
// target host address with the one provided.
func WithHostAddress(addr string) func(*SSHOptions) {
	return func(opt *SSHOptions) {
		opt.HostAddress = addr
	}
}

// SSH connects to a node and, if 'command' is specified, executes the command on it,
// otherwise runs interactive shell
//
// Returns nil if successful, or (possibly) *exec.ExitError
func (tc *TeleportClient) SSH(ctx context.Context, command []string, runLocally bool, opts ...func(*SSHOptions)) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/SSH",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("proxy_web", tc.Config.WebProxyAddr),
			attribute.String("proxy_ssh", tc.Config.SSHProxyAddr),
		),
	)
	defer span.End()

	var options SSHOptions
	for _, opt := range opts {
		opt(&options)
	}

	// connect to proxy first:
	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}

	clt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clt.Close()

	// which nodes are we executing this commands on?
	nodeAddrs, err := tc.getTargetNodes(ctx, clt.AuthClient, options)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(nodeAddrs) == 0 {
		return trace.BadParameter("no target host specified")
	}

	if len(nodeAddrs) > 1 {
		return tc.runShellOrCommandOnMultipleNodes(ctx, clt, nodeAddrs, command)
	}
	return tc.runShellOrCommandOnSingleNode(ctx, clt, nodeAddrs[0].addr, command, runLocally)
}

// ConnectToNode attempts to establish a connection to the node resolved to by the provided
// NodeDetails. Connecting is attempted both with the already provisioned certificates and
// if per session mfa is required, after completing the mfa ceremony. In the event that both
// fail the error from the connection attempt with the already provisioned certificates will
// be returned. The client from whichever attempt succeeds first will be returned.
func (tc *TeleportClient) ConnectToNode(ctx context.Context, clt *ClusterClient, nodeDetails NodeDetails, user string) (_ *NodeClient, err error) {
	node := nodeName(targetNode{addr: nodeDetails.Addr})
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ConnectToNode",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", nodeDetails.Cluster),
			attribute.String("node", node),
		),
	)
	defer func() { apitracing.EndSpan(span, err) }()

	// if per-session mfa is required, perform the mfa ceremony to get
	// new certificates and use them to connect.
	if nodeDetails.MFACheck != nil && nodeDetails.MFACheck.Required {
		clt, err := tc.connectToNodeWithMFA(ctx, clt, nodeDetails, user)
		return clt, trace.Wrap(err)
	}

	type clientRes struct {
		clt *NodeClient
		err error
	}

	directResultC := make(chan clientRes, 1)
	mfaResultC := make(chan clientRes, 1)

	// use a child context so the goroutines can terminate the other if they succeed
	directCtx, directCancel := context.WithCancel(ctx)
	mfaCtx, mfaCancel := context.WithCancel(ctx)
	go func() {
		connectCtx, span := tc.Tracer.Start(
			directCtx,
			"teleportClient/connectToNode",
			oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			oteltrace.WithAttributes(
				attribute.String("cluster", nodeDetails.Cluster),
				attribute.String("node", node),
			),
		)
		defer span.End()

		// Try connecting to the node with the certs we already have. Note that the different context being provided
		// here is intentional. The underlying stream backing the connection will run for the duration of the session
		// and cause the current span to have a duration longer than just the initial connection. To avoid this the
		// parent context is used.
		conn, details, err := clt.DialHostWithResumption(ctx, nodeDetails.Addr, nodeDetails.Cluster, tc.localAgent.ExtendedAgent)
		if err != nil {
			directResultC <- clientRes{err: err}
			return
		}

		sshConfig := clt.ProxyClient.SSHConfig(user)
		clt, err := NewNodeClient(connectCtx, sshConfig, conn, nodeDetails.ProxyFormat(), nodeDetails.Addr, tc, details.FIPS,
			WithNodeHostname(nodeDetails.hostname), WithSSHLogDir(tc.SSHLogDir))
		directResultC <- clientRes{clt: clt, err: err}
	}()

	go func() {
		// try performing mfa and then connecting with the single use certs
		clt, err := tc.connectToNodeWithMFA(mfaCtx, clt, nodeDetails, user)
		mfaResultC <- clientRes{clt: clt, err: err}
	}()

	var directErr, mfaErr error
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			mfaCancel()
			directCancel()
			return nil, ctx.Err()
		case res := <-directResultC:
			if res.clt != nil {
				mfaCancel()
				res.clt.AddCancel(directCancel)
				return res.clt, nil
			}

			directErr = res.err
		case res := <-mfaResultC:
			if res.clt != nil {
				directCancel()
				res.clt.AddCancel(mfaCancel)
				return res.clt, nil
			}

			mfaErr = res.err
		}
	}

	mfaCancel()
	directCancel()

	switch {
	// No MFA errors, return any errors from the direct connection
	case mfaErr == nil:
		return nil, trace.Wrap(directErr)
	// Any direct connection errors other than access denied, which should be returned
	// if MFA is required, take precedent over MFA errors due to users not having any
	// enrolled devices.
	case !trace.IsAccessDenied(directErr) && errors.Is(mfaErr, auth.ErrNoMFADevices):
		return nil, trace.Wrap(directErr)
	case !errors.Is(mfaErr, io.EOF) && // Ignore any errors from MFA due to locks being enforced, the direct error will be friendlier
		!errors.Is(mfaErr, MFARequiredUnknownErr{}) && // Ignore any failures that occurred before determining if MFA was required
		!errors.Is(mfaErr, services.ErrSessionMFANotRequired): // Ignore any errors caused by attempting the MFA ceremony when MFA will not grant access
		return nil, trace.Wrap(mfaErr)
	default:
		return nil, trace.Wrap(directErr)
	}
}

// MFARequiredUnknownErr indicates that connections to an instance failed
// due to being unable to determine if mfa is required
type MFARequiredUnknownErr struct {
	err error
}

// MFARequiredUnknown creates a new MFARequiredUnknownErr that wraps the
// error encountered attempting to determine if the mfa ceremony should proceed.
func MFARequiredUnknown(err error) error {
	return MFARequiredUnknownErr{err: err}
}

// Error returns the error string of the wrapped error if one exists.
func (m MFARequiredUnknownErr) Error() string {
	if m.err == nil {
		return ""
	}

	return m.err.Error()
}

// Unwrap returns the underlying error from checking if an mfa
// ceremony should have been performed.
func (m MFARequiredUnknownErr) Unwrap() error {
	return m.err
}

// Is determines if the provided error is an MFARequiredUnknownErr.
func (m MFARequiredUnknownErr) Is(err error) bool {
	switch err.(type) {
	case MFARequiredUnknownErr:
		return true
	case *MFARequiredUnknownErr:
		return true
	default:
		return false
	}
}

// connectToNodeWithMFA checks if per session mfa is required to connect to the target host, and
// if it is required, then the mfa ceremony is attempted. The target host is dialed once the ceremony
// completes and new certificates are retrieved.
func (tc *TeleportClient) connectToNodeWithMFA(ctx context.Context, clt *ClusterClient, nodeDetails NodeDetails, user string) (*NodeClient, error) {
	node := nodeName(targetNode{addr: nodeDetails.Addr})
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/connectToNodeWithMFA",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", nodeDetails.Cluster),
			attribute.String("node", node),
		),
	)
	defer span.End()

	// There is no need to attempt a mfa ceremony if the user is attempting
	// to connect to a resource via `tsh ssh --headless`. The entire point
	// of headless authentication is to allow connections to originate from a
	// machine without access to a WebAuthn device.
	if tc.AuthConnector == constants.HeadlessConnector {
		return nil, trace.Wrap(services.ErrSessionMFANotRequired)
	}

	if nodeDetails.MFACheck != nil && !nodeDetails.MFACheck.Required {
		return nil, trace.Wrap(services.ErrSessionMFANotRequired)
	}

	// per-session mfa is required, perform the mfa ceremony
	cfg, err := clt.SessionSSHConfig(ctx, user, nodeDetails)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, details, err := clt.DialHostWithResumption(ctx, nodeDetails.Addr, nodeDetails.Cluster, tc.localAgent.ExtendedAgent)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nodeClient, err := NewNodeClient(ctx, cfg, conn, nodeDetails.ProxyFormat(), nodeDetails.Addr, tc, details.FIPS,
		WithNodeHostname(nodeDetails.hostname), WithSSHLogDir(tc.SSHLogDir))
	return nodeClient, trace.Wrap(err)
}

func (tc *TeleportClient) runShellOrCommandOnSingleNode(ctx context.Context, clt *ClusterClient, nodeAddr string, command []string, runLocally bool) error {
	cluster := clt.ClusterName()
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/runShellOrCommandOnSingleNode",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("node", nodeAddr),
			attribute.String("cluster", cluster),
		),
	)
	defer span.End()

	nodeClient, err := tc.ConnectToNode(
		ctx,
		clt,
		NodeDetails{Addr: nodeAddr, Namespace: tc.Namespace, Cluster: cluster},
		tc.Config.HostLogin,
	)
	if err != nil {
		tc.ExitStatus = 1
		return trace.Wrap(err)
	}
	defer nodeClient.Close()
	// If forwarding ports were specified, start port forwarding.
	if err := tc.startPortForwarding(ctx, nodeClient); err != nil {
		return trace.Wrap(err)
	}

	// If no remote command execution was requested block on which ever comes first:
	//  1) the context which will unblock upon error or user terminating the process
	//  2) ssh.Client.Wait which will unblock when the connection has shut down
	if tc.NoRemoteExec {
		connClosed := make(chan error, 1)
		go func() {
			connClosed <- nodeClient.Client.Wait()
		}()
		log.Debugf("Connected to node, no remote command execution was requested, blocking indefinitely.")
		select {
		case <-ctx.Done():
			// Only return an error if the context was canceled by something other than SIGINT.
			if err := ctx.Err(); !errors.Is(err, context.Canceled) {
				return trace.Wrap(err)
			}
		case err := <-connClosed:
			if !errors.Is(err, io.EOF) {
				return trace.Wrap(err)
			}
		}

		return nil
	}

	// After port forwarding, run a local command that uses the connection, and
	// then disconnect.
	if runLocally {
		if len(tc.Config.LocalForwardPorts) == 0 {
			fmt.Println("Executing command locally without connecting to any servers. This makes no sense.")
		}
		return runLocalCommand(tc.Config.HostLogin, command)
	}

	if len(command) > 0 {
		// Reuse the existing nodeClient we connected above.
		return nodeClient.RunCommand(ctx, command)
	}
	return trace.Wrap(nodeClient.RunInteractiveShell(ctx, types.SessionPeerMode, nil, nil))
}

func (tc *TeleportClient) runShellOrCommandOnMultipleNodes(ctx context.Context, clt *ClusterClient, nodes []targetNode, command []string) error {
	cluster := clt.ClusterName()
	nodeAddrs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeAddrs = append(nodeAddrs, node.addr)
	}
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/runShellOrCommandOnMultipleNodes",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", cluster),
			attribute.StringSlice("node", nodeAddrs),
		),
	)
	defer span.End()

	// There was a command provided, run a non-interactive session against each match
	if len(command) > 0 {
		fmt.Printf("\x1b[1mWARNING\x1b[0m: Multiple nodes matched label selector, running command on all.\n")
		return tc.runCommandOnNodes(ctx, clt, nodes, command)
	}

	// Issue "shell" request to the first matching node.
	fmt.Printf("\x1b[1mWARNING\x1b[0m: Multiple nodes match the label selector, picking first: %q\n", nodeAddrs[0])
	return tc.runShellOrCommandOnSingleNode(ctx, clt, nodeAddrs[0], nil, false)
}

func (tc *TeleportClient) startPortForwarding(ctx context.Context, nodeClient *NodeClient) error {
	for _, fp := range tc.Config.LocalForwardPorts {
		addr := net.JoinHostPort(fp.SrcIP, strconv.Itoa(fp.SrcPort))
		socket, err := net.Listen("tcp", addr)
		if err != nil {
			return trace.Errorf("Failed to bind to %v: %v.", addr, err)
		}
		go nodeClient.listenAndForward(ctx, socket, addr, net.JoinHostPort(fp.DestHost, strconv.Itoa(fp.DestPort)))
	}
	for _, fp := range tc.Config.DynamicForwardedPorts {
		addr := net.JoinHostPort(fp.SrcIP, strconv.Itoa(fp.SrcPort))
		socket, err := net.Listen("tcp", addr)
		if err != nil {
			return trace.Errorf("Failed to bind to %v: %v.", addr, err)
		}
		go nodeClient.dynamicListenAndForward(ctx, socket, addr)
	}
	for _, fp := range tc.Config.RemoteForwardPorts {
		addr := net.JoinHostPort(fp.SrcIP, strconv.Itoa(fp.SrcPort))
		socket, err := nodeClient.Client.Listen("tcp", addr)
		if err != nil {
			// We log the error here instead of returning it to be consistent with
			// the other port forwarding methods, which don't stop the session
			// if forwarding fails.
			message := fmt.Sprintf("Failed to bind on remote host to %v: %v.", addr, err)
			if strings.Contains(err.Error(), remoteForwardUnsupportedMessage) {
				message = "Node does not support remote port forwarding (-R)."
			}
			log.Error(message)
		} else {
			go nodeClient.remoteListenAndForward(ctx, socket, net.JoinHostPort(fp.DestHost, strconv.Itoa(fp.DestPort)), addr)
		}
	}
	return nil
}

// Join connects to the existing/active SSH session
func (tc *TeleportClient) Join(ctx context.Context, mode types.SessionParticipantMode, namespace string, sessionID session.ID, input io.Reader) (err error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/Join",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("session", sessionID.String()),
			attribute.String("mode", string(mode)),
		),
	)
	defer span.End()

	if namespace == "" {
		return trace.BadParameter(auth.MissingNamespaceError)
	}
	tc.Stdin = input
	if sessionID.Check() != nil {
		return trace.Errorf("Invalid session ID format: %s", string(sessionID))
	}

	// connect to proxy:
	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}
	clt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clt.Close()

	// Session joining is not supported in proxy recording mode
	if recConfig, err := clt.AuthClient.GetSessionRecordingConfig(ctx); err != nil {
		// If the user can't see the recording mode, just let them try joining below
		if !trace.IsAccessDenied(err) {
			return trace.Wrap(err)
		}
	} else if services.IsRecordAtProxy(recConfig.GetMode()) {
		return trace.BadParameter("session joining is not supported in proxy recording mode")
	}

	session, err := clt.AuthClient.GetSessionTracker(ctx, string(sessionID))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("session %q not found or it has ended", sessionID)
		}
		return trace.Wrap(err)
	}

	switch kind := session.GetSessionKind(); kind {
	case types.KubernetesSessionKind:
		return trace.BadParameter("session joining for Kubernetes is supported with the command tsh kube join")
	case types.SSHSessionKind:
		// continue
	default:
		return trace.BadParameter("session joining is not supported for %v sessions", kind)
	}
	if types.IsOpenSSHNodeSubKind(session.GetTargetSubKind()) {
		return trace.BadParameter("session joining is only supported for Teleport nodes, not OpenSSH nodes")
	}

	// connect to server:
	nc, err := tc.ConnectToNode(ctx,
		clt,
		NodeDetails{Addr: session.GetAddress() + ":0", Namespace: tc.Namespace, Cluster: clt.ClusterName()},
		tc.Config.HostLogin,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nc.Close()

	// Start forwarding ports if configured.
	if err := tc.startPortForwarding(ctx, nc); err != nil {
		return trace.Wrap(err)
	}

	presenceCtx, presenceCancel := context.WithCancel(ctx)
	defer presenceCancel()

	var beforeStart func(io.Writer)
	if mode == types.SessionModeratorMode {
		beforeStart = func(out io.Writer) {
			nc.OnMFA = func() {
				RunPresenceTask(presenceCtx, out, clt.AuthClient, session.GetSessionID(), tc.NewMFAPrompt(mfa.WithQuiet()))
			}
		}
	}

	fmt.Printf("Joining session with participant mode: %v. \n\n", mode)

	// running shell with a given session means "join" it:
	err = nc.RunInteractiveShell(ctx, mode, session, beforeStart)
	return trace.Wrap(err)
}

// Play replays the recorded session.
func (tc *TeleportClient) Play(ctx context.Context, sessionID string, speed float64) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/Play",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("session", sessionID),
		),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	return playSession(ctx, sessionID, speed, clusterClient.AuthClient)
}

const (
	keyCtrlC = 3
	keyCtrlD = 4
	keySpace = 32
	keyLeft  = 68
	keyRight = 67
	keyUp    = 65
	keyDown  = 66
)

func playSession(ctx context.Context, sessionID string, speed float64, streamer player.Streamer) error {
	sid, err := session.ParseID(sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	term, err := terminal.New(os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		return trace.Wrap(err)
	}
	defer term.Close()

	// configure terminal for direct unbuffered echo-less input
	if term.IsAttached() {
		err := term.InitRaw(true)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	term.Clear() // clear screen between runs:
	term.SetCursorPos(1, 1)

	player, err := player.New(&player.Config{
		SessionID: *sid,
		Streamer:  streamer,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	player.SetSpeed(speed)
	if err := player.Play(); err != nil {
		return trace.Wrap(err)
	}
	playing := true

	// playback control goroutine
	const skipDuration = 10 * time.Second
	go func() {
		var key [1]byte
		for {
			_, err := term.Stdin().Read(key[:])
			if err != nil {
				return
			}
			switch key[0] {
			case keyCtrlC, keyCtrlD:
				player.Close()
				return
			case keySpace:
				if playing {
					player.Pause()
				} else {
					player.Play()
				}
				playing = !playing
			case keyLeft, keyDown:
				current := time.Duration(player.LastPlayed() * int64(time.Millisecond))
				player.SetPos(max(current-skipDuration, 0)) // rewind
				term.Clear()
				term.SetCursorPos(1, 1)
			case keyRight, keyUp:
				current := time.Duration(player.LastPlayed() * int64(time.Millisecond))
				player.SetPos(current + skipDuration) // advance forward
			}
		}
	}()

	var lastTime time.Time
	for evt := range player.C() {
		switch evt := evt.(type) {
		case *apievents.WindowsDesktopSessionStart:
			// TODO(zmb3): restore the playback URL
			message := "Desktop sessions cannot be played with tsh play." +
				" Export the recording to video with tsh recordings export" +
				" or view the recording in your web browser."
			return trace.BadParameter(message)
		case *apievents.AppSessionStart, *apievents.DatabaseSessionStart, *apievents.AppSessionChunk:
			return trace.BadParameter("Interactive session replay is only supported for SSH and Kubernetes sessions." +
				" To play app or database sessions, specify --format=json or --format=yaml.")
		case *apievents.Resize:
			if err := setTermSize(term.Stdout(), evt.TerminalSize); err != nil {
				continue
			}
		case *apievents.SessionStart:
			if err := setTermSize(term.Stdout(), evt.TerminalSize); err != nil {
				continue
			}
		case *apievents.SessionPrint:
			term.Stdout().Write(evt.Data)
			if evt.Time != lastTime {
				term.SetWindowTitle(evt.Time.Format(time.Stamp))
			}
			lastTime = evt.Time
		default:
			continue
		}
	}

	return nil
}

func setTermSize(w io.Writer, size string) error {
	width, height, ok := strings.Cut(size, ":")
	if !ok {
		return trace.Errorf("invalid terminal size %q", size)
	}
	// resize terminal window by sending control sequence:
	_, err := fmt.Fprintf(w, "\x1b[8;%s;%st", height, width)
	return err
}

// PlayFile plays the recorded session from a file.
func PlayFile(ctx context.Context, filename, sid string, speed float64) error {
	streamer := &playFromFileStreamer{filename: filename}
	return playSession(ctx, sid, speed, streamer)
}

// SFTP securely copies files between Nodes or SSH servers using SFTP
func (tc *TeleportClient) SFTP(ctx context.Context, args []string, port int, opts sftp.Options, quiet bool) (err error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/SFTP",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if len(args) < 2 {
		return trace.Errorf("local and remote destinations are required")
	}
	first := args[0]
	last := args[len(args)-1]

	// local copy?
	if !isRemoteDest(first) && !isRemoteDest(last) {
		return trace.BadParameter("no remote destination specified")
	}

	var config *sftpConfig
	if isRemoteDest(last) {
		config, err = tc.uploadConfig(args, port, opts)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		config, err = tc.downloadConfig(args, port, opts)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if config.hostLogin == "" {
		config.hostLogin = tc.Config.HostLogin
	}

	if !quiet {
		config.cfg.ProgressStream = func(fileInfo os.FileInfo) io.ReadWriter {
			return sftp.NewProgressBar(fileInfo.Size(), fileInfo.Name(), tc.Stdout)
		}
	}

	return trace.Wrap(tc.TransferFiles(ctx, config.hostLogin, config.addr, config.cfg))
}

type sftpConfig struct {
	cfg       *sftp.Config
	addr      string
	hostLogin string
}

func (tc *TeleportClient) uploadConfig(args []string, port int, opts sftp.Options) (*sftpConfig, error) {
	// args are guaranteed to have len(args) > 1
	srcPaths := args[:len(args)-1]
	// copy everything except the last arg (the destination)
	dstPath := args[len(args)-1]

	dst, addr, err := getSFTPDestination(dstPath, port)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := sftp.CreateUploadConfig(srcPaths, dst.Path, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sftpConfig{
		cfg:       cfg,
		addr:      addr,
		hostLogin: dst.Login,
	}, nil
}

func (tc *TeleportClient) downloadConfig(args []string, port int, opts sftp.Options) (*sftpConfig, error) {
	if len(args) > 2 {
		return nil, trace.BadParameter("only one source file is supported when downloading files")
	}

	// args are guaranteed to have len(args) > 1
	src, addr, err := getSFTPDestination(args[0], port)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := sftp.CreateDownloadConfig(src.Path, args[1], opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sftpConfig{
		cfg:       cfg,
		addr:      addr,
		hostLogin: src.Login,
	}, nil
}

func getSFTPDestination(target string, port int) (dest *sftp.Destination, addr string, err error) {
	dest, err = sftp.ParseDestination(target)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	addr = net.JoinHostPort(dest.Host.Host(), strconv.Itoa(port))
	return dest, addr, nil
}

// TransferFiles copies files between the current machine and the
// specified Node using the supplied config
func (tc *TeleportClient) TransferFiles(ctx context.Context, hostLogin, nodeAddr string, cfg *sftp.Config) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/TransferFiles",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if hostLogin == "" {
		return trace.BadParameter("host login is not specified")
	}
	if nodeAddr == "" {
		return trace.BadParameter("node address is not specified")
	}

	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}
	clt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clt.Close()

	client, err := tc.ConnectToNode(
		ctx,
		clt,
		NodeDetails{
			Addr:      nodeAddr,
			Namespace: tc.Namespace,
			Cluster:   clt.ClusterName(),
		},
		hostLogin,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(client.TransferFiles(ctx, cfg))
}

func isRemoteDest(name string) bool {
	return strings.ContainsRune(name, ':')
}

// ListNodesWithFilters returns all nodes that match the filters in the current cluster
// that the logged in user has access to.
func (tc *TeleportClient) ListNodesWithFilters(ctx context.Context) ([]types.Server, error) {
	req := proto.ListUnifiedResourcesRequest{
		Kinds:               []string{types.KindNode},
		Labels:              tc.Labels,
		SearchKeywords:      tc.SearchKeywords,
		PredicateExpression: tc.PredicateExpression,
		UseSearchAsRoles:    tc.UseSearchAsRoles,
		SortBy: types.SortBy{
			Field: types.ResourceKind,
		},
	}

	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ListNodesWithFilters",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	clt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	var servers []types.Server
	for {
		page, next, err := client.GetUnifiedResourcePage(ctx, clt.AuthClient, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range page {
			srv, ok := r.ResourceWithLabels.(types.Server)
			if !ok {
				log.Warnf("expected types.Server but received unexpected type %T", r)
				continue
			}

			servers = append(servers, srv)
		}

		req.StartKey = next
		if req.StartKey == "" {
			break
		}
	}

	return servers, nil
}

// GetClusterAlerts returns a list of matching alerts from the current cluster.
func (tc *TeleportClient) GetClusterAlerts(ctx context.Context, req types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	ctx, span := tc.Tracer.Start(ctx,
		"teleportClient/GetClusterAlerts",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	alerts, err := clusterClient.AuthClient.GetClusterAlerts(ctx, req)
	return alerts, trace.Wrap(err)
}

// ListAppServersWithFilters returns a list of application servers.
func (tc *TeleportClient) ListAppServersWithFilters(ctx context.Context, customFilter *proto.ListResourcesRequest) ([]types.AppServer, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ListAppServersWithFilters",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	filter := customFilter
	if filter == nil {
		filter = tc.ResourceFilter(types.KindAppServer)
	}

	servers, err := client.GetAllResources[types.AppServer](ctx, clusterClient.AuthClient, filter)
	return servers, trace.Wrap(err)
}

// ListApps returns all registered applications.
func (tc *TeleportClient) ListApps(ctx context.Context, customFilter *proto.ListResourcesRequest) ([]types.Application, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ListApps",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	servers, err := tc.ListAppServersWithFilters(ctx, customFilter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var apps []types.Application
	for _, server := range servers {
		apps = append(apps, server.GetApp())
	}
	return types.DeduplicateApps(apps), nil
}

// DeleteAppSession removes the specified application access session.
func (tc *TeleportClient) DeleteAppSession(ctx context.Context, sessionID string) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/DeleteAppSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer rootAuthClient.Close()

	return trace.Wrap(rootAuthClient.DeleteAppSession(ctx, types.DeleteAppSessionRequest{SessionID: sessionID}))
}

// ListDatabaseServersWithFilters returns all registered database proxy servers.
func (tc *TeleportClient) ListDatabaseServersWithFilters(ctx context.Context, customFilter *proto.ListResourcesRequest) ([]types.DatabaseServer, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ListDatabaseServersWithFilters",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	filter := customFilter
	if filter == nil {
		filter = tc.ResourceFilter(types.KindDatabaseServer)
	}

	servers, err := client.GetAllResources[types.DatabaseServer](ctx, clusterClient.AuthClient, filter)
	return servers, trace.Wrap(err)
}

// ListDatabases returns all registered databases.
func (tc *TeleportClient) ListDatabases(ctx context.Context, customFilter *proto.ListResourcesRequest) ([]types.Database, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ListDatabases",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	servers, err := tc.ListDatabaseServersWithFilters(ctx, customFilter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var databases []types.Database
	for _, server := range servers {
		databases = append(databases, server.GetDatabase())
	}
	return types.DeduplicateDatabases(databases), nil
}

// roleGetter retrieves roles for the current user
type roleGetter interface {
	GetRoles(ctx context.Context) ([]types.Role, error)
}

// commandLimit determines how many commands may be executed in parallel.
// The limit will one of the following:
//   - 1 if per session mfa is required
//   - 1 if we cannot determine the users role set
//   - half the max connection limit defined by the users role set
//
// Out of an abundance of caution we only use half the max connection
// limit to allow other connections to be established.
func commandLimit(ctx context.Context, getter roleGetter, mfaRequired bool) int {
	if mfaRequired {
		return 1
	}

	roles, err := getter.GetRoles(ctx)
	if err != nil {
		return 1
	}

	max := services.NewRoleSet(roles...).MaxConnections()
	limit := max / 2

	switch {
	case max == 0:
		return -1
	case max == 1:
		return 1
	case limit <= 0:
		return 1
	default:
		return int(limit)
	}
}

type execResult struct {
	hostname   string
	exitStatus int
}

// runCommandOnNodes executes a given bash command on a bunch of remote nodes.
func (tc *TeleportClient) runCommandOnNodes(ctx context.Context, clt *ClusterClient, nodes []targetNode, command []string) error {
	cluster := clt.ClusterName()
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/runCommandOnNodes",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", cluster),
		),
	)
	defer span.End()

	// Let's check if the first node requires mfa.
	// If it's required, run commands sequentially to avoid
	// race conditions and weird ux during mfa.
	mfaRequiredCheck, err := clt.AuthClient.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Node{
			Node: &proto.NodeLogin{
				Node:  nodeName(targetNode{addr: nodes[0].addr}),
				Login: tc.Config.HostLogin,
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if tc.SSHLogDir != "" {
		if err := os.MkdirAll(tc.SSHLogDir, 0o700); err != nil {
			return trace.ConvertSystemError(err)
		}
	}

	resultsCh := make(chan execResult, len(nodes))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(commandLimit(ctx, clt.AuthClient, mfaRequiredCheck.Required))
	for _, node := range nodes {
		node := node
		g.Go(func() error {
			ctx, span := tc.Tracer.Start(
				gctx,
				"teleportClient/executingCommand",
				oteltrace.WithSpanKind(oteltrace.SpanKindClient),
				oteltrace.WithAttributes(attribute.String("node", node.addr)),
			)
			defer span.End()

			nodeClient, err := tc.ConnectToNode(
				ctx,
				clt,
				NodeDetails{
					Addr:      node.addr,
					Namespace: tc.Namespace,
					Cluster:   cluster,
					MFACheck:  mfaRequiredCheck,
					hostname:  node.hostname,
				},
				tc.Config.HostLogin,
			)
			if err != nil {
				// Returning the error here would cancel all the other goroutines, so
				// print the error instead to let them all finish.
				fmt.Fprintln(tc.Stderr, err)
				return nil
			}
			defer nodeClient.Close()

			displayName := nodeName(node)
			fmt.Printf("Running command on %v:\n", displayName)

			if err := nodeClient.RunCommand(ctx, command, WithLabeledOutput()); err != nil && tc.ExitStatus == 0 {
				fmt.Fprintln(tc.Stderr, err)
				return nil
			}
			resultsCh <- execResult{
				hostname:   displayName,
				exitStatus: tc.ExitStatus,
			}
			return nil
		})
	}

	// Non-command-related errors will have already been reported by the goroutines,
	// and command-related errors will be reported in writeCommandResults.
	g.Wait()

	close(resultsCh)
	results := make([]execResult, 0, len(resultsCh))
	for result := range resultsCh {
		results = append(results, result)
	}

	return trace.Wrap(tc.writeCommandResults(results))
}

func (tc *TeleportClient) writeCommandResults(nodes []execResult) error {
	fmt.Println()
	var succeededNodes []string
	var failedNodes []string
	for _, node := range nodes {
		if node.exitStatus != 0 {
			failedNodes = append(failedNodes, node.hostname)
			fmt.Printf("[%v] failed with exit code %d\n", node.hostname, node.exitStatus)
		} else {
			succeededNodes = append(succeededNodes, node.hostname)
			fmt.Printf("[%v] success\n", node.hostname)
		}
	}
	fmt.Printf("\n%d host(s) succeeded; %d host(s) failed\n", len(succeededNodes), len(failedNodes))

	if tc.SSHLogDir != "" {
		if len(succeededNodes) > 0 {
			successFile, err := os.Create(filepath.Join(tc.SSHLogDir, "hosts.succeeded"))
			if err != nil {
				return trace.Wrap(err)
			}
			defer successFile.Close()
			for _, node := range succeededNodes {
				fmt.Fprintln(successFile, node)
			}
		}

		if len(failedNodes) > 0 {
			failFile, err := os.Create(filepath.Join(tc.SSHLogDir, "hosts.failed"))
			if err != nil {
				return trace.Wrap(err)
			}
			defer failFile.Close()
			for _, node := range failedNodes {
				fmt.Fprintln(failFile, node)
			}
		}
	}

	if len(failedNodes) > 0 {
		return trace.Errorf("%d command(s) failed", len(failedNodes))
	}
	return nil
}

func (tc *TeleportClient) newSessionEnv() map[string]string {
	env := map[string]string{
		teleport.SSHSessionWebProxyAddr: tc.WebProxyAddr,
	}
	if tc.SessionID != "" {
		env[sshutils.SessionEnvVar] = tc.SessionID
	}

	for key, val := range tc.extraEnvs {
		env[key] = val
	}
	return env
}

// getProxyLogin determines which SSH principal to use when connecting to proxy.
func (tc *TeleportClient) getProxySSHPrincipal() string {
	if tc.ProxySSHPrincipal != "" {
		return tc.ProxySSHPrincipal
	}

	// if we have any keys in the cache, pull the user's valid principals from it.
	if tc.localAgent != nil {
		signers, err := tc.localAgent.Signers()
		if err == nil && len(signers) > 0 {
			cert, ok := signers[0].PublicKey().(*ssh.Certificate)
			if ok && len(cert.ValidPrincipals) > 0 {
				return cert.ValidPrincipals[0]
			}
		}
	}

	proxyPrincipal := tc.Config.HostLogin
	if len(tc.JumpHosts) > 0 && tc.JumpHosts[0].Username != "" {
		log.Debugf("Setting proxy login to jump host's parameter user %q", tc.JumpHosts[0].Username)
		proxyPrincipal = tc.JumpHosts[0].Username
	}
	return proxyPrincipal
}

const unconfiguredPublicAddrMsg = `WARNING:

The following error has occurred as Teleport does not recognize the address
that is being used to connect to it. This usually indicates that the
'public_addr' configuration option of the 'proxy_service' has not been
set to match the address you are hosting the proxy on.

If 'public_addr' is configured correctly, this could be an indicator of an
attempted man-in-the-middle attack.
`

// formatConnectToProxyErr adds additional user actionable advice to errors
// that are raised during ConnectToProxy.
func formatConnectToProxyErr(err error) error {
	if err == nil {
		return nil
	}

	// Handles the error that occurs when you connect to the Proxy SSH service
	// and the Proxy does not have a correct `public_addr` configured, and the
	// system is configured with non-multiplexed ports.
	if utils.IsHandshakeFailedError(err) {
		const principalStr = "not in the set of valid principals for given certificate"
		if strings.Contains(err.Error(), principalStr) {
			return trace.Wrap(err, unconfiguredPublicAddrMsg)
		}
	}

	return err
}

// ConnectToCluster will dial the auth and proxy server and return a ClusterClient when
// successful.
func (tc *TeleportClient) ConnectToCluster(ctx context.Context) (_ *ClusterClient, err error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ConnectToCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("proxy_web", tc.Config.WebProxyAddr),
			attribute.String("proxy_ssh", tc.Config.SSHProxyAddr),
		),
	)
	defer func() { apitracing.EndSpan(span, err) }()

	cfg, err := tc.generateClientConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pclt, err := proxyclient.NewClient(ctx, proxyclient.ClientConfig{
		ProxyAddress:      cfg.proxyAddress,
		TLSRoutingEnabled: tc.TLSRoutingEnabled,
		TLSConfigFunc: func(cluster string) (*tls.Config, error) {
			if cluster == "" {
				tlsCfg, err := tc.LoadTLSConfig()
				return tlsCfg, trace.Wrap(err)
			}

			tlsCfg, err := tc.LoadTLSConfigForClusters([]string{cluster})
			return tlsCfg, trace.Wrap(err)
		},
		DialOpts:           tc.Config.DialOpts,
		UnaryInterceptors:  []grpc.UnaryClientInterceptor{interceptors.GRPCClientUnaryErrorInterceptor},
		StreamInterceptors: []grpc.StreamClientInterceptor{interceptors.GRPCClientStreamErrorInterceptor},
		SSHConfig:          cfg.ClientConfig,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		ViaJumpHost:        len(tc.JumpHosts) > 0,
		PROXYHeaderGetter:  CreatePROXYHeaderGetter(ctx, tc.PROXYSigner),
		// Connections are only upgraded through web port. Do not upgrade when
		// using SSHProxyAddr in separate port mode.
		ALPNConnUpgradeRequired: tc.TLSRoutingEnabled && tc.TLSRoutingConnUpgradeRequired,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster := tc.SiteName
	connected := pclt.ClusterName()
	root, err := tc.rootClusterName()
	if err == nil && len(tc.JumpHosts) > 0 && connected != root {
		cluster = connected
	}

	authClientCfg, err := pclt.ClientConfig(ctx, cluster)
	if err != nil {
		return nil, trace.NewAggregate(err, pclt.Close())
	}
	authClientCfg.MFAPromptConstructor = tc.NewMFAPrompt
	authClient, err := auth.NewClient(authClientCfg)
	if err != nil {
		return nil, trace.NewAggregate(err, pclt.Close())
	}

	return &ClusterClient{
		tc:          tc,
		ProxyClient: pclt,
		AuthClient:  authClient,
		Tracer:      tc.Tracer,
		cluster:     cluster,
		root:        root,
	}, nil
}

// clientConfig wraps [ssh.ClientConfig] with additional
// information about a cluster.
type clientConfig struct {
	*ssh.ClientConfig
	proxyAddress string
	clusterName  func() string
}

// generateClientConfig returns clientConfig that can be used to establish a
// connection to a cluster.
func (tc *TeleportClient) generateClientConfig(ctx context.Context) (*clientConfig, error) {
	proxyAddr := tc.Config.SSHProxyAddr
	if tc.TLSRoutingEnabled {
		proxyAddr = tc.Config.WebProxyAddr
	}

	hostKeyCallback := tc.HostKeyCallback
	authMethods := append([]ssh.AuthMethod{}, tc.Config.AuthMethods...)
	clusterName := func() string { return tc.SiteName }
	if len(tc.JumpHosts) > 0 {
		log.Debugf("Overriding SSH proxy to JumpHosts's address %q", tc.JumpHosts[0].Addr.String())
		proxyAddr = tc.JumpHosts[0].Addr.Addr

		if tc.localAgent != nil {
			// Wrap host key and auth callbacks using clusterGuesser.
			//
			// clusterGuesser will use the host key callback to guess the target
			// cluster based on the host certificate. It will then use the auth
			// callback to load the appropriate SSH certificate for that cluster.
			clusterGuesser := newProxyClusterGuesser(hostKeyCallback, tc.SignersForClusterWithReissue)
			hostKeyCallback = clusterGuesser.hostKeyCallback
			authMethods = append(authMethods, clusterGuesser.authMethod(ctx))

			rootClusterName, err := tc.rootClusterName()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			clusterName = func() string {
				// Only return the inferred cluster name if it's not the root
				// cluster. If it's the root cluster proxy, tc.SiteName could
				// be pointing at a leaf cluster and we don't want to override
				// that.
				cluster := clusterGuesser.clusterName()
				if cluster != rootClusterName {
					return cluster
				}
				return tc.SiteName
			}
		}
	} else if tc.localAgent != nil {
		// Load SSH certs for all clusters we have, in case we don't yet
		// have a certificate for tc.SiteName (like during `tsh login leaf`).
		signers, err := tc.localAgent.Signers()
		// errNoLocalKeyStore is returned when running in the proxy. The proxy
		// should be passing auth methods via tc.Config.AuthMethods.
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if len(signers) > 0 {
			authMethods = append(authMethods, ssh.PublicKeys(signers...))
		}
	}

	if len(authMethods) == 0 {
		return nil, trace.BadParameter("no SSH auth methods loaded, are you logged in?")
	}

	return &clientConfig{
		ClientConfig: &ssh.ClientConfig{
			User:            tc.getProxySSHPrincipal(),
			HostKeyCallback: hostKeyCallback,
			Auth:            authMethods,
			Timeout:         apidefaults.DefaultIOTimeout,
		},
		proxyAddress: proxyAddr,
		clusterName:  clusterName,
	}, nil
}

// CreatePROXYHeaderGetter returns PROXY headers signer with embedded client source/destination IP addresses,
// which are taken from the context.
func CreatePROXYHeaderGetter(ctx context.Context, proxySigner multiplexer.PROXYHeaderSigner) client.PROXYHeaderGetter {
	if proxySigner == nil {
		return nil
	}

	src, dst := authz.ClientAddrsFromContext(ctx)
	if src != nil && dst != nil {
		return func() ([]byte, error) {
			return proxySigner.SignPROXYHeader(src, dst)
		}
	}

	return nil
}

func (tc *TeleportClient) rootClusterName() (string, error) {
	if tc.localAgent == nil {
		return "", trace.NotFound("cannot load root cluster name without local agent")
	}
	tlsKey, err := tc.localAgent.GetCoreKey()
	if err != nil {
		return "", trace.Wrap(err)
	}
	rootClusterName, err := tlsKey.RootClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return rootClusterName, nil
}

// proxyClusterGuesser matches client SSH certificates to the target cluster of
// an SSH proxy. It uses an ssh.HostKeyCallback to infer the cluster name from
// the proxy host certificate. It then passes that name to signersForCluster to
// get the SSH certificates for that cluster.
type proxyClusterGuesser struct {
	cluster atomic.Pointer[string]

	nextHostKeyCallback ssh.HostKeyCallback
	signersForCluster   func(context.Context, string) ([]ssh.Signer, error)
}

func newProxyClusterGuesser(nextHostKeyCallback ssh.HostKeyCallback, signersForCluster func(context.Context, string) ([]ssh.Signer, error)) *proxyClusterGuesser {
	return &proxyClusterGuesser{
		nextHostKeyCallback: nextHostKeyCallback,
		signersForCluster:   signersForCluster,
	}
}

func (g *proxyClusterGuesser) hostKeyCallback(hostname string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return trace.BadParameter("remote proxy did not present a host certificate")
	}

	clusterName, ok := cert.Permissions.Extensions[utils.CertExtensionAuthority]
	if ok {
		g.cluster.CompareAndSwap(nil, &clusterName)
	}

	if clusterName == "" {
		log.Debugf("Target SSH server %q does not have a cluster name embedded in their certificate; will use all available client certificates to authenticate", hostname)
	}

	if g.nextHostKeyCallback != nil {
		return g.nextHostKeyCallback(hostname, remote, key)
	}
	return nil
}

func (g *proxyClusterGuesser) clusterName() string {
	name := g.cluster.Load()
	if name != nil {
		return *name
	}
	return ""
}

func (g *proxyClusterGuesser) authMethod(ctx context.Context) ssh.AuthMethod {
	return ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		return g.signersForCluster(ctx, g.clusterName())
	})
}

// WithoutJumpHosts executes the given function with a Teleport client that has
// no JumpHosts set, i.e. presumably falling back to the proxy specified in the
// profile.
func (tc *TeleportClient) WithoutJumpHosts(fn func(tcNoJump *TeleportClient) error) error {
	tcNoJump := &TeleportClient{
		Config:         tc.Config,
		localAgent:     tc.localAgent,
		OnShellCreated: tc.OnShellCreated,
		eventsCh:       make(chan events.EventFields, 1024),
		lastPing:       tc.lastPing,
	}
	tcNoJump.JumpHosts = nil

	return trace.Wrap(fn(tcNoJump))
}

// Logout removes certificate and key for the currently logged in user from
// the filesystem and agent.
func (tc *TeleportClient) Logout() error {
	if tc.localAgent == nil {
		return nil
	}
	return tc.localAgent.DeleteKey()
}

// LogoutDatabase removes certificate for a particular database.
func (tc *TeleportClient) LogoutDatabase(dbName string) error {
	if tc.localAgent == nil {
		return nil
	}
	if tc.SiteName == "" {
		return trace.BadParameter("cluster name must be set for database logout")
	}
	if dbName == "" {
		return trace.BadParameter("please specify database name to log out of")
	}
	return tc.localAgent.DeleteUserCerts(tc.SiteName, WithDBCerts{dbName})
}

// LogoutApp removes certificate for the specified app.
func (tc *TeleportClient) LogoutApp(appName string) error {
	if tc.localAgent == nil {
		return nil
	}
	if tc.SiteName == "" {
		return trace.BadParameter("cluster name must be set for app logout")
	}
	if appName == "" {
		return trace.BadParameter("please specify app name to log out of")
	}
	return tc.localAgent.DeleteUserCerts(tc.SiteName, WithAppCerts{appName})
}

// LogoutAll removes all certificates for all users from the filesystem
// and agent.
func (tc *TeleportClient) LogoutAll() error {
	if tc.localAgent == nil {
		return nil
	}
	if err := tc.localAgent.DeleteKeys(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PingAndShowMOTD pings the Teleport Proxy and displays the Message Of The Day if it's available.
func (tc *TeleportClient) PingAndShowMOTD(ctx context.Context) (*webclient.PingResponse, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/PingAndShowMOTD",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	pr, err := tc.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "Teleport proxy not available at %s.", tc.WebProxyAddr)
	}

	if pr.Auth.HasMessageOfTheDay {
		err = tc.ShowMOTD(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return pr, nil
}

// GetWebConfig retrieves Teleport proxy web config
func (tc *TeleportClient) GetWebConfig(ctx context.Context) (*webclient.WebConfig, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/GetWebConfig",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	cfg, err := GetWebConfig(ctx, tc.WebProxyAddr, tc.InsecureSkipVerify)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg, nil
}

// Login logs the user into a Teleport cluster by talking to a Teleport proxy.
//
// The returned Key should typically be passed to ConnectToRootCluster in order to
//
//	update the local agent state and create an initial connection to the cluster.
//
// If the initial login fails due to a private key policy not being met, Login
// will automatically retry login with a private key that meets the required policy.
// This will initiate the same login flow again, aka prompt for password/otp/sso/mfa.
func (tc *TeleportClient) Login(ctx context.Context) (*Key, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/Login",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	// Ping the endpoint to see if it's up and find the type of authentication
	// supported, also show the message of the day if available.
	pr, err := tc.PingAndShowMOTD(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tc.KeyTTL == 0 {
		tc.KeyTTL = time.Duration(pr.Auth.DefaultSessionTTL)
	}
	// todo(lxea): DELETE IN v15(?) where the auth is guaranteed to send us a valid MaxSessionTTL or the auth is guaranteed to interpret 0 duration as the auth's default?
	if tc.KeyTTL == 0 {
		tc.KeyTTL = apidefaults.CertDuration
	}

	// Get the SSHLoginFunc that matches client and cluster settings.
	sshLoginFunc, err := tc.getSSHLoginFunc(pr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := tc.SSHLogin(ctx, sshLoginFunc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use proxy identity if set in key response.
	if key.Username != "" {
		tc.Username = key.Username
		if tc.localAgent != nil {
			tc.localAgent.username = key.Username
		}
	}

	return key, nil
}

// LoginWeb logs the user in via the Teleport web api the same way that the web UI does.
func (tc *TeleportClient) LoginWeb(ctx context.Context) (*WebClient, types.WebSession, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/LoginWeb",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	// Ping the endpoint to see if it's up and find the type of authentication
	// supported, also show the message of the day if available.
	pr, err := tc.Ping(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Get the SSHLoginFunc that matches client and cluster settings.
	webLoginFunc, err := tc.getWebLoginFunc(pr)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var clt *WebClient
	var session types.WebSession
	_, err = tc.loginWithHardwareKeyRetry(ctx, func(ctx context.Context, priv *keys.PrivateKey) error {
		clt, session, err = webLoginFunc(ctx, priv)
		return trace.Wrap(err)
	})

	return clt, session, trace.Wrap(err)
}

// AttemptDeviceLogin attempts device authentication for the current device.
// It expects to receive the latest activated key, as acquired via
// [TeleportClient.Login], and augments the certificates within the key with
// device extensions.
//
// If successful, the new device certificates are automatically activated.
//
// A nil response from this method doesn't mean that device authentication was
// successful, as skipping the ceremony is valid for various reasons (Teleport
// cluster doesn't support device authn, device wasn't enrolled, etc).
// Use [TeleportClient.DeviceLogin] if you want more control over process.
func (tc *TeleportClient) AttemptDeviceLogin(ctx context.Context, key *Key, rootAuthClient auth.ClientI) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/AttemptDeviceLogin",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	pingResp, err := tc.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if !tc.dtAttemptLoginIgnorePing && pingResp.Auth.DeviceTrust.Disabled {
		log.Debug("Device Trust: skipping device authentication, device trust disabled")
		return nil
	}

	newCerts, err := tc.DeviceLogin(
		ctx,
		rootAuthClient,
		&devicepb.UserCertificates{
			// Augment the SSH certificate.
			// The TLS certificate is already part of the connection.
			SshAuthorizedKey: key.Cert,
		})
	switch {
	case errors.Is(err, devicetrust.ErrDeviceKeyNotFound):
		log.Debug("Device Trust: Skipping device authentication, device key not found")
		return nil // err swallowed on purpose
	case errors.Is(err, devicetrust.ErrPlatformNotSupported):
		log.Debug("Device Trust: Skipping device authentication, platform not supported")
		return nil // err swallowed on purpose
	case trace.IsNotImplemented(err):
		log.Debug("Device Trust: Skipping device authentication, not supported by server")
		return nil // err swallowed on purpose
	case err != nil:
		log.WithError(err).Debug("Device Trust: device authentication failed")
		return nil // err swallowed on purpose
	}

	log.Debug("Device Trust: acquired augmented user certificates")
	cp := *key
	cp.Cert = newCerts.SshAuthorizedKey
	cp.TLSCert = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: newCerts.X509Der,
	})

	if err := tc.localAgent.AddKey(&cp); err != nil {
		return trace.Wrap(err)
	}

	// Get the list of host certificates that this cluster knows about.
	hostCerts, err := rootAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCerts := auth.AuthoritiesToTrustedCerts(hostCerts)

	// Update the CA pool and known hosts for all CAs the cluster knows about.
	return trace.Wrap(tc.localAgent.SaveTrustedCerts(trustedCerts))
}

// DeviceLogin attempts to authenticate the current device with Teleport.
//
// The device must be previously registered and enrolled for the authentication
// to succeed (see `tsh device enroll`). Alternatively, if the cluster supports
// auto-enrollment, then DeviceLogin will attempt to auto-enroll the device on
// certain failures and login again.
//
// DeviceLogin may fail for a variety of reasons, some of them legitimate
// (non-Enterprise cluster, Device Trust is disabled, etc). Because of that, a
// failure in this method may not warrant failing a broader action (for example,
// `tsh login`).
//
// Device Trust is a Teleport Enterprise feature.
func (tc *TeleportClient) DeviceLogin(ctx context.Context, rootAuthClient auth.ClientI, certs *devicepb.UserCertificates) (*devicepb.UserCertificates, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/DeviceLogin",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	// Allow tests to override the default authn function.
	runCeremony := tc.DTAuthnRunCeremony
	if runCeremony == nil {
		runCeremony = dtauthn.NewCeremony().Run
	}

	// Login without a previous auto-enroll attempt.
	devicesClient := rootAuthClient.DevicesClient()
	newCerts, loginErr := runCeremony(ctx, devicesClient, certs)
	// Success or auto-enroll impossible.
	if loginErr == nil || errors.Is(loginErr, devicetrust.ErrPlatformNotSupported) || trace.IsNotImplemented(loginErr) {
		return newCerts, trace.Wrap(loginErr)
	}

	// Is auto-enroll enabled?
	pingResp, err := tc.Ping(ctx)
	if err != nil {
		log.WithError(err).Debug("Device Trust: swallowing Ping error for previous Login error")
		return nil, trace.Wrap(loginErr) // err swallowed for loginErr
	}
	if !tc.dtAutoEnrollIgnorePing && !pingResp.Auth.DeviceTrust.AutoEnroll {
		return nil, trace.Wrap(loginErr) // err swallowed for loginErr
	}

	autoEnroll := tc.dtAutoEnroll
	if autoEnroll == nil {
		autoEnroll = dtenroll.AutoEnroll
	}

	// Auto-enroll and Login again.
	if _, err := autoEnroll(ctx, devicesClient); err != nil {
		log.WithError(err).Debug("Device Trust: device auto-enroll failed")
		return nil, trace.Wrap(loginErr) // err swallowed for loginErr
	}
	newCerts, err = runCeremony(ctx, devicesClient, certs)
	return newCerts, trace.Wrap(err)
}

// getSSHLoginFunc returns an SSHLoginFunc that matches client and cluster settings.
func (tc *TeleportClient) getSSHLoginFunc(pr *webclient.PingResponse) (SSHLoginFunc, error) {
	switch pr.Auth.Type {
	case constants.Local:
		authType := constants.LocalConnector
		if pr.Auth.Local != nil {
			authType = pr.Auth.Local.Name
		}
		switch authType {
		case constants.PasswordlessConnector:
			// Sanity check settings.
			if !pr.Auth.AllowPasswordless {
				return nil, trace.BadParameter("passwordless disallowed by cluster settings")
			}
			return tc.pwdlessLogin, nil
		case constants.HeadlessConnector:
			// Sanity check settings.
			if !pr.Auth.AllowHeadless {
				return nil, trace.BadParameter("headless disallowed by cluster settings")
			}
			if tc.AllowHeadless {
				return func(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
					return tc.headlessLogin(ctx, priv)
				}, nil
			}
			return nil, trace.BadParameter("" +
				"Headless login is not supported for this command. " +
				"Only 'tsh ls', 'tsh ssh', and 'tsh scp' are supported.")
		case constants.LocalConnector, "":
			// if passwordless is enabled and there are passwordless credentials
			// registered, we can try to go with passwordless login even though
			// auth=local was selected.
			if tc.canDefaultToPasswordless(pr) {
				log.Debug("Trying passwordless login because credentials were found")
				return tc.pwdlessLogin, nil
			}

			return func(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
				return tc.localLogin(ctx, priv, pr.Auth.SecondFactor)
			}, nil
		default:
			return nil, trace.BadParameter("unsupported authentication connector type: %q", pr.Auth.Local.Name)
		}
	case constants.OIDC:
		oidc := pr.Auth.OIDC
		return func(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
			return tc.ssoLogin(ctx, priv, oidc.Name, oidc.Display, constants.OIDC)
		}, nil
	case constants.SAML:
		saml := pr.Auth.SAML
		return func(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
			return tc.ssoLogin(ctx, priv, saml.Name, saml.Display, constants.SAML)
		}, nil
	case constants.Github:
		github := pr.Auth.Github
		return func(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
			return tc.ssoLogin(ctx, priv, github.Name, github.Display, constants.Github)
		}, nil
	default:
		return nil, trace.BadParameter("unsupported authentication type: %q", pr.Auth.Type)
	}
}

// getWebLoginFunc returns an WebLoginFunc that matches client and cluster settings.
func (tc *TeleportClient) getWebLoginFunc(pr *webclient.PingResponse) (WebLoginFunc, error) {
	switch pr.Auth.Type {
	case constants.Local:
		switch pr.Auth.Local.Name {
		case constants.PasswordlessConnector:
			// Sanity check settings.
			if !pr.Auth.AllowPasswordless {
				return nil, trace.BadParameter("passwordless disallowed by cluster settings")
			}
			return tc.pwdlessLoginWeb, nil
		case constants.HeadlessConnector:
			return nil, trace.BadParameter("headless logins not allowed for web sessions")
		case constants.LocalConnector, "":
			// if passwordless is enabled and there are passwordless credentials
			// registered, we can try to go with passwordless login even though
			// auth=local was selected.
			if tc.canDefaultToPasswordless(pr) {
				log.Debug("Trying passwordless login because credentials were found")
				return tc.pwdlessLoginWeb, nil
			}

			return func(ctx context.Context, priv *keys.PrivateKey) (*WebClient, types.WebSession, error) {
				return tc.localLoginWeb(ctx, priv, pr.Auth.SecondFactor)
			}, nil
		default:
			return nil, trace.BadParameter("unsupported authentication connector type: %q", pr.Auth.Local.Name)
		}
	case constants.OIDC:
		return nil, trace.NotImplemented("SSO login not supported")
	case constants.SAML:
		return nil, trace.NotImplemented("SSO login not supported")
	case constants.Github:
		return nil, trace.NotImplemented("SSO login not supported")
	default:
		return nil, trace.BadParameter("unsupported authentication type: %q", pr.Auth.Type)
	}
}

// pwdlessLoginWeb performs a passwordless ceremony and then makes a request to authenticate via the web api.
func (tc *TeleportClient) pwdlessLoginWeb(ctx context.Context, priv *keys.PrivateKey) (*WebClient, types.WebSession, error) {
	// Only pass on the user if explicitly set, otherwise let the credential
	// picker kick in.
	user := ""
	if tc.ExplicitUsername {
		user = tc.Username
	}

	sshLogin, err := tc.newSSHLogin(priv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clt, session, err := SSHAgentPasswordlessLoginWeb(ctx, SSHLoginPasswordless{
		SSHLogin:                sshLogin,
		User:                    user,
		AuthenticatorAttachment: tc.AuthenticatorAttachment,
		StderrOverride:          tc.Stderr,
		WebauthnLogin:           tc.WebauthnLogin,
	})
	return clt, session, trace.Wrap(err)
}

// localLoginWeb performs the mfa ceremony and then makes a request to authenticate via the web api.
func (tc *TeleportClient) localLoginWeb(ctx context.Context, priv *keys.PrivateKey, secondFactor constants.SecondFactorType) (*WebClient, types.WebSession, error) {
	// TODO(awly): mfa: ideally, clients should always go through mfaLocalLogin
	// (with a nop MFA challenge if no 2nd factor is required). That way we can
	// deprecate the direct login endpoint.
	switch secondFactor {
	case constants.SecondFactorOff, constants.SecondFactorOTP:
		clt, session, err := tc.directLoginWeb(ctx, secondFactor, priv)
		return clt, session, trace.Wrap(err)
	case constants.SecondFactorU2F, constants.SecondFactorWebauthn, constants.SecondFactorOn, constants.SecondFactorOptional:
		clt, session, err := tc.mfaLocalLoginWeb(ctx, priv)
		return clt, session, trace.Wrap(err)
	default:
		return nil, nil, trace.BadParameter("unsupported second factor type: %q", secondFactor)
	}
}

// directLoginWeb asks for a password + OTP token then makes a request to authenticate via the web api.
func (tc *TeleportClient) directLoginWeb(ctx context.Context, secondFactorType constants.SecondFactorType, priv *keys.PrivateKey) (*WebClient, types.WebSession, error) {
	password, err := tc.AskPassword(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Only ask for a second factor if it's enabled.
	var otpToken string
	if secondFactorType == constants.SecondFactorOTP {
		otpToken, err = tc.AskOTP(ctx)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	sshLogin, err := tc.newSSHLogin(priv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// authenticate via the web api
	clt, session, err := SSHAgentLoginWeb(ctx, SSHLoginDirect{
		SSHLogin: sshLogin,
		User:     tc.Username,
		Password: password,
		OTPToken: otpToken,
	})
	return clt, session, trace.Wrap(err)
}

// mfaLocalLoginWeb asks for a password and performs the challenge-response authentication with the web api
func (tc *TeleportClient) mfaLocalLoginWeb(ctx context.Context, priv *keys.PrivateKey) (*WebClient, types.WebSession, error) {
	password, err := tc.AskPassword(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	sshLogin, err := tc.newSSHLogin(priv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clt, session, err := SSHAgentMFAWebSessionLogin(ctx, SSHLoginMFA{
		SSHLogin:  sshLogin,
		User:      tc.Username,
		Password:  password,
		PromptMFA: tc.NewMFAPrompt(),
	})
	return clt, session, trace.Wrap(err)
}

// hasTouchIDCredentials provides indirection for tests.
var hasTouchIDCredentials = touchid.HasCredentials

// canDefaultToPasswordless checks without user interaction
// if there is any registered passwordless login.
func (tc *TeleportClient) canDefaultToPasswordless(pr *webclient.PingResponse) bool {
	// Verify if client flags are compatible with passwordless.
	allowedConnector := tc.AuthConnector == ""
	allowedAttachment := tc.AuthenticatorAttachment == wancli.AttachmentAuto || tc.AuthenticatorAttachment == wancli.AttachmentPlatform
	if !allowedConnector || !allowedAttachment || tc.PreferOTP {
		return false
	}

	// Verify if server is compatible with passwordless.
	if !pr.Auth.AllowPasswordless || pr.Auth.Webauthn == nil {
		return false
	}

	// Only pass on the user if explicitly set, otherwise let the credential
	// picker kick in.
	user := ""
	if tc.ExplicitUsername {
		user = tc.Username
	}

	return hasTouchIDCredentials(pr.Auth.Webauthn.RPID, user)
}

// SSHLoginFunc is a function which carries out authn with an auth server and returns an auth response.
type SSHLoginFunc func(context.Context, *keys.PrivateKey) (*auth.SSHLoginResponse, error)

// SSHLogin uses the given login function to login the client. This function handles
// private key logic and parsing the resulting auth response.
func (tc *TeleportClient) SSHLogin(ctx context.Context, sshLoginFunc SSHLoginFunc) (*Key, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/SSHLogin",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	var response *auth.SSHLoginResponse
	priv, err := tc.loginWithHardwareKeyRetry(ctx, func(ctx context.Context, priv *keys.PrivateKey) error {
		var err error
		response, err = sshLoginFunc(ctx, priv)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check that a host certificate for at least one cluster was returned.
	if len(response.HostSigners) == 0 {
		return nil, trace.BadParameter("bad response from the server: expected at least one certificate, got 0")
	}

	// extract the new certificate out of the response
	key := NewKey(priv)
	key.Cert = response.Cert
	key.TLSCert = response.TLSCert
	key.TrustedCerts = response.HostSigners
	key.Username = response.Username
	key.ProxyHost = tc.WebProxyHost()

	if tc.KubernetesCluster != "" {
		key.KubeTLSCerts[tc.KubernetesCluster] = response.TLSCert
	}
	if tc.DatabaseService != "" {
		key.DBTLSCerts[tc.DatabaseService] = response.TLSCert
	}

	// Store the requested cluster name in the key.
	key.ClusterName = tc.SiteName
	if key.ClusterName == "" {
		rootClusterName := key.TrustedCerts[0].ClusterName
		key.ClusterName = rootClusterName
		tc.SiteName = rootClusterName
	}

	return key, nil
}

// WebLoginFunc is a function which carries out authn with the web server and returns a web session and cookies.
type WebLoginFunc func(context.Context, *keys.PrivateKey) (*WebClient, types.WebSession, error)

func (tc *TeleportClient) loginWithHardwareKeyRetry(ctx context.Context, login func(ctx context.Context, priv *keys.PrivateKey) error) (*keys.PrivateKey, error) {
	priv, err := tc.GetNewLoginKey(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginErr := login(ctx, priv)
	if loginErr != nil {
		if keys.IsPrivateKeyPolicyError(loginErr) {
			privateKeyPolicy, err := keys.ParsePrivateKeyPolicyError(loginErr)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if err := tc.updatePrivateKeyPolicy(privateKeyPolicy); err != nil {
				return nil, trace.Wrap(err)
			}

			fmt.Fprintf(tc.Stderr, "Relogging in with hardware-backed private key.\n")
			priv, err = tc.GetNewLoginKey(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			loginErr = login(ctx, priv)
		}
	}

	return priv, trace.Wrap(loginErr)
}

func (tc *TeleportClient) updatePrivateKeyPolicy(policy keys.PrivateKeyPolicy) error {
	// The current private key was rejected due to an unmet key policy requirement.
	fmt.Fprintf(tc.Stderr, "Unmet private key policy %q.\n", policy)

	// Set the private key policy to the expected value and re-login.
	tc.PrivateKeyPolicy = policy
	return nil
}

// GetNewLoginKey gets a new private key for login.
func (tc *TeleportClient) GetNewLoginKey(ctx context.Context) (priv *keys.PrivateKey, err error) {
	_, span := tc.Tracer.Start(
		ctx,
		"teleportClient/GetNewLoginKey",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if tc.PrivateKeyPolicy.IsHardwareKeyPolicy() {
		log.Debugf("Attempting to login with YubiKey private key.")
		if tc.PIVSlot != "" {
			log.Debugf("Using PIV slot %q specified by client or server settings.", tc.PIVSlot)
		}
		priv, err = keys.GetYubiKeyPrivateKey(ctx, tc.PrivateKeyPolicy, tc.PIVSlot)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return priv, nil
	}

	log.Debugf("Attempting to login with a new RSA private key.")
	priv, err = native.GeneratePrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return priv, nil
}

// new SSHLogin generates a new SSHLogin using the given login key.
func (tc *TeleportClient) newSSHLogin(priv *keys.PrivateKey) (SSHLogin, error) {
	return SSHLogin{
		ProxyAddr:            tc.WebProxyAddr,
		PubKey:               priv.MarshalSSHPublicKey(),
		TTL:                  tc.KeyTTL,
		Insecure:             tc.InsecureSkipVerify,
		Pool:                 loopbackPool(tc.WebProxyAddr),
		Compatibility:        tc.CertificateFormat,
		RouteToCluster:       tc.SiteName,
		KubernetesCluster:    tc.KubernetesCluster,
		AttestationStatement: priv.GetAttestationStatement(),
		ExtraHeaders:         tc.ExtraProxyHeaders,
	}, nil
}

func (tc *TeleportClient) pwdlessLogin(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/pwdlessLogin",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	// Only pass on the user if explicitly set, otherwise let the credential
	// picker kick in.
	user := ""
	if tc.ExplicitUsername {
		user = tc.Username
	}

	sshLogin, err := tc.newSSHLogin(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := SSHAgentPasswordlessLogin(ctx, SSHLoginPasswordless{
		SSHLogin:                sshLogin,
		User:                    user,
		AuthenticatorAttachment: tc.AuthenticatorAttachment,
		StderrOverride:          tc.Stderr,
		WebauthnLogin:           tc.WebauthnLogin,
	})

	return response, trace.Wrap(err)
}

func (tc *TeleportClient) localLogin(ctx context.Context, priv *keys.PrivateKey, secondFactor constants.SecondFactorType) (*auth.SSHLoginResponse, error) {
	var err error
	var response *auth.SSHLoginResponse

	// TODO(awly): mfa: ideally, clients should always go through mfaLocalLogin
	// (with a nop MFA challenge if no 2nd factor is required). That way we can
	// deprecate the direct login endpoint.
	switch secondFactor {
	case constants.SecondFactorOff, constants.SecondFactorOTP:
		response, err = tc.directLogin(ctx, secondFactor, priv)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case constants.SecondFactorU2F, constants.SecondFactorWebauthn, constants.SecondFactorOn, constants.SecondFactorOptional:
		response, err = tc.mfaLocalLogin(ctx, priv)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unsupported second factor type: %q", secondFactor)
	}

	// Ignore username returned from proxy
	response.Username = ""
	return response, nil
}

// directLogin asks for a password + OTP token, makes a request to CA via proxy
func (tc *TeleportClient) directLogin(ctx context.Context, secondFactorType constants.SecondFactorType, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/directLogin",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	password, err := tc.AskPassword(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Only ask for a second factor if it's enabled.
	var otpToken string
	if secondFactorType == constants.SecondFactorOTP {
		otpToken, err = tc.AskOTP(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	sshLogin, err := tc.newSSHLogin(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Ask the CA (via proxy) to sign our public key:
	response, err := SSHAgentLogin(ctx, SSHLoginDirect{
		SSHLogin: sshLogin,
		User:     tc.Username,
		Password: password,
		OTPToken: otpToken,
	})

	return response, trace.Wrap(err)
}

// mfaLocalLogin asks for a password and performs the challenge-response authentication
func (tc *TeleportClient) mfaLocalLogin(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/mfaLocalLogin",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	password, err := tc.AskPassword(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshLogin, err := tc.newSSHLogin(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := SSHAgentMFALogin(ctx, SSHLoginMFA{
		SSHLogin:  sshLogin,
		User:      tc.Username,
		Password:  password,
		PromptMFA: tc.NewMFAPrompt(),
	})

	return response, trace.Wrap(err)
}

func (tc *TeleportClient) headlessLogin(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
	if tc.MockHeadlessLogin != nil {
		return tc.MockHeadlessLogin(ctx, priv)
	}

	headlessAuthenticationID := services.NewHeadlessAuthenticationID(priv.MarshalSSHPublicKey())

	webUILink, err := url.JoinPath("https://"+tc.WebProxyAddr, "web", "headless", headlessAuthenticationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tshApprove := fmt.Sprintf("tsh headless approve --user=%v --proxy=%v %v", tc.Username, tc.WebProxyAddr, headlessAuthenticationID)

	fmt.Fprintf(tc.Stderr, "Complete headless authentication in your local web browser:\n\n%s\n"+
		"\nor execute this command in your local terminal:\n\n%s\n", webUILink, tshApprove)

	response, err := SSHAgentHeadlessLogin(ctx, SSHLoginHeadless{
		SSHLogin: SSHLogin{
			ProxyAddr:         tc.WebProxyAddr,
			PubKey:            priv.MarshalSSHPublicKey(),
			TTL:               tc.KeyTTL,
			Insecure:          tc.InsecureSkipVerify,
			Compatibility:     tc.CertificateFormat,
			KubernetesCluster: tc.KubernetesCluster,
		},
		User:                     tc.Username,
		HeadlessAuthenticationID: headlessAuthenticationID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// SSOLoginFunc is a function used in tests to mock SSO logins.
type SSOLoginFunc func(ctx context.Context, connectorID string, priv *keys.PrivateKey, protocol string) (*auth.SSHLoginResponse, error)

// TODO(atburke): DELETE in v17.0.0
func versionSupportsKeyPolicyMessage(proxyVersion *semver.Version) bool {
	switch proxyVersion.Major {
	case 15:
		return !proxyVersion.LessThan(*semver.New("15.2.5"))
	case 14:
		return !proxyVersion.LessThan(*semver.New("14.3.17"))
	case 13:
		return !proxyVersion.LessThan(*semver.New("13.4.22"))
	default:
		return proxyVersion.Major > 15
	}
}

// samlLogin opens browser window and uses OIDC or SAML redirect cycle with browser
func (tc *TeleportClient) ssoLogin(ctx context.Context, priv *keys.PrivateKey, connectorID, connectorName, protocol string) (*auth.SSHLoginResponse, error) {
	if tc.MockSSOLogin != nil {
		// sso login response is being mocked for testing purposes
		return tc.MockSSOLogin(ctx, connectorID, priv, protocol)
	}

	sshLogin, err := tc.newSSHLogin(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pr, err := tc.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyVersion := semver.New(pr.ServerVersion)

	// ask the CA (via proxy) to sign our public key:
	response, err := SSHAgentSSOLogin(ctx, SSHLoginSSO{
		SSHLogin:                      sshLogin,
		ConnectorID:                   connectorID,
		ConnectorName:                 connectorName,
		Protocol:                      protocol,
		BindAddr:                      tc.BindAddr,
		CallbackAddr:                  tc.CallbackAddr,
		Browser:                       tc.Browser,
		PrivateKeyPolicy:              tc.PrivateKeyPolicy,
		ProxySupportsKeyPolicyMessage: versionSupportsKeyPolicyMessage(proxyVersion),
	}, nil)
	return response, trace.Wrap(err)
}

// ConnectToRootCluster activates the provided key and connects to the
// root cluster with its credentials.
func (tc *TeleportClient) ConnectToRootCluster(ctx context.Context, key *Key) (*ClusterClient, auth.ClientI, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ConnectToRootCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if err := tc.activateKey(ctx, key); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, nil, trace.NewAggregate(err, clusterClient.Close())
	}

	if err := tc.UpdateTrustedCA(ctx, rootAuthClient); err != nil {
		return nil, nil, trace.NewAggregate(err, rootAuthClient.Close(), clusterClient.Close())
	}

	return clusterClient, rootAuthClient, nil
}

// activateKey saves the target session cert into the local
// keystore (and into the ssh-agent) for future use.
func (tc *TeleportClient) activateKey(ctx context.Context, key *Key) error {
	_, span := tc.Tracer.Start(
		ctx,
		"teleportClient/activateKey",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if tc.localAgent == nil {
		// skip activation if no local agent is present
		return nil
	}

	// save the cert to the local storage (~/.tsh usually):
	if err := tc.localAgent.AddKey(key); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Ping makes a ping request to the proxy, and updates tc based on the
// response. The successful ping response is cached, multiple calls to Ping
// will return the original response and skip the round-trip.
//
// Ping can be called for its side-effect of applying the proxy-provided
// settings (such as various listening addresses).
func (tc *TeleportClient) Ping(ctx context.Context) (*webclient.PingResponse, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/Ping",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	// If, at some point, there's a need to bypass this caching, consider
	// adding a bool argument. At the time of writing this we always want to
	// cache.
	if tc.lastPing != nil {
		return tc.lastPing, nil
	}
	pr, err := webclient.Ping(&webclient.Config{
		Context:       ctx,
		ProxyAddr:     tc.WebProxyAddr,
		Insecure:      tc.InsecureSkipVerify,
		Pool:          loopbackPool(tc.WebProxyAddr),
		ConnectorName: tc.AuthConnector,
		ExtraHeaders:  tc.ExtraProxyHeaders,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify server->client and client->server compatibility.
	if tc.CheckVersions {
		if !utils.MeetsVersion(teleport.Version, pr.MinClientVersion) {
			fmt.Fprintf(tc.Stderr, `
WARNING
Detected potentially incompatible client and server versions.
Minimum client version supported by the server is %v but you are using %v.
Please upgrade tsh to %v or newer or use the --skip-version-check flag to bypass this check.
Future versions of tsh will fail when incompatible versions are detected.

`,
				pr.MinClientVersion, teleport.Version, pr.MinClientVersion)
		}

		// Recent `tsh mfa` changes require at least Teleport v15.
		const minServerVersion = "15.0.0-aa" // "-aa" matches all development versions
		if !utils.MeetsVersion(pr.ServerVersion, minServerVersion) {
			fmt.Fprintf(tc.Stderr, `
WARNING
Detected incompatible client and server versions.
Minimum server version supported by tsh is %v but your server is using %v.
Please use a tsh version that matches your server.
You may use the --skip-version-check flag to bypass this check.

`,
				minServerVersion, pr.ServerVersion)
		}
	}

	// Update tc with proxy and auth settings specified in Ping response.
	if err := tc.applyProxySettings(pr.Proxy); err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform the ALPN handshake test during Ping as it's part of the Proxy
	// settings. Only do this when Ping is successful. If tc.lastPing is
	// cached, there is no need to do this test again.
	tc.TLSRoutingConnUpgradeRequired = client.IsALPNConnUpgradeRequired(ctx, tc.WebProxyAddr, tc.InsecureSkipVerify)

	tc.applyAuthSettings(pr.Auth)

	tc.lastPing = pr

	return pr, nil
}

// ShowMOTD fetches the cluster MotD, displays it (if any) and waits for
// confirmation from the user.
func (tc *TeleportClient) ShowMOTD(ctx context.Context) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/ShowMOTD",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	motd, err := webclient.GetMOTD(
		&webclient.Config{
			Context:      ctx,
			ProxyAddr:    tc.WebProxyAddr,
			Insecure:     tc.InsecureSkipVerify,
			Pool:         loopbackPool(tc.WebProxyAddr),
			ExtraHeaders: tc.ExtraProxyHeaders,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	if motd.Text != "" {
		fmt.Fprintln(tc.Stderr, motd.Text)

		// If possible, prompt the user for acknowledement before continuing.
		if stdin := prompt.Stdin(); stdin.IsTerminal() {
			// We're re-using the password reader for user acknowledgment for
			// aesthetic purposes, because we want to hide any garbage the
			// user might enter at the prompt. Whatever the user enters will
			// be simply discarded, and the user can still CTRL+C out if they
			// disagree.
			fmt.Fprintln(tc.Stderr, "Press [ENTER] to continue.")
			if _, err := stdin.ReadPassword(ctx); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// GetTrustedCA returns a list of host certificate authorities
// trusted by the cluster client is authenticated with.
func (tc *TeleportClient) GetTrustedCA(ctx context.Context, clusterName string) ([]types.CertAuthority, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/GetTrustedCA",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("cluster", clusterName)),
	)
	defer span.End()

	// Connect to the proxy.
	if !tc.Config.ProxySpecified() {
		return nil, trace.BadParameter("proxy server is not specified")
	}
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	// Get a client to the Auth Server.
	clt, err := clusterClient.ConnectToCluster(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	// Get the list of host certificates that this cluster knows about.
	cas, err := clt.GetCertAuthorities(ctx, types.HostCA, false)
	return cas, trace.Wrap(err)
}

// UpdateTrustedCA connects to the Auth Server and fetches all host certificates
// and updates ~/.tsh/keys/proxy/certs.pem and ~/.tsh/known_hosts.
func (tc *TeleportClient) UpdateTrustedCA(ctx context.Context, getter services.AuthorityGetter) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/UpdateTrustedCA",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if tc.localAgent == nil {
		return trace.BadParameter("UpdateTrustedCA called on a client without localAgent")
	}
	// Get the list of host certificates that this cluster knows about.
	hostCerts, err := getter.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCerts := auth.AuthoritiesToTrustedCerts(hostCerts)

	// Update the CA pool and known hosts for all CAs the cluster knows about.
	err = tc.localAgent.SaveTrustedCerts(trustedCerts)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// applyProxySettings updates configuration changes based on the advertised
// proxy settings, overriding existing fields in tc.
func (tc *TeleportClient) applyProxySettings(proxySettings webclient.ProxySettings) error {
	// Kubernetes proxy settings.
	if proxySettings.Kube.Enabled {
		switch {
		// PublicAddr is the first preference.
		case proxySettings.Kube.PublicAddr != "":
			if _, err := utils.ParseAddr(proxySettings.Kube.PublicAddr); err != nil {
				return trace.BadParameter(
					"failed to parse value received from the server: %q, contact your administrator for help",
					proxySettings.Kube.PublicAddr)
			}
			tc.KubeProxyAddr = proxySettings.Kube.PublicAddr
		// ListenAddr is the second preference unless TLS routing is enabled.
		case proxySettings.Kube.ListenAddr != "" && !proxySettings.TLSRoutingEnabled:
			addr, err := utils.ParseAddr(proxySettings.Kube.ListenAddr)
			if err != nil {
				return trace.BadParameter(
					"failed to parse value received from the server: %q, contact your administrator for help",
					proxySettings.Kube.ListenAddr)
			}
			// If ListenAddr host is 0.0.0.0 or [::], replace it with something
			// routable from the web endpoint.
			if net.ParseIP(addr.Host()).IsUnspecified() {
				webProxyHost, _ := tc.WebProxyHostPort()
				tc.KubeProxyAddr = net.JoinHostPort(webProxyHost, strconv.Itoa(addr.Port(defaults.KubeListenPort)))
			} else {
				tc.KubeProxyAddr = proxySettings.Kube.ListenAddr
			}
		// If neither PublicAddr nor TunnelAddr are passed, use the web
		// interface hostname with default k8s port as a guess.
		default:
			webProxyHost, _ := tc.WebProxyHostPort()
			tc.KubeProxyAddr = net.JoinHostPort(webProxyHost, strconv.Itoa(defaults.KubeListenPort))
		}
	} else {
		// Zero the field, in case there was a previous value set (e.g. loaded
		// from profile directory).
		tc.KubeProxyAddr = ""
	}

	// Read in settings for HTTP endpoint of the proxy.
	if proxySettings.SSH.PublicAddr != "" {
		addr, err := utils.ParseAddr(proxySettings.SSH.PublicAddr)
		if err != nil {
			return trace.BadParameter(
				"failed to parse value received from the server: %q, contact your administrator for help",
				proxySettings.SSH.PublicAddr)
		}
		tc.WebProxyAddr = net.JoinHostPort(addr.Host(), strconv.Itoa(addr.Port(defaults.HTTPListenPort)))

		if tc.localAgent != nil {
			// Update local agent (that reads/writes to ~/.tsh) with the new address
			// of the web proxy. This will control where the keys are stored on disk
			// after login.
			tc.localAgent.UpdateProxyHost(addr.Host())
		}
	}
	// Read in settings for the SSH endpoint of the proxy.
	//
	// If listen_addr is set, take host from ProxyWebHost and port from what
	// was set. This is to maintain backward compatibility when Teleport only
	// supported public_addr.
	if proxySettings.SSH.ListenAddr != "" {
		addr, err := utils.ParseAddr(proxySettings.SSH.ListenAddr)
		if err != nil {
			return trace.BadParameter(
				"failed to parse value received from the server: %q, contact your administrator for help",
				proxySettings.SSH.ListenAddr)
		}
		webProxyHost, _ := tc.WebProxyHostPort()
		tc.SSHProxyAddr = net.JoinHostPort(webProxyHost, strconv.Itoa(addr.Port(defaults.SSHProxyListenPort)))
	}
	// If ssh_public_addr is set, override settings from listen_addr.
	if proxySettings.SSH.SSHPublicAddr != "" {
		addr, err := utils.ParseAddr(proxySettings.SSH.SSHPublicAddr)
		if err != nil {
			return trace.BadParameter(
				"failed to parse value received from the server: %q, contact your administrator for help",
				proxySettings.SSH.SSHPublicAddr)
		}
		tc.SSHProxyAddr = net.JoinHostPort(addr.Host(), strconv.Itoa(addr.Port(defaults.SSHProxyListenPort)))
	}

	// Read Postgres proxy settings.
	switch {
	case proxySettings.DB.PostgresPublicAddr != "":
		addr, err := utils.ParseAddr(proxySettings.DB.PostgresPublicAddr)
		if err != nil {
			return trace.BadParameter("failed to parse Postgres public address received from server: %q, contact your administrator for help",
				proxySettings.DB.PostgresPublicAddr)
		}
		tc.PostgresProxyAddr = net.JoinHostPort(addr.Host(), strconv.Itoa(addr.Port(tc.WebProxyPort())))
		// Listen address port applies if set and not in TLS routing mode.
	case proxySettings.DB.PostgresListenAddr != "" && !proxySettings.TLSRoutingEnabled:
		addr, err := utils.ParseAddr(proxySettings.DB.PostgresListenAddr)
		if err != nil {
			return trace.BadParameter("failed to parse Postgres listen address received from server: %q, contact your administrator for help",
				proxySettings.DB.PostgresListenAddr)
		}
		tc.PostgresProxyAddr = net.JoinHostPort(tc.WebProxyHost(), strconv.Itoa(addr.Port(defaults.PostgresListenPort)))
	default:
		webProxyHost, webProxyPort := tc.WebProxyHostPort()
		tc.PostgresProxyAddr = net.JoinHostPort(webProxyHost, strconv.Itoa(webProxyPort))
	}

	// Read Mongo proxy settings.
	switch {
	case proxySettings.DB.MongoPublicAddr != "":
		addr, err := utils.ParseAddr(proxySettings.DB.MongoPublicAddr)
		if err != nil {
			return trace.BadParameter("failed to parse Mongo public address received from server: %q, contact your administrator for help",
				proxySettings.DB.MongoPublicAddr)
		}
		tc.MongoProxyAddr = net.JoinHostPort(addr.Host(), strconv.Itoa(addr.Port(tc.WebProxyPort())))
	case proxySettings.DB.MongoListenAddr != "":
		addr, err := utils.ParseAddr(proxySettings.DB.MongoListenAddr)
		if err != nil {
			return trace.BadParameter("failed to parse Mongo listen address received from server: %q, contact your administrator for help",
				proxySettings.DB.MongoListenAddr)
		}
		tc.MongoProxyAddr = net.JoinHostPort(tc.WebProxyHost(), strconv.Itoa(addr.Port(defaults.MongoListenPort)))
	}

	// Read MySQL proxy settings if enabled on the server.
	switch {
	case proxySettings.DB.MySQLPublicAddr != "":
		addr, err := utils.ParseAddr(proxySettings.DB.MySQLPublicAddr)
		if err != nil {
			return trace.BadParameter("failed to parse MySQL public address received from server: %q, contact your administrator for help",
				proxySettings.DB.MySQLPublicAddr)
		}
		tc.MySQLProxyAddr = net.JoinHostPort(addr.Host(), strconv.Itoa(addr.Port(defaults.MySQLListenPort)))
	case proxySettings.DB.MySQLListenAddr != "":
		addr, err := utils.ParseAddr(proxySettings.DB.MySQLListenAddr)
		if err != nil {
			return trace.BadParameter("failed to parse MySQL listen address received from server: %q, contact your administrator for help",
				proxySettings.DB.MySQLListenAddr)
		}
		tc.MySQLProxyAddr = net.JoinHostPort(tc.WebProxyHost(), strconv.Itoa(addr.Port(defaults.MySQLListenPort)))
	}

	tc.TLSRoutingEnabled = proxySettings.TLSRoutingEnabled
	if tc.TLSRoutingEnabled {
		// If proxy supports TLS Routing all k8s requests will be sent to the WebProxyAddr where TLS Routing will identify
		// k8s requests by "kube-teleport-proxy-alpn." SNI prefix and route to the kube proxy service.
		tc.KubeProxyAddr = tc.WebProxyAddr
	}

	return nil
}

// applyAuthSettings updates configuration changes based on the advertised
// authentication settings, overriding existing fields in tc.
func (tc *TeleportClient) applyAuthSettings(authSettings webclient.AuthenticationSettings) {
	tc.LoadAllCAs = authSettings.LoadAllCAs

	// If PIVSlot is not already set, default to the server setting.
	if tc.PIVSlot == "" {
		tc.PIVSlot = authSettings.PIVSlot
	}

	// Update the private key policy from auth settings if it is stricter than the saved setting.
	if authSettings.PrivateKeyPolicy != "" && !authSettings.PrivateKeyPolicy.IsSatisfiedBy(tc.PrivateKeyPolicy) {
		tc.PrivateKeyPolicy = authSettings.PrivateKeyPolicy
	}
}

// AddTrustedCA adds a new CA as trusted CA for this client, used in tests
func (tc *TeleportClient) AddTrustedCA(ctx context.Context, ca types.CertAuthority) error {
	_, span := tc.Tracer.Start(
		ctx,
		"teleportClient/AddTrustedCA",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	if tc.localAgent == nil {
		return trace.BadParameter("TeleportClient.AddTrustedCA called on a client without localAgent")
	}

	err := tc.localAgent.SaveTrustedCerts(auth.AuthoritiesToTrustedCerts([]types.CertAuthority{ca}))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// AddKey adds a key to the client's local agent, used in tests.
func (tc *TeleportClient) AddKey(key *Key) error {
	if tc.localAgent == nil {
		return trace.BadParameter("TeleportClient.AddKey called on a client without localAgent")
	}
	if key.ClusterName == "" {
		key.ClusterName = tc.SiteName
	}
	return tc.localAgent.AddKey(key)
}

// SendEvent adds a events.EventFields to the channel.
func (tc *TeleportClient) SendEvent(ctx context.Context, e events.EventFields) error {
	// Try and send the event to the eventsCh. If blocking, keep blocking until
	// the passed in context in canceled.
	select {
	case tc.eventsCh <- e:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// EventsChannel returns a channel that can be used to listen for events that
// occur for this session.
func (tc *TeleportClient) EventsChannel() <-chan events.EventFields {
	return tc.eventsCh
}

// loopbackPool reads trusted CAs if it finds it in a predefined location
// and will work only if target proxy address is loopback
func loopbackPool(proxyAddr string) *x509.CertPool {
	if !apiutils.IsLoopback(proxyAddr) {
		log.Debugf("not using loopback pool for remote proxy addr: %v", proxyAddr)
		return nil
	}
	log.Debugf("attempting to use loopback pool for local proxy addr: %v", proxyAddr)
	certPool, err := x509.SystemCertPool()
	if err != nil {
		log.Debugf("could not open system cert pool, using empty cert pool instead: %v", err)
		certPool = x509.NewCertPool()
	}

	certPath := filepath.Join(defaults.DataDir, defaults.SelfSignedCertPath)
	log.Debugf("reading self-signed certs from: %v", certPath)

	pemByte, err := os.ReadFile(certPath)
	if err != nil {
		log.Debugf("could not open any path in: %v", certPath)
		return nil
	}

	for {
		var block *pem.Block
		block, pemByte = pem.Decode(pemByte)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			log.Debugf("could not parse cert in: %v, err: %v", certPath, err)
			return nil
		}
		certPool.AddCert(cert)
	}
	log.Debugf("using local pool for loopback proxy: %v, err: %v", certPath, err)
	return certPool
}

// connectToSSHAgent connects to the system SSH agent and returns an agent.Agent.
func connectToSSHAgent() agent.ExtendedAgent {
	socketPath := os.Getenv(teleport.SSHAuthSock)
	conn, err := agentconn.Dial(socketPath)
	if err != nil {
		log.Warnf("[KEY AGENT] Unable to connect to SSH agent on socket %q: %v", socketPath, err)
		return nil
	}

	log.Infof("[KEY AGENT] Connected to the system agent: %q", socketPath)
	return agent.NewClient(conn)
}

// Username returns the current user's username
func Username() (string, error) {
	u, err := apiutils.CurrentUser()
	if err != nil {
		return "", trace.Wrap(err)
	}

	username := u.Username

	// If on Windows, strip the domain name.
	if runtime.GOOS == constants.WindowsOS {
		idx := strings.LastIndex(username, "\\")
		if idx > -1 {
			username = username[idx+1:]
		}
	}

	return username, nil
}

// AskOTP prompts the user to enter the OTP token.
func (tc *TeleportClient) AskOTP(ctx context.Context) (token string, err error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/AskOTP",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	stdin := prompt.Stdin()
	if !stdin.IsTerminal() {
		return "", trace.Wrap(prompt.ErrNotTerminal, "cannot perform OTP login without a terminal")
	}
	return prompt.Password(ctx, tc.Stderr, prompt.Stdin(), "Enter your OTP token")
}

// AskPassword prompts the user to enter the password
func (tc *TeleportClient) AskPassword(ctx context.Context) (pwd string, err error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/AskPassword",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	stdin := prompt.Stdin()
	if !stdin.IsTerminal() {
		return "", trace.Wrap(prompt.ErrNotTerminal, "cannot perform password login without a terminal")
	}
	return prompt.Password(
		ctx, tc.Stderr, stdin, fmt.Sprintf("Enter password for Teleport user %v", tc.Config.Username))
}

// LoadTLSConfig returns the user's TLS configuration, either from static
// configuration or from its key store.
func (tc *TeleportClient) LoadTLSConfig() (*tls.Config, error) {
	if tc.TLS != nil {
		return tc.TLS.Clone(), nil
	}

	tlsKey, err := tc.localAgent.GetCoreKey()
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch TLS key for %v", tc.Username)
	}

	var clusters []string
	if tc.LoadAllCAs {
		clusters, err = tc.localAgent.GetClusterNames()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		rootCluster, err := tlsKey.RootClusterName()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters = []string{rootCluster}
		if tc.SiteName != "" && rootCluster != tc.SiteName {
			// In case of establishing connection to leaf cluster the client validate
			// ssh cert against root cluster proxy cert and leaf cluster cert.
			clusters = append(clusters, tc.SiteName)
		}
	}

	tlsConfig, err := tlsKey.TeleportClientTLSConfig(nil /* cipherSuites */, clusters)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate client TLS config")
	}
	tlsConfig.InsecureSkipVerify = tc.InsecureSkipVerify
	return tlsConfig, nil
}

// LoadTLSConfigForClusters returns the client's TLS configuration, either from static
// configuration or from its key store. If loaded from the key store, CA certs will be
// loaded for the given clusters only.
func (tc *TeleportClient) LoadTLSConfigForClusters(clusters []string) (*tls.Config, error) {
	if tc.TLS != nil {
		return tc.TLS.Clone(), nil
	}

	tlsKey, err := tc.localAgent.GetCoreKey()
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch TLS key for %v", tc.Username)
	}

	tlsConfig, err := tlsKey.TeleportClientTLSConfig(nil /* cipherSuites */, clusters)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate client TLS config")
	}
	tlsConfig.InsecureSkipVerify = tc.InsecureSkipVerify
	return tlsConfig, nil
}

// ParseLabelSpec parses a string like 'name=value,"long name"="quoted value"` into a map like
// { "name" -> "value", "long name" -> "quoted value" }
func ParseLabelSpec(spec string) (map[string]string, error) {
	var tokens []string
	openQuotes := false
	var tokenStart, assignCount int
	specLen := len(spec)
	// tokenize the label spec:
	for i, ch := range spec {
		endOfToken := false
		// end of line?
		if i+utf8.RuneLen(ch) == specLen {
			i += utf8.RuneLen(ch)
			endOfToken = true
		}
		switch ch {
		case '"':
			openQuotes = !openQuotes
		case '=', ',', ';':
			if !openQuotes {
				endOfToken = true
				if ch == '=' {
					assignCount++
				}
			}
		}
		if endOfToken && i > tokenStart {
			tokens = append(tokens, strings.TrimSpace(strings.Trim(spec[tokenStart:i], `"`)))
			tokenStart = i + 1
		}
	}
	// simple validation of tokenization: must have an even number of tokens (because they're pairs)
	// and the number of such pairs must be equal the number of assignments
	if len(tokens)%2 != 0 || assignCount != len(tokens)/2 {
		return nil, fmt.Errorf("invalid label spec: '%s', should be 'key=value'", spec)
	}
	// break tokens in pairs and put into a map:
	labels := make(map[string]string)
	for i := 0; i < len(tokens); i += 2 {
		labels[tokens[i]] = tokens[i+1]
	}
	return labels, nil
}

// ParseSearchKeywords parses a string ie: foo,bar,"quoted value"` into a slice of
// strings: ["foo", "bar", "quoted value"].
// Almost a replica to ParseLabelSpec, but with few modifications such as
// allowing a custom delimiter. Defaults to comma delimiter if not defined.
func ParseSearchKeywords(spec string, customDelimiter rune) []string {
	delimiter := customDelimiter
	if delimiter == 0 {
		delimiter = rune(',')
	}

	var tokens []string
	openQuotes := false
	var tokenStart int
	specLen := len(spec)
	// tokenize the label search:
	for i, ch := range spec {
		endOfToken := false
		if i+utf8.RuneLen(ch) == specLen {
			i += utf8.RuneLen(ch)
			endOfToken = true
		}
		switch ch {
		case '"':
			openQuotes = !openQuotes
		case delimiter:
			if !openQuotes {
				endOfToken = true
			}
		}
		if endOfToken && i > tokenStart {
			tokens = append(tokens, strings.TrimSpace(strings.Trim(spec[tokenStart:i], `"`)))
			tokenStart = i + 1
		}
	}

	return tokens
}

// Executes the given command on the client machine (localhost). If no command is given,
// executes shell
func runLocalCommand(hostLogin string, command []string) error {
	if len(command) == 0 {
		if hostLogin == "" {
			user, err := apiutils.CurrentUser()
			if err != nil {
				return trace.Wrap(err)
			}
			hostLogin = user.Username
		}
		shell, err := shell.GetLoginShell(hostLogin)
		if err != nil {
			return trace.Wrap(err)
		}
		command = []string{shell}
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// String returns the same string spec which can be parsed by ParsePortForwardSpec.
func (fp ForwardedPorts) String() (retval []string) {
	for _, p := range fp {
		retval = append(retval, p.ToString())
	}
	return retval
}

// ParsePortForwardSpec parses parameter to -L flag, i.e. strings like "[ip]:80:remote.host:3000"
// The opposite of this function (spec generation) is ForwardedPorts.String()
func ParsePortForwardSpec(spec []string) (ports ForwardedPorts, err error) {
	if len(spec) == 0 {
		return ports, nil
	}
	const errTemplate = "invalid port forwarding spec '%s': expected format `80:remote.host:80`"
	ports = make([]ForwardedPort, len(spec))

	for i, str := range spec {
		parts := strings.Split(str, ":")
		if len(parts) < 3 || len(parts) > 4 {
			return nil, trace.BadParameter(errTemplate, str)
		}
		if len(parts) == 3 {
			parts = append([]string{"127.0.0.1"}, parts...)
		}
		p := &ports[i]
		p.SrcIP = parts[0]
		p.SrcPort, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, trace.BadParameter(errTemplate, str)
		}
		p.DestHost = parts[2]
		p.DestPort, err = strconv.Atoi(parts[3])
		if err != nil {
			return nil, trace.BadParameter(errTemplate, str)
		}
	}
	return ports, nil
}

// String returns the same string spec which can be parsed by
// ParseDynamicPortForwardSpec.
func (fp DynamicForwardedPorts) String() (retval []string) {
	for _, p := range fp {
		retval = append(retval, p.ToString())
	}
	return retval
}

// ParseDynamicPortForwardSpec parses the dynamic port forwarding spec
// passed in the -D flag. The format of the dynamic port forwarding spec
// is [bind_address:]port.
func ParseDynamicPortForwardSpec(spec []string) (DynamicForwardedPorts, error) {
	result := make(DynamicForwardedPorts, 0, len(spec))

	for _, str := range spec {
		// Check whether this is only the port number, like "1080".
		// net.SplitHostPort would fail on that unless there's a colon in
		// front.
		if !strings.Contains(str, ":") {
			str = ":" + str
		}
		host, port, err := net.SplitHostPort(str)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// If no host is provided, bind to localhost.
		if host == "" {
			host = defaults.Localhost
		}

		srcPort, err := strconv.Atoi(port)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		result = append(result, DynamicForwardedPort{
			SrcIP:   host,
			SrcPort: srcPort,
		})
	}

	return result, nil
}

// InsecureSkipHostKeyChecking is used when the user passes in
// "StrictHostKeyChecking yes".
func InsecureSkipHostKeyChecking(host string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}

// isFIPS returns if the binary was build with BoringCrypto, which implies
// FedRAMP/FIPS 140-2 mode for tsh.
func isFIPS() bool {
	return modules.GetModules().IsBoringBinary()
}

func findActiveDatabases(key *Key) ([]tlsca.RouteToDatabase, error) {
	dbCerts, err := key.DBTLSCertificates()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var databases []tlsca.RouteToDatabase
	for _, cert := range dbCerts {
		tlsID, err := tlsca.FromSubject(cert.Subject, time.Time{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// If the cert expiration time is less than 5s consider cert as expired and don't add
		// it to the user profile as an active database.
		if time.Until(cert.NotAfter) < 5*time.Second {
			continue
		}
		if tlsID.RouteToDatabase.ServiceName != "" {
			databases = append(databases, tlsID.RouteToDatabase)
		}
	}
	return databases, nil
}

func findActiveApps(key *Key) ([]tlsca.RouteToApp, error) {
	appCerts, err := key.AppTLSCertificates()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var apps []tlsca.RouteToApp
	for _, cert := range appCerts {
		tlsID, err := tlsca.FromSubject(cert.Subject, time.Time{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// If the cert expiration time is less than 5s consider cert as expired and don't add
		// it to the user profile as an active database.
		if time.Until(cert.NotAfter) < 5*time.Second {
			continue
		}
		if tlsID.RouteToApp.Name != "" {
			apps = append(apps, tlsID.RouteToApp)
		}
	}
	return apps, nil
}

// getDesktopEventWebURL returns the web UI URL users can access to
// watch a desktop session recording in the browser
func getDesktopEventWebURL(proxyHost string, cluster string, sid *session.ID, events []events.EventFields) string {
	if len(events) < 1 {
		return ""
	}
	start := events[0].GetTimestamp()
	end := events[len(events)-1].GetTimestamp()
	duration := end.Sub(start)

	return fmt.Sprintf("https://%s/web/cluster/%s/session/%s?recordingType=desktop&durationMs=%d", proxyHost, cluster, sid, duration/time.Millisecond)
}

// SearchSessionEvents allows searching for session events with a full pagination support.
func (tc *TeleportClient) SearchSessionEvents(ctx context.Context, fromUTC, toUTC time.Time, pageSize int, order types.EventOrder, max int) ([]apievents.AuditEvent, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/SearchSessionEvents",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.Int("page_size", pageSize),
			attribute.String("from", fromUTC.Format(time.RFC3339)),
			attribute.String("to", toUTC.Format(time.RFC3339)),
		),
	)
	defer span.End()
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	sessions, err := GetPaginatedSessions(ctx, fromUTC, toUTC, pageSize, order, max, clusterClient.AuthClient)
	return sessions, trace.Wrap(err)
}

func parseMFAMode(in string) (wancli.AuthenticatorAttachment, error) {
	switch in {
	case "auto", "":
		return wancli.AttachmentAuto, nil
	case "platform":
		return wancli.AttachmentPlatform, nil
	case "cross-platform":
		return wancli.AttachmentCrossPlatform, nil
	default:
		return 0, trace.BadParameter("unsupported mfa mode %q", in)
	}
}

// NewKubernetesServiceClient connects to the proxy and returns an authenticated gRPC
// client to the Kubernetes service.
func (tc *TeleportClient) NewKubernetesServiceClient(ctx context.Context, clusterName string) (kubeproto.KubeServiceClient, error) {
	if !tc.TLSRoutingEnabled {
		return nil, trace.BadParameter("kube service is not supported if TLS routing is not enabled")
	}
	// get tlsConfig to dial to proxy.
	tlsConfig, err := tc.LoadTLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Set the ALPN protocols to use when dialing the proxy gRPC mTLS endpoint.
	tlsConfig.NextProtos = []string{string(alpncommon.ProtocolProxyGRPCSecure), http2.NextProtoTLS}

	clt, err := client.New(ctx, client.Config{
		Addrs:            []string{tc.Config.WebProxyAddr},
		DialInBackground: false,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		ALPNConnUpgradeRequired:  tc.TLSRoutingConnUpgradeRequired,
		InsecureAddressDiscovery: tc.InsecureSkipVerify,
		MFAPromptConstructor:     tc.NewMFAPrompt,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return kubeproto.NewKubeServiceClient(clt.GetConnection()), nil
}

// IsALPNConnUpgradeRequiredForWebProxy returns true if connection upgrade is
// required for provided addr. The provided address must be a web proxy
// address.
func (tc *TeleportClient) IsALPNConnUpgradeRequiredForWebProxy(ctx context.Context, proxyAddr string) bool {
	// Use cached value.
	if proxyAddr == tc.WebProxyAddr {
		return tc.TLSRoutingConnUpgradeRequired
	}
	// Do a test for other proxy addresses.
	return client.IsALPNConnUpgradeRequired(ctx, proxyAddr, tc.InsecureSkipVerify)
}

// RootClusterCACertPool returns a *x509.CertPool with the root cluster CA.
func (tc *TeleportClient) RootClusterCACertPool(ctx context.Context) (*x509.CertPool, error) {
	_, span := tc.Tracer.Start(
		ctx,
		"teleportClient/RootClusterCACertPool",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	key, err := tc.localAgent.GetCoreKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rootClusterName, err := key.RootClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool, err := key.clientCertPool(rootClusterName)
	return pool, trace.Wrap(err)
}

// HeadlessApprove handles approval of a headless authentication request.
func (tc *TeleportClient) HeadlessApprove(ctx context.Context, headlessAuthenticationID string, confirm bool) error {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/HeadlessApprove",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("proxy_web", tc.Config.WebProxyAddr),
			attribute.String("proxy_ssh", tc.Config.SSHProxyAddr),
		),
	)
	defer span.End()

	// connect to proxy first:
	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	rootClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer rootClient.Close()

	headlessAuthn, err := rootClient.GetHeadlessAuthentication(ctx, headlessAuthenticationID)
	if err != nil {
		return trace.Wrap(err)
	}

	if !headlessAuthn.State.IsPending() {
		return trace.Errorf("cannot approve a headless authentication from a non-pending state: %v", headlessAuthn.State.Stringify())
	}

	fmt.Fprintf(tc.Stdout, "Headless login attempt from IP address %q requires approval.\nContact your administrator if you didn't initiate this login attempt.\n", headlessAuthn.ClientIpAddress)

	if confirm {
		ok, err := prompt.Confirmation(ctx, tc.Stdout, prompt.Stdin(), "Approve?")
		if err != nil {
			return trace.Wrap(err)
		}

		if !ok {
			err = rootClient.UpdateHeadlessAuthenticationState(ctx, headlessAuthenticationID, types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED, nil)
			return trace.Wrap(err)
		}
	}

	chal, err := rootClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{},
		},
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_HEADLESS_LOGIN,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := tc.PromptMFA(ctx, chal)
	if err != nil {
		return trace.Wrap(err)
	}

	err = rootClient.UpdateHeadlessAuthenticationState(ctx, headlessAuthenticationID, types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED, resp)
	return trace.Wrap(err)
}
