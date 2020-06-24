/*
Copyright 2016-2019 Gravitational, Inc.

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

package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/shell"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/agentconn"
	"github.com/gravitational/teleport/lib/wrappers"

	"github.com/gravitational/trace"

	"github.com/docker/docker/pkg/term"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentClient,
})

const (
	// ProfileDir is a directory location where tsh profiles (and session keys) are stored
	ProfileDir = ".tsh"
)

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

	// Remote host to connect
	Host string

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

	// KeyTTL is a time to live for the temporary SSH keypair to remain valid:
	KeyTTL time.Duration

	// InsecureSkipVerify is an option to skip HTTPS cert check
	InsecureSkipVerify bool

	// SkipLocalAuth tells the client to use AuthMethods parameter for authentication and NOT
	// use its own SSH agent or ask user for passwords. This is used by external programs linking
	// against Teleport client and obtaining credentials from elsewhere.
	SkipLocalAuth bool

	// Agent is used when SkipLocalAuth is true
	Agent agent.Agent

	// ForwardAgent is used by the client to request agent forwarding from the server.
	ForwardAgent bool

	// AuthMethods are used to login into the cluster. If specified, the client will
	// use them in addition to certs stored in its local agent (from disk)
	AuthMethods []ssh.AuthMethod

	// TLSConfig is TLS configuration, if specified, the client
	// will use this TLS configuration to access API endpoints
	TLS *tls.Config

	// DefaultPrincipal determines the default SSH username (principal) the client should be using
	// when connecting to auth/proxy servers. Usually it's returned with a certificate,
	// but this variables provides a default (used by the web-based terminal client)
	DefaultPrincipal string

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	// ExitStatus carries the returned value (exit status) of the remote
	// process execution (via SSH exec)
	ExitStatus int

	// SiteName specifies site to execute operation,
	// if omitted, first available site will be selected
	SiteName string

	// LocalForwardPorts are the local ports tsh listens on for port forwarding
	// (parameters to -L ssh flag).
	LocalForwardPorts ForwardedPorts

	// DynamicForwardedPorts are the list of ports tsh listens on for dynamic
	// port forwarding (parameters to -D ssh flag).
	DynamicForwardedPorts DynamicForwardedPorts

	// HostKeyCallback will be called to check host keys of the remote
	// node, if not specified will be using CheckHostSignature function
	// that uses local cache to validate hosts
	HostKeyCallback ssh.HostKeyCallback

	// KeyDir defines where temporary session keys will be stored.
	// if empty, they'll go to ~/.tsh
	KeysDir string

	// Env is a map of environmnent variables to send when opening session
	Env map[string]string

	// Interactive, when set to true, tells tsh to launch a remote command
	// in interactive mode, i.e. attaching the temrinal to it
	Interactive bool

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

	// CheckVersions will check that client version is compatible
	// with auth server version when connecting.
	CheckVersions bool

	// BindAddr is an optional host:port to bind to for SSO redirect flows.
	BindAddr string

	// NoRemoteExec will not execute a remote command after connecting to a host,
	// will block instead. Useful when port forwarding. Equivalent of -N for OpenSSH.
	NoRemoteExec bool

	// Browser can be used to pass the name of a browser to override the system default
	// (not currently implemented), or set to 'none' to suppress browser opening entirely.
	Browser string

	// UseLocalSSHAgent will write user certificates to the local ssh-agent (or
	// similar) socket at $SSH_AUTH_SOCK.
	UseLocalSSHAgent bool

	// EnableEscapeSequences will scan Stdin for SSH escape sequences during
	// command/shell execution. This also requires Stdin to be an interactive
	// terminal.
	EnableEscapeSequences bool
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
		UseLocalSSHAgent:      true,
		EnableEscapeSequences: true,
	}
}

// ProfileStatus combines metadata from the logged in profile and associated
// SSH certificate.
type ProfileStatus struct {
	// ProxyURL is the URL the web client is accessible at.
	ProxyURL url.URL

	// Username is the Teleport username.
	Username string

	// Roles is a list of Teleport Roles this user has been assigned.
	Roles []string

	// Logins are the Linux accounts, also known as principals in OpenSSH terminology.
	Logins []string

	// ValidUntil is the time at which this SSH certificate will expire.
	ValidUntil time.Time

	// Extensions is a list of enabled SSH features for the certificate.
	Extensions []string

	// Cluster is a selected cluster
	Cluster string

	// Traits hold claim data used to populate a role at runtime.
	Traits wrappers.Traits

	// ActiveRequests tracks the privilege escalation requests applied
	// during certificate construction.
	ActiveRequests services.RequestIDs
}

// IsExpired returns true if profile is not expired yet
func (p *ProfileStatus) IsExpired(clock clockwork.Clock) bool {
	return p.ValidUntil.Sub(clock.Now()) <= 0
}

// RetryWithRelogin is a helper error handling method,
// attempts to relogin and retry the function once
func RetryWithRelogin(ctx context.Context, tc *TeleportClient, fn func() error) error {
	err := fn()
	if err == nil {
		return nil
	}
	// Assume that failed handshake is a result of expired credentials,
	// retry the login procedure
	if !utils.IsHandshakeFailedError(err) && !utils.IsCertExpiredError(err) && !trace.IsBadParameter(err) && !trace.IsTrustError(err) {
		return err
	}
	log.Debugf("Activating relogin on %v.", err)
	key, err := tc.Login(ctx, true)
	if err != nil {
		if trace.IsTrustError(err) {
			return trace.Wrap(err, "refusing to connect to untrusted proxy %v without --insecure flag\n", tc.Config.SSHProxyAddr)
		}
		return trace.Wrap(err)
	}
	// Save profile to record proxy credentials
	if err := tc.SaveProfile(key.ProxyHost, "", ProfileCreateNew|ProfileMakeCurrent); err != nil {
		log.Warningf("Failed to save profile: %v", err)
		return trace.Wrap(err)
	}
	// Override client's auth methods, current cluster and user name
	authMethod, err := key.AsAuthMethod()
	if err != nil {
		return trace.Wrap(err)
	}
	// After successful login we have local agent updated with latest
	// and greatest auth information, setup client to try only this new
	// method fetched from key, to isolate the retry
	tc.Config.AuthMethods = []ssh.AuthMethod{authMethod}
	return fn()
}

// readProfile reads in the profile as well as the associated certificate
// and returns a *ProfileStatus which can be used to print the status of the
// profile.
func readProfile(profileDir string, profileName string) (*ProfileStatus, error) {
	var err error

	// Read in the profile for this proxy.
	profile, err := ProfileFromFile(filepath.Join(profileDir, profileName))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Read in the SSH certificate for the user logged into this proxy.
	store, err := NewFSLocalKeyStore(profileDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keys, err := store.GetKey(profile.Name(), profile.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(keys.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, ok := publicKey.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("no certificate found")
	}

	// Extract from the certificate how much longer it will be valid for.
	validUntil := time.Unix(int64(cert.ValidBefore), 0)

	// Extract roles from certificate. Note, if the certificate is in old format,
	// this will be empty.
	var roles []string
	rawRoles, ok := cert.Extensions[teleport.CertExtensionTeleportRoles]
	if ok {
		roles, err = services.UnmarshalCertRoles(rawRoles)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	sort.Strings(roles)

	// Extract traits from the certificate. Note if the certificate is in the
	// old format, this will be empty.
	var traits wrappers.Traits
	rawTraits, ok := cert.Extensions[teleport.CertExtensionTeleportTraits]
	if ok {
		err = wrappers.UnmarshalTraits([]byte(rawTraits), &traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var activeRequests services.RequestIDs
	rawRequests, ok := cert.Extensions[teleport.CertExtensionTeleportActiveRequests]
	if ok {
		if err := activeRequests.Unmarshal([]byte(rawRequests)); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Extract extensions from certificate. This lists the abilities of the
	// certificate (like can the user request a PTY, port forwarding, etc.)
	var extensions []string
	for ext := range cert.Extensions {
		if ext == teleport.CertExtensionTeleportRoles ||
			ext == teleport.CertExtensionTeleportTraits ||
			ext == teleport.CertExtensionTeleportRouteToCluster ||
			ext == teleport.CertExtensionTeleportActiveRequests {
			continue
		}
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)

	// Extract cluster name from the profile.
	clusterName := profile.SiteName
	// DELETE IN: 4.2.0.
	//
	// Older versions of tsh did not always store the cluster name in the
	// profile. If no cluster name is found, fallback to the name of the profile
	// for backward compatibility.
	if clusterName == "" {
		clusterName = profile.Name()
	}

	return &ProfileStatus{
		ProxyURL: url.URL{
			Scheme: "https",
			Host:   profile.WebProxyAddr,
		},
		Username:       profile.Username,
		Logins:         cert.ValidPrincipals,
		ValidUntil:     validUntil,
		Extensions:     extensions,
		Roles:          roles,
		Cluster:        clusterName,
		Traits:         traits,
		ActiveRequests: activeRequests,
	}, nil
}

// fullProfileName takes a profile directory and the host the user is trying
// to connect to and returns the name of the profile file.
func fullProfileName(profileDir string, proxyHost string) (string, error) {
	var err error
	var profileName string

	// If no profile name was passed in, try and extract the active profile from
	// the ~/.tsh/profile symlink. If one was passed in, append .yaml to name.
	if proxyHost == "" {
		profileName, err = os.Readlink(filepath.Join(profileDir, "profile"))
		if err != nil {
			return "", trace.ConvertSystemError(err)
		}
	} else {
		profileName = proxyHost + ".yaml"
	}

	// Make sure the profile requested actually exists.
	_, err = os.Stat(filepath.Join(profileDir, profileName))
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}

	return profileName, nil
}

// Status returns the active profile as well as a list of available profiles.
func Status(profileDir string, proxyHost string) (*ProfileStatus, []*ProfileStatus, error) {
	var err error
	var profile *ProfileStatus
	var others []*ProfileStatus

	// remove ports from proxy host, because profile name is stored
	// by host name
	if proxyHost != "" {
		proxyHost, err = utils.Host(proxyHost)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	// Construct the full path to the profile requested and make sure it exists.
	profileDir = FullProfilePath(profileDir)
	stat, err := os.Stat(profileDir)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if !stat.IsDir() {
		return nil, nil, trace.BadParameter("profile path not a directory")
	}

	// Construct the name of the profile requested. If an empty string was
	// passed in, the name of the active profile will be extracted from the
	// ~/.tsh/profile symlink.
	profileName, err := fullProfileName(profileDir, proxyHost)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, nil, trace.NotFound("not logged in")
		}
		return nil, nil, trace.Wrap(err)
	}

	// Read in the active profile first. If readProfile returns trace.NotFound,
	// that means the profile may have been corrupted (for example keys were
	// deleted but profile exists), treat this as the user not being logged in.
	profile, err = readProfile(profileDir, profileName)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}
		// Make sure the profile is nil, which tsh uses to detect that no
		// active profile exists.
		profile = nil
	}

	// Next, get list of all other available profiles. Filter out logged in
	// profile if it exists and return a slice of *ProfileStatus.
	files, err := ioutil.ReadDir(profileDir)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}
		if file.Name() == profileName {
			continue
		}
		ps, err := readProfile(profileDir, file.Name())
		if err != nil {
			// parts of profile are missing?
			// status skips these files
			if trace.IsNotFound(err) {
				continue
			}
			return nil, nil, trace.Wrap(err)
		}
		others = append(others, ps)
	}

	return profile, others, nil
}

// LoadProfile populates Config with the values stored in the given
// profiles directory. If profileDir is an empty string, the default profile
// directory ~/.tsh is used.
func (c *Config) LoadProfile(profileDir string, proxyName string) error {
	profileDir = FullProfilePath(profileDir)
	// read the profile:
	cp, err := ProfileFromDir(profileDir, ProxyHost(proxyName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	c.Username = cp.Username
	c.SiteName = cp.SiteName
	c.KubeProxyAddr = cp.KubeProxyAddr
	c.WebProxyAddr = cp.WebProxyAddr
	c.SSHProxyAddr = cp.SSHProxyAddr

	c.LocalForwardPorts, err = ParsePortForwardSpec(cp.ForwardedPorts)
	if err != nil {
		log.Warnf("Unable to parse port forwarding in user profile: %v.", err)
	}

	c.DynamicForwardedPorts, err = ParseDynamicPortForwardSpec(cp.DynamicForwardedPorts)
	if err != nil {
		log.Warnf("Unable to parse dynamic port forwarding in user profile: %v.", err)
	}

	return nil
}

// SaveProfile updates the given profiles directory with the current configuration
// If profileDir is an empty string, the default ~/.tsh is used
func (c *Config) SaveProfile(profileAliasHost, profileDir string, profileOptions ...ProfileOptions) error {
	if c.WebProxyAddr == "" {
		return nil
	}

	// The profile is saved to a directory with the name of the proxy web endpoint.
	webProxyHost, _ := c.WebProxyHostPort()
	profileDir = FullProfilePath(profileDir)
	profilePath := path.Join(profileDir, webProxyHost) + ".yaml"

	profileAliasPath := ""
	if profileAliasHost != "" {
		profileAliasPath = path.Join(profileDir, profileAliasHost) + ".yaml"
	}

	var cp ClientProfile
	cp.Username = c.Username
	cp.WebProxyAddr = c.WebProxyAddr
	cp.SSHProxyAddr = c.SSHProxyAddr
	cp.KubeProxyAddr = c.KubeProxyAddr
	cp.ForwardedPorts = c.LocalForwardPorts.String()
	cp.SiteName = c.SiteName

	// create a profile file and set it current base on the option
	var opts ProfileOptions
	if len(profileOptions) == 0 {
		// default behavior is to override the profile
		opts = ProfileMakeCurrent
	} else {
		for _, flag := range profileOptions {
			opts |= flag
		}
	}
	if err := cp.SaveTo(ProfileLocation{
		AliasPath: profileAliasPath,
		Path:      profilePath,
		Options:   opts,
	}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ParseProxyHost parses the proxyHost string and updates the config.
//
// Format of proxyHost string:
//   proxy_web_addr:<proxy_web_port>,<proxy_ssh_port>
func (c *Config) ParseProxyHost(proxyHost string) error {
	host, port, err := net.SplitHostPort(proxyHost)
	if err != nil {
		host = proxyHost
		port = ""
	}

	// Split on comma.
	parts := strings.Split(port, ",")

	switch {
	// Default ports for both the SSH and Web proxy.
	case len(parts) == 0:
		c.WebProxyAddr = net.JoinHostPort(host, strconv.Itoa(defaults.HTTPListenPort))
		c.SSHProxyAddr = net.JoinHostPort(host, strconv.Itoa(defaults.SSHProxyListenPort))
	// User defined HTTP proxy port, default SSH proxy port.
	case len(parts) == 1:
		webPort := parts[0]
		if webPort == "" {
			webPort = strconv.Itoa(defaults.HTTPListenPort)
		}
		c.WebProxyAddr = net.JoinHostPort(host, webPort)
		c.SSHProxyAddr = net.JoinHostPort(host, strconv.Itoa(defaults.SSHProxyListenPort))
	// User defined HTTP and SSH proxy ports.
	case len(parts) == 2:
		webPort := parts[0]
		if webPort == "" {
			webPort = strconv.Itoa(defaults.HTTPListenPort)
		}
		sshPort := parts[1]
		if sshPort == "" {
			sshPort = strconv.Itoa(defaults.SSHProxyListenPort)
		}
		c.WebProxyAddr = net.JoinHostPort(host, webPort)
		c.SSHProxyAddr = net.JoinHostPort(host, sshPort)
	default:
		return trace.BadParameter("unable to parse port: %v", port)
	}

	return nil
}

// KubeProxyHostPort returns the host and port of the Kubernetes proxy.
func (c *Config) KubeProxyHostPort() (string, int) {
	if c.KubeProxyAddr != "" {
		addr, err := utils.ParseAddr(c.KubeProxyAddr)
		if err == nil {
			return addr.Host(), addr.Port(defaults.KubeProxyListenPort)
		}
	}

	webProxyHost, _ := c.WebProxyHostPort()
	return webProxyHost, defaults.KubeProxyListenPort
}

// KubeClusterAddr returns a public HTTPS address of the proxy for use by
// Kubernetes clients.
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

	webProxyHost, _ := c.WebProxyHostPort()
	return webProxyHost, defaults.HTTPListenPort
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
	lastPing *PingResponse
}

// ShellCreatedCallback can be supplied for every teleport client. It will
// be called right after the remote shell is created, but the session
// hasn't begun yet.
//
// It allows clients to cancel SSH action
type ShellCreatedCallback func(s *ssh.Session, c *ssh.Client, terminal io.ReadWriteCloser) (exit bool, err error)

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
	if c.KeyTTL == 0 {
		c.KeyTTL = defaults.CertDuration
	}
	c.Namespace = services.ProcessNamespace(c.Namespace)

	tc = &TeleportClient{Config: *c}

	if tc.Stdout == nil {
		tc.Stdout = os.Stdout
	}
	if tc.Stderr == nil {
		tc.Stderr = os.Stderr
	}
	if tc.Stdin == nil {
		tc.Stdin = os.Stdin
	}

	// Create a buffered channel to hold events that occurred during this session.
	// This channel must be buffered because the SSH connection directly feeds
	// into it. Delays in pulling messages off the global SSH request channel
	// could lead to the connection hanging.
	tc.eventsCh = make(chan events.EventFields, 1024)

	// sometimes we need to use external auth without using local auth
	// methods, e.g. in automation daemons
	if c.SkipLocalAuth {
		if len(c.AuthMethods) == 0 {
			return nil, trace.BadParameter("SkipLocalAuth is true but no AuthMethods provided")
		}
		// if the client was passed an agent in the configuration and skip local auth, use
		// the passed in agent.
		if c.Agent != nil {
			tc.localAgent = &LocalKeyAgent{Agent: c.Agent}
		}
	} else {
		// initialize the local agent (auth agent which uses local SSH keys signed by the CA):
		webProxyHost, _ := tc.WebProxyHostPort()
		tc.localAgent, err = NewLocalAgent(c.KeysDir, webProxyHost, c.Username, c.UseLocalSSHAgent)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if tc.HostKeyCallback == nil {
			tc.HostKeyCallback = tc.localAgent.CheckHostSignature
		}
	}

	return tc, nil
}

// accessPoint returns access point based on the cache policy
func (tc *TeleportClient) accessPoint(clt auth.AccessPoint, proxyHostPort string, clusterName string) (auth.AccessPoint, error) {
	// If no caching policy was set or on Windows (where Teleport does not
	// support file locking at the moment), return direct access to the access
	// point.
	if tc.CachePolicy == nil || runtime.GOOS == teleport.WindowsOS {
		log.Debugf("not using caching access point")
		return clt, nil
	}
	return clt, nil
}

// LocalAgent is a getter function for the client's local agent
func (tc *TeleportClient) LocalAgent() *LocalKeyAgent {
	return tc.localAgent
}

// getTargetNodes returns a list of node addresses this SSH command needs to
// operate on.
func (tc *TeleportClient) getTargetNodes(ctx context.Context, proxy *ProxyClient) ([]string, error) {
	var (
		err    error
		nodes  []services.Server
		retval = make([]string, 0)
	)
	if tc.Labels != nil && len(tc.Labels) > 0 {
		nodes, err = proxy.FindServersByLabels(ctx, tc.Namespace, tc.Labels)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for i := 0; i < len(nodes); i++ {
			addr := nodes[i].GetAddr()
			if addr == "" {
				// address is empty, try dialing by UUID instead.
				addr = fmt.Sprintf("%s:0", nodes[i].GetName())
			}
			retval = append(retval, addr)
		}
	}
	if len(nodes) == 0 {
		// detect the common error when users use host:port address format
		_, port, err := net.SplitHostPort(tc.Host)
		// client has used host:port notation
		if err == nil {
			return nil, trace.BadParameter(
				"please use ssh subcommand with '--port=%v' flag instead of semicolon",
				port)
		}
		addr := net.JoinHostPort(tc.Host, strconv.Itoa(tc.HostPort))
		retval = append(retval, addr)
	}
	return retval, nil
}

// GenerateCertsForCluster generates certificates for the user
// that have a metadata instructing server to route the requests to the cluster
func (tc *TeleportClient) GenerateCertsForCluster(ctx context.Context, routeToCluster string) error {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return proxyClient.GenerateCertsForCluster(ctx, routeToCluster)
}

func (tc *TeleportClient) ReissueUserCerts(ctx context.Context, params ReissueParams) error {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return proxyClient.ReissueUserCerts(ctx, params)
}

// CreateAccessRequest registers a new access request with the auth server.
func (tc *TeleportClient) CreateAccessRequest(ctx context.Context, req services.AccessRequest) error {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return proxyClient.CreateAccessRequest(ctx, req)
}

// GetAccessRequests loads all access requests matching the supplied filter.
func (tc *TeleportClient) GetAccessRequests(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return proxyClient.GetAccessRequests(ctx, filter)
}

// NewWatcher sets up a new event watcher.
func (tc *TeleportClient) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return proxyClient.NewWatcher(ctx, watch)
}

// SSH connects to a node and, if 'command' is specified, executes the command on it,
// otherwise runs interactive shell
//
// Returns nil if successful, or (possibly) *exec.ExitError
func (tc *TeleportClient) SSH(ctx context.Context, command []string, runLocally bool) error {
	// connect to proxy first:
	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()
	siteInfo, err := proxyClient.currentCluster()
	if err != nil {
		return trace.Wrap(err)
	}
	// which nodes are we executing this commands on?
	nodeAddrs, err := tc.getTargetNodes(ctx, proxyClient)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(nodeAddrs) == 0 {
		return trace.BadParameter("no target host specified")
	}
	nodeClient, err := proxyClient.ConnectToNode(
		ctx,
		NodeAddr{Addr: nodeAddrs[0], Namespace: tc.Namespace, Cluster: siteInfo.Name},
		tc.Config.HostLogin,
		false)
	if err != nil {
		tc.ExitStatus = 1
		return trace.Wrap(err)
	}

	// If forwarding ports were specified, start port forwarding.
	tc.startPortForwarding(ctx, nodeClient)

	// If no remote command execution was requested, block on the context which
	// will unblock upon error or SIGINT.
	if tc.NoRemoteExec {
		log.Debugf("Connected to node, no remote command execution was requested, blocking until context closes.")
		<-ctx.Done()

		// Only return an error if the context was canceled by something other than SIGINT.
		if ctx.Err() != context.Canceled {
			return ctx.Err()
		}
		return nil
	}

	// After port forwarding, run a local command that uses the connection, and
	// then disconnect.
	if runLocally {
		if len(tc.Config.LocalForwardPorts) == 0 {
			fmt.Println("Executing command locally without connecting to any servers. This makes no sense.")
		}
		return runLocalCommand(command)
	}

	// Issue "exec" request(s) to run on remote node(s).
	if len(command) > 0 {
		if len(nodeAddrs) > 1 {
			fmt.Printf("\x1b[1mWARNING\x1b[0m: Multiple nodes matched label selector, running command on all.")
		}
		return tc.runCommand(ctx, siteInfo.Name, nodeAddrs, proxyClient, command)
	}

	// Issue "shell" request to run single node.
	if len(nodeAddrs) > 1 {
		fmt.Printf("\x1b[1mWARNING\x1b[0m: Multiple nodes match the label selector, picking first: %v\n", nodeAddrs[0])
	}
	return tc.runShell(nodeClient, nil)
}

func (tc *TeleportClient) startPortForwarding(ctx context.Context, nodeClient *NodeClient) {
	if len(tc.Config.LocalForwardPorts) > 0 {
		for _, fp := range tc.Config.LocalForwardPorts {
			addr := net.JoinHostPort(fp.SrcIP, strconv.Itoa(fp.SrcPort))
			socket, err := net.Listen("tcp", addr)
			if err != nil {
				log.Errorf("Failed to bind to %v: %v.", addr, err)
				continue
			}
			go nodeClient.listenAndForward(ctx, socket, net.JoinHostPort(fp.DestHost, strconv.Itoa(fp.DestPort)))
		}
	}
	if len(tc.Config.DynamicForwardedPorts) > 0 {
		for _, fp := range tc.Config.DynamicForwardedPorts {
			addr := net.JoinHostPort(fp.SrcIP, strconv.Itoa(fp.SrcPort))
			socket, err := net.Listen("tcp", addr)
			if err != nil {
				log.Errorf("Failed to bind to %v: %v.", addr, err)
				continue
			}
			go nodeClient.dynamicListenAndForward(ctx, socket)
		}
	}
}

// Join connects to the existing/active SSH session
func (tc *TeleportClient) Join(ctx context.Context, namespace string, sessionID session.ID, input io.Reader) (err error) {
	if namespace == "" {
		return trace.BadParameter(auth.MissingNamespaceError)
	}
	tc.Stdin = input
	if sessionID.Check() != nil {
		return trace.Errorf("Invalid session ID format: %s", string(sessionID))
	}
	var notFoundErrorMessage = fmt.Sprintf("session '%s' not found or it has ended", sessionID)

	// connect to proxy:
	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()
	site, err := proxyClient.ConnectToCurrentCluster(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	// find the session ID on the site:
	sessions, err := site.GetSessions(namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	var session *session.Session
	for _, s := range sessions {
		if s.ID == sessionID {
			session = &s
			break
		}
	}
	if session == nil {
		return trace.NotFound(notFoundErrorMessage)
	}

	// pick the 1st party of the session and use his server ID to connect to
	if len(session.Parties) == 0 {
		return trace.NotFound(notFoundErrorMessage)
	}
	serverID := session.Parties[0].ServerID

	// find a server address by its ID
	nodes, err := site.GetNodes(namespace, services.SkipValidation())
	if err != nil {
		return trace.Wrap(err)
	}
	var node services.Server
	for _, n := range nodes {
		if n.GetName() == serverID {
			node = n
			break
		}
	}
	if node == nil {
		return trace.NotFound(notFoundErrorMessage)
	}
	target := node.GetAddr()
	if target == "" {
		// address is empty, try dialing by UUID instead
		target = fmt.Sprintf("%s:0", serverID)
	}
	// connect to server:
	nc, err := proxyClient.ConnectToNode(ctx, NodeAddr{
		Addr:      target,
		Namespace: tc.Namespace,
		Cluster:   tc.SiteName,
	}, tc.Config.HostLogin, false)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nc.Close()

	// Start forwarding ports if configured.
	tc.startPortForwarding(ctx, nc)

	// running shell with a given session means "join" it:
	return tc.runShell(nc, session)
}

// Play replays the recorded session
func (tc *TeleportClient) Play(ctx context.Context, namespace, sessionID string) (err error) {
	if namespace == "" {
		return trace.BadParameter(auth.MissingNamespaceError)
	}
	sid, err := session.ParseID(sessionID)
	if err != nil {
		return fmt.Errorf("'%v' is not a valid session ID (must be GUID)", sid)
	}
	// connect to the auth server (site) who made the recording
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	site, err := proxyClient.ConnectToCurrentCluster(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// request events for that session (to get timing data)
	sessionEvents, err := site.GetSessionEvents(namespace, *sid, 0, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// read the stream into a buffer:
	var stream []byte
	for {
		tmp, err := site.GetSessionChunk(namespace, *sid, len(stream), events.MaxChunkBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(tmp) == 0 {
			break
		}
		stream = append(stream, tmp...)
	}

	// configure terminal for direct unbuffered echo-less input:
	if term.IsTerminal(0) {
		state, err := term.SetRawTerminal(0)
		if err != nil {
			return nil
		}
		defer term.RestoreTerminal(0, state)
	}
	player := newSessionPlayer(sessionEvents, stream)
	// keys:
	const (
		keyCtrlC = 3
		keyCtrlD = 4
		keySpace = 32
		keyLeft  = 68
		keyRight = 67
		keyUp    = 65
		keyDown  = 66
	)
	// playback control goroutine
	go func() {
		defer player.Stop()
		key := make([]byte, 1)
		for {
			_, err = os.Stdin.Read(key)
			if err != nil {
				return
			}
			switch key[0] {
			// Ctrl+C or Ctrl+D
			case keyCtrlC, keyCtrlD:
				return
			// Space key
			case keySpace:
				player.TogglePause()
			// <- arrow
			case keyLeft, keyDown:
				player.Rewind()
			// -> arrow
			case keyRight, keyUp:
				player.Forward()
			}
		}
	}()

	// player starts playing in its own goroutine
	player.Play()

	// wait for keypresses loop to end
	<-player.stopC
	fmt.Println("\n\nend of session playback")
	return trace.Wrap(err)
}

// ExecuteSCP executes SCP command. It executes scp.Command using
// lower-level API integrations that mimic SCP CLI command behavior
func (tc *TeleportClient) ExecuteSCP(ctx context.Context, cmd scp.Command) (err error) {
	// connect to proxy first:
	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}

	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	clusterInfo, err := proxyClient.currentCluster()
	if err != nil {
		return trace.Wrap(err)
	}

	// which nodes are we executing this commands on?
	nodeAddrs, err := tc.getTargetNodes(ctx, proxyClient)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(nodeAddrs) == 0 {
		return trace.BadParameter("no target host specified")
	}

	nodeClient, err := proxyClient.ConnectToNode(
		ctx,
		NodeAddr{Addr: nodeAddrs[0], Namespace: tc.Namespace, Cluster: clusterInfo.Name},
		tc.Config.HostLogin,
		false)
	if err != nil {
		tc.ExitStatus = 1
		return trace.Wrap(err)
	}

	err = nodeClient.ExecuteSCP(cmd)
	if err != nil {
		// converts SSH error code to tc.ExitStatus
		exitError, _ := trace.Unwrap(err).(*ssh.ExitError)
		if exitError != nil {
			tc.ExitStatus = exitError.ExitStatus()
		}
		return err

	}

	return nil
}

// SCP securely copies file(s) from one SSH server to another
func (tc *TeleportClient) SCP(ctx context.Context, args []string, port int, recursive bool, quiet bool) (err error) {
	if len(args) < 2 {
		return trace.Errorf("Need at least two arguments for scp")
	}
	first := args[0]
	last := args[len(args)-1]

	// local copy?
	if !isRemoteDest(first) && !isRemoteDest(last) {
		return trace.BadParameter("making local copies is not supported")
	}

	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}
	log.Infof("Connecting to proxy to copy (recursively=%v)...", recursive)
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	// helper function connects to the src/target node:
	connectToNode := func(addr string) (*NodeClient, error) {
		// determine which cluster we're connecting to:
		siteInfo, err := proxyClient.currentCluster()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return proxyClient.ConnectToNode(ctx,
			NodeAddr{Addr: addr, Namespace: tc.Namespace, Cluster: siteInfo.Name},
			tc.HostLogin, false)
	}

	var progressWriter io.Writer
	if !quiet {
		progressWriter = tc.Stdout
	}

	// gets called to convert SSH error code to tc.ExitStatus
	onError := func(err error) error {
		exitError, _ := trace.Unwrap(err).(*ssh.ExitError)
		if exitError != nil {
			tc.ExitStatus = exitError.ExitStatus()
		}
		return err
	}
	// upload:
	if isRemoteDest(last) {
		filesToUpload := args[:len(args)-1]

		// If more than a single file were provided, scp must be in directory mode
		// and the target on the remote host needs to be a directory.
		var directoryMode bool
		if len(filesToUpload) > 1 {
			directoryMode = true
		}

		dest, err := scp.ParseSCPDestination(last)
		if err != nil {
			return trace.Wrap(err)
		}
		if dest.Login != "" {
			tc.HostLogin = dest.Login
		}
		addr := net.JoinHostPort(dest.Host.Host(), strconv.Itoa(port))

		client, err := connectToNode(addr)
		if err != nil {
			return trace.Wrap(err)
		}

		// copy everything except the last arg (that's destination)
		for _, src := range filesToUpload {
			scpConfig := scp.Config{
				User:           tc.Username,
				ProgressWriter: progressWriter,
				RemoteLocation: dest.Path,
				Flags: scp.Flags{
					Target:        []string{src},
					Recursive:     recursive,
					DirectoryMode: directoryMode,
				},
			}

			cmd, err := scp.CreateUploadCommand(scpConfig)
			if err != nil {
				return trace.Wrap(err)
			}

			err = client.ExecuteSCP(cmd)
			if err != nil {
				return onError(err)
			}
		}
		// download:
	} else {
		src, err := scp.ParseSCPDestination(first)
		if err != nil {
			return trace.Wrap(err)
		}
		addr := net.JoinHostPort(src.Host.Host(), strconv.Itoa(port))
		if src.Login != "" {
			tc.HostLogin = src.Login
		}
		client, err := connectToNode(addr)
		if err != nil {
			return trace.Wrap(err)
		}
		// copy everything except the last arg (that's destination)
		for _, dest := range args[1:] {
			scpConfig := scp.Config{
				User: tc.Username,
				Flags: scp.Flags{
					Recursive: recursive,
					Target:    []string{dest},
				},
				RemoteLocation: src.Path,
				ProgressWriter: progressWriter,
			}

			cmd, err := scp.CreateDownloadCommand(scpConfig)
			if err != nil {
				return trace.Wrap(err)
			}

			err = client.ExecuteSCP(cmd)
			if err != nil {
				return onError(err)
			}
		}
	}
	return nil
}

func isRemoteDest(name string) bool {
	return strings.ContainsRune(name, ':')
}

// ListNodes returns a list of nodes connected to a proxy
func (tc *TeleportClient) ListNodes(ctx context.Context) ([]services.Server, error) {
	var err error
	// userhost is specified? that must be labels
	if tc.Host != "" {
		tc.Labels, err = ParseLabelSpec(tc.Host)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// connect to the proxy and ask it to return a full list of servers
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	return proxyClient.FindServersByLabels(ctx, tc.Namespace, tc.Labels)
}

// ListAllNodes is the same as ListNodes except that it ignores labels.
func (tc *TeleportClient) ListAllNodes(ctx context.Context) ([]services.Server, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	return proxyClient.FindServersByLabels(ctx, tc.Namespace, nil)
}

// runCommand executes a given bash command on a bunch of remote nodes
func (tc *TeleportClient) runCommand(
	ctx context.Context, siteName string, nodeAddresses []string, proxyClient *ProxyClient, command []string) error {

	resultsC := make(chan error, len(nodeAddresses))
	for _, address := range nodeAddresses {
		go func(address string) {
			var (
				err         error
				nodeSession *NodeSession
			)
			defer func() {
				resultsC <- err
			}()
			var nodeClient *NodeClient
			nodeClient, err = proxyClient.ConnectToNode(ctx,
				NodeAddr{Addr: address, Namespace: tc.Namespace, Cluster: siteName},
				tc.Config.HostLogin, false)
			if err != nil {
				fmt.Fprintln(tc.Stderr, err)
				return
			}
			defer nodeClient.Close()

			// run the command on one node:
			if len(nodeAddresses) > 1 {
				fmt.Printf("Running command on %v:\n", address)
			}
			nodeSession, err = newSession(nodeClient, nil, tc.Config.Env, tc.Stdin, tc.Stdout, tc.Stderr, tc.useLegacyID(nodeClient), tc.EnableEscapeSequences)
			if err != nil {
				log.Error(err)
				return
			}
			defer nodeSession.Close()
			if err = nodeSession.runCommand(ctx, command, tc.OnShellCreated, tc.Config.Interactive); err != nil {
				originErr := trace.Unwrap(err)
				exitErr, ok := originErr.(*ssh.ExitError)
				if ok {
					tc.ExitStatus = exitErr.ExitStatus()
				} else {
					// if an error occurs, but no exit status is passed back, GoSSH returns
					// a generic error like this. in this case the error message is printed
					// to stderr by the remote process so we have to quietly return 1:
					if strings.Contains(originErr.Error(), "exited without exit status") {
						tc.ExitStatus = 1
					}
				}
			}
		}(address)
	}
	var lastError error
	for range nodeAddresses {
		if err := <-resultsC; err != nil {
			lastError = err
		}
	}
	return trace.Wrap(lastError)
}

// runShell starts an interactive SSH session/shell.
// sessionID : when empty, creates a new shell. otherwise it tries to join the existing session.
func (tc *TeleportClient) runShell(nodeClient *NodeClient, sessToJoin *session.Session) error {
	nodeSession, err := newSession(nodeClient, sessToJoin, tc.Env, tc.Stdin, tc.Stdout, tc.Stderr, tc.useLegacyID(nodeClient), tc.EnableEscapeSequences)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = nodeSession.runShell(tc.OnShellCreated); err != nil {
		return trace.Wrap(err)
	}
	if nodeSession.ExitMsg == "" {
		fmt.Fprintln(tc.Stderr, "the connection was closed on the remote side on ", time.Now().Format(time.RFC822))
	} else {
		fmt.Fprintln(tc.Stderr, nodeSession.ExitMsg)
	}
	return nil
}

// getProxyLogin determines which SSH principal to use when connecting to proxy.
func (tc *TeleportClient) getProxySSHPrincipal() string {
	proxyPrincipal := tc.Config.HostLogin
	if tc.DefaultPrincipal != "" {
		proxyPrincipal = tc.DefaultPrincipal
	}
	if len(tc.JumpHosts) > 1 && tc.JumpHosts[0].Username != "" {
		log.Debugf("Setting proxy login to jump host's parameter user %q", tc.JumpHosts[0].Username)
		proxyPrincipal = tc.JumpHosts[0].Username
	}
	// see if we already have a signed key in the cache, we'll use that instead
	if !tc.Config.SkipLocalAuth && tc.localAgent != nil {
		signers, err := tc.localAgent.Signers()
		if err != nil || len(signers) == 0 {
			return proxyPrincipal
		}
		cert, ok := signers[0].PublicKey().(*ssh.Certificate)
		if ok && len(cert.ValidPrincipals) > 0 {
			return cert.ValidPrincipals[0]
		}
	}
	return proxyPrincipal
}

// authMethods returns a list (slice) of all SSH auth methods this client
// can use to try to authenticate
func (tc *TeleportClient) authMethods() []ssh.AuthMethod {
	m := append([]ssh.AuthMethod(nil), tc.Config.AuthMethods...)
	if tc.localAgent != nil {
		m = append(m, tc.localAgent.AuthMethods()...)
	}
	return m
}

// ConnectToProxy will dial to the proxy server and return a ProxyClient when
// successful. If the passed in context is canceled, this function will return
// a trace.ConnectionProblem right away.
func (tc *TeleportClient) ConnectToProxy(ctx context.Context) (*ProxyClient, error) {
	var err error
	var proxyClient *ProxyClient

	// Use connectContext and the cancel function to signal when a response is
	// returned from connectToProxy.
	connectContext, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		proxyClient, err = tc.connectToProxy(ctx)
	}()

	select {
	// ConnectToProxy returned a result, return that back to the caller.
	case <-connectContext.Done():
		return proxyClient, trace.Wrap(err)
	// The passed in context timed out. This is often due to the network being
	// down and the user hitting Ctrl-C.
	case <-ctx.Done():
		return nil, trace.ConnectionProblem(ctx.Err(), "connection canceled")
	}
}

// connectToProxy will dial to the proxy server and return a ProxyClient when
// successful.
func (tc *TeleportClient) connectToProxy(ctx context.Context) (*ProxyClient, error) {
	proxyPrincipal := tc.getProxySSHPrincipal()
	sshConfig := &ssh.ClientConfig{
		User:            proxyPrincipal,
		HostKeyCallback: tc.HostKeyCallback,
	}

	sshProxyAddr := tc.Config.SSHProxyAddr
	if len(tc.JumpHosts) > 0 {
		log.Debugf("Overriding SSH proxy to JumpHosts's address %q", tc.JumpHosts[0].Addr.String())
		sshProxyAddr = tc.JumpHosts[0].Addr.Addr
	}

	// helper to create a ProxyClient struct
	makeProxyClient := func(sshClient *ssh.Client, m ssh.AuthMethod) *ProxyClient {
		return &ProxyClient{
			teleportClient:  tc,
			Client:          sshClient,
			proxyAddress:    sshProxyAddr,
			proxyPrincipal:  proxyPrincipal,
			hostKeyCallback: sshConfig.HostKeyCallback,
			authMethod:      m,
			hostLogin:       tc.Config.HostLogin,
			siteName:        tc.Config.SiteName,
			clientAddr:      tc.ClientAddr,
		}
	}
	successMsg := fmt.Sprintf("Successful auth with proxy %v", sshProxyAddr)
	var err error
	// try to authenticate using every non interactive auth method we have:
	for i, m := range tc.authMethods() {
		log.Infof("Connecting proxy=%v login='%v' method=%d", sshProxyAddr, sshConfig.User, i)
		var sshClient *ssh.Client

		sshConfig.Auth = []ssh.AuthMethod{m}
		sshClient, err = ssh.Dial("tcp", sshProxyAddr, sshConfig)
		if err != nil {
			log.Warningf("Failed to authenticate with proxy: %v", err)
			err = trace.BadParameter("failed to authenticate with proxy %v: %v", sshProxyAddr, err)
			continue
		}
		log.Infof(successMsg)
		return makeProxyClient(sshClient, m), nil
	}
	// we have exhausted all auth existing auth methods and local login
	// is disabled in configuration, or the user refused connecting to untrusted hosts
	if err == nil {
		err = trace.BadParameter("failed to authenticate with proxy %v", tc.Config.SSHProxyAddr)
	}
	return nil, trace.Wrap(err)
}

// Logout removes certificate and key for the currently logged in user from
// the filesystem and agent.
func (tc *TeleportClient) Logout() error {
	if tc.localAgent == nil {
		return nil
	}
	if err := tc.localAgent.DeleteKey(); err != nil {
		return trace.Wrap(err)
	}

	return nil
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

// Login logs the user into a Teleport cluster by talking to a Teleport proxy.
//
// If 'activateKey' is true, saves the received session cert into the local
// keystore (and into the ssh-agent) for future use.
//
func (tc *TeleportClient) Login(ctx context.Context, activateKey bool) (*Key, error) {
	// preserve original web proxy host that could have
	webProxyHost, _ := tc.WebProxyHostPort()

	// Ping the endpoint to see if it's up and find the type of authentication
	// supported.
	pr, err := tc.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate a new keypair. the public key will be signed via proxy if client's
	// password+OTP are valid
	key, err := NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var response *auth.SSHLoginResponse

	switch pr.Auth.Type {
	case teleport.Local:
		response, err = tc.localLogin(ctx, pr.Auth.SecondFactor, key.Pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.OIDC:
		response, err = tc.ssoLogin(ctx, pr.Auth.OIDC.Name, key.Pub, teleport.OIDC)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// in this case identity is returned by the proxy
		tc.Username = response.Username
		if tc.localAgent != nil {
			tc.localAgent.username = response.Username
		}
	case teleport.SAML:
		response, err = tc.ssoLogin(ctx, pr.Auth.SAML.Name, key.Pub, teleport.SAML)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// in this case identity is returned by the proxy
		tc.Username = response.Username
		if tc.localAgent != nil {
			tc.localAgent.username = response.Username
		}
	case teleport.Github:
		response, err = tc.ssoLogin(ctx, pr.Auth.Github.Name, key.Pub, teleport.Github)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// in this case identity is returned by the proxy
		tc.Username = response.Username
		if tc.localAgent != nil {
			tc.localAgent.username = response.Username
		}
	default:
		return nil, trace.BadParameter("unsupported authentication type: %q", pr.Auth.Type)
	}

	// extract the new certificate out of the response
	key.Cert = response.Cert
	key.TLSCert = response.TLSCert
	key.ProxyHost = webProxyHost
	key.TrustedCA = response.HostSigners

	// Check that a host certificate for at least one cluster was returned and
	// extract the name of the current cluster from the first host certificate.
	if len(response.HostSigners) <= 0 {
		return nil, trace.BadParameter("bad response from the server: expected at least one certificate, got 0")
	}

	// Add the cluster name into the key from the host certificate.
	key.ClusterName = response.HostSigners[0].ClusterName

	if activateKey && tc.localAgent != nil {
		// save the list of CAs client trusts to ~/.tsh/known_hosts
		err = tc.localAgent.AddHostSignersToCache(response.HostSigners)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// save the list of TLS CAs client trusts
		err = tc.localAgent.SaveCerts(response.HostSigners)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// save the cert to the local storage (~/.tsh usually):
		_, err = tc.localAgent.AddKey(key)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Connect to the Auth Server of the main cluster and fetch the known hosts
		// for this cluster.
		if err := tc.UpdateTrustedCA(ctx, key.ClusterName); err != nil {
			return nil, trace.Wrap(err)
		}

		// Update the cluster name (which will be saved in the profile) with the
		// name of the cluster the caller requested to connect to.
		tc.SiteName, err = updateClusterName(ctx, tc, tc.SiteName, response.HostSigners)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return key, nil
}

// Ping makes a ping request to the proxy, and updates tc based on the
// response. The successful ping response is cached, multiple calls to Ping
// will return the original response and skip the round-trip.
//
// Ping can be called for its side-effect of applying the proxy-provided
// settings (such as various listening addresses).
func (tc *TeleportClient) Ping(ctx context.Context) (*PingResponse, error) {
	// If, at some point, there's a need to bypass this caching, consider
	// adding a bool argument. At the time of writing this we always want to
	// cache.
	if tc.lastPing != nil {
		return tc.lastPing, nil
	}
	pr, err := Ping(
		ctx,
		tc.WebProxyAddr,
		tc.InsecureSkipVerify,
		loopbackPool(tc.WebProxyAddr),
		tc.AuthConnector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If version checking was requested and the server advertises a minimum version.
	if tc.CheckVersions && pr.MinClientVersion != "" {
		if err := utils.CheckVersions(teleport.Version, pr.MinClientVersion); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Update tc with proxy settings specified in Ping response.
	if err := tc.applyProxySettings(pr.Proxy); err != nil {
		return nil, trace.Wrap(err)
	}

	tc.lastPing = pr

	return pr, nil
}

// certGetter is used in updateClusterName to control the response of
// GetTrustedCA in testing.
type certGetter interface {
	// GetTrustedCA returns a list of trusted clusters.
	GetTrustedCA(context.Context, string) ([]services.CertAuthority, error)
}

// updateClusterName returns the name of the cluster the user is connected to.
func updateClusterName(ctx context.Context, certGetter certGetter, clusterName string, certificates []auth.TrustedCerts) (string, error) {
	// Extract the name of the cluster the caller actually connected to.
	if len(certificates) == 0 {
		return "", trace.BadParameter("missing host certificates")
	}
	certificateClusterName := certificates[0].ClusterName

	// The caller did not specify a cluster name, for example "tsh login", or
	// requested the same name that is on the host certificate. In this case
	// return the cluster name on the host certificate returned.
	if clusterName == "" || clusterName == certificateClusterName {
		return certificateClusterName, nil
	}

	// If the caller requested login to a leaf cluster, make sure the cluster
	// exists.
	leafClusters, err := certGetter.GetTrustedCA(ctx, certificateClusterName)
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, leafCluster := range leafClusters {
		if leafCluster.GetClusterName() == clusterName {
			return clusterName, nil
		}
	}
	return "", trace.BadParameter(`unknown cluster: %q, run "tsh clusters" for a list of clusters`, clusterName)
}

// GetTrustedCA returns a list of host certificate authorities
// trusted by the cluster client is authenticated with.
func (tc *TeleportClient) GetTrustedCA(ctx context.Context, clusterName string) ([]services.CertAuthority, error) {
	// Connect to the proxy.
	if !tc.Config.ProxySpecified() {
		return nil, trace.BadParameter("proxy server is not specified")
	}
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	// Get a client to the Auth Server.
	clt, err := proxyClient.ClusterAccessPoint(ctx, clusterName, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the list of host certificates that this cluster knows about.
	return clt.GetCertAuthorities(services.HostCA, false)
}

// UpdateTrustedCA connects to the Auth Server and fetches all host certificates
// and updates ~/.tsh/keys/proxy/certs.pem and ~/.tsh/known_hosts.
func (tc *TeleportClient) UpdateTrustedCA(ctx context.Context, clusterName string) error {
	if tc.localAgent == nil {
		return trace.BadParameter("TeleportClient.UpdateTrustedCA called on a client without localAgent")
	}
	// Get the list of host certificates that this cluster knows about.
	hostCerts, err := tc.GetTrustedCA(ctx, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCerts := auth.AuthoritiesToTrustedCerts(hostCerts)

	// Update the ~/.tsh/known_hosts file to include all the CA the cluster
	// knows about.
	err = tc.localAgent.AddHostSignersToCache(trustedCerts)
	if err != nil {
		return trace.Wrap(err)
	}

	// Update the CA pool with all the CA the cluster knows about.
	err = tc.localAgent.SaveCerts(trustedCerts)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// applyProxySettings updates configuration changes based on the advertised
// proxy settings, overriding existing fields in tc.
func (tc *TeleportClient) applyProxySettings(proxySettings ProxySettings) error {
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
		// ListenAddr is the second preference.
		case proxySettings.Kube.ListenAddr != "":
			if _, err := utils.ParseAddr(proxySettings.Kube.ListenAddr); err != nil {
				return trace.BadParameter(
					"failed to parse value received from the server: %q, contact your administrator for help",
					proxySettings.Kube.ListenAddr)
			}
			tc.KubeProxyAddr = proxySettings.Kube.ListenAddr
		// If neither PublicAddr nor ListenAddr are passed, use the web
		// interface hostname with default k8s port as a guess.
		default:
			webProxyHost, _ := tc.WebProxyHostPort()
			tc.KubeProxyAddr = fmt.Sprintf("%s:%d", webProxyHost, defaults.KubeProxyListenPort)
		}
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

	return nil
}

func (tc *TeleportClient) localLogin(ctx context.Context, secondFactor string, pub []byte) (*auth.SSHLoginResponse, error) {
	var err error
	var response *auth.SSHLoginResponse

	switch secondFactor {
	case teleport.OFF, teleport.OTP, teleport.TOTP, teleport.HOTP:
		response, err = tc.directLogin(ctx, secondFactor, pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.U2F:
		response, err = tc.u2fLogin(ctx, pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unsupported second factor type: %q", secondFactor)
	}

	return response, nil
}

// AddTrustedCA adds a new CA as trusted CA for this client, used in tests
func (tc *TeleportClient) AddTrustedCA(ca services.CertAuthority) error {
	if tc.localAgent == nil {
		return trace.BadParameter("TeleportClient.AddTrustedCA called on a client without localAgent")
	}
	err := tc.localAgent.AddHostSignersToCache(auth.AuthoritiesToTrustedCerts([]services.CertAuthority{ca}))
	if err != nil {
		return trace.Wrap(err)
	}

	// only host CA has TLS certificates, user CA will overwrite trusted certs
	// to empty file if called
	if ca.GetType() == services.HostCA {
		err = tc.localAgent.SaveCerts(auth.AuthoritiesToTrustedCerts([]services.CertAuthority{ca}))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// AddKey adds a key to the client's local agent, used in tests.
func (tc *TeleportClient) AddKey(host string, key *Key) (*agent.AddedKey, error) {
	if tc.localAgent == nil {
		return nil, trace.BadParameter("TeleportClient.AddKey called on a client without localAgent")
	}
	return tc.localAgent.AddKey(key)
}

// directLogin asks for a password + HOTP token, makes a request to CA via proxy
func (tc *TeleportClient) directLogin(ctx context.Context, secondFactorType string, pub []byte) (*auth.SSHLoginResponse, error) {
	var err error

	var password string
	var otpToken string

	password, err = tc.AskPassword()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// only ask for a second factor if it's enabled
	if secondFactorType != teleport.OFF {
		otpToken, err = tc.AskOTP()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// ask the CA (via proxy) to sign our public key:
	response, err := SSHAgentLogin(ctx, SSHLoginDirect{
		SSHLogin: SSHLogin{
			ProxyAddr:      tc.WebProxyAddr,
			PubKey:         pub,
			TTL:            tc.KeyTTL,
			Insecure:       tc.InsecureSkipVerify,
			Pool:           loopbackPool(tc.WebProxyAddr),
			Compatibility:  tc.CertificateFormat,
			RouteToCluster: tc.SiteName,
		},
		User:     tc.Config.Username,
		Password: password,
		OTPToken: otpToken,
	})

	return response, trace.Wrap(err)
}

// samlLogin opens browser window and uses OIDC or SAML redirect cycle with browser
func (tc *TeleportClient) ssoLogin(ctx context.Context, connectorID string, pub []byte, protocol string) (*auth.SSHLoginResponse, error) {
	log.Debugf("samlLogin start")
	// ask the CA (via proxy) to sign our public key:
	response, err := SSHAgentSSOLogin(ctx, SSHLoginSSO{
		SSHLogin: SSHLogin{
			ProxyAddr:      tc.WebProxyAddr,
			PubKey:         pub,
			TTL:            tc.KeyTTL,
			Insecure:       tc.InsecureSkipVerify,
			Pool:           loopbackPool(tc.WebProxyAddr),
			Compatibility:  tc.CertificateFormat,
			RouteToCluster: tc.SiteName,
		},
		ConnectorID: connectorID,
		Protocol:    protocol,
		BindAddr:    tc.BindAddr,
		Browser:     tc.Browser,
	})
	return response, trace.Wrap(err)
}

// directLogin asks for a password and performs the challenge-response authentication
func (tc *TeleportClient) u2fLogin(ctx context.Context, pub []byte) (*auth.SSHLoginResponse, error) {
	// U2F login requires the official u2f-host executable
	_, err := exec.LookPath("u2f-host")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	password, err := tc.AskPassword()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := SSHAgentU2FLogin(ctx, SSHLoginU2F{
		SSHLogin: SSHLogin{
			ProxyAddr:      tc.WebProxyAddr,
			PubKey:         pub,
			TTL:            tc.KeyTTL,
			Insecure:       tc.InsecureSkipVerify,
			Pool:           loopbackPool(tc.WebProxyAddr),
			Compatibility:  tc.CertificateFormat,
			RouteToCluster: tc.SiteName,
		},
		User:     tc.Config.Username,
		Password: password,
	})

	return response, trace.Wrap(err)
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
	if !utils.IsLoopback(proxyAddr) {
		log.Debugf("not using loopback pool for remote proxy addr: %v", proxyAddr)
		return nil
	}
	log.Debugf("attempting to use loopback pool for local proxy addr: %v", proxyAddr)
	certPool := x509.NewCertPool()

	certPath := filepath.Join(defaults.DataDir, defaults.SelfSignedCertPath)
	pemByte, err := ioutil.ReadFile(certPath)
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

// connectToSSHAgent connects to the local SSH agent and returns a agent.Agent.
func connectToSSHAgent() agent.Agent {
	socketPath := os.Getenv(teleport.SSHAuthSock)
	conn, err := agentconn.Dial(socketPath)
	if err != nil {
		log.Errorf("[KEY AGENT] Unable to connect to SSH agent on socket: %q.", socketPath)
		return nil
	}

	log.Infof("[KEY AGENT] Connected to the system agent: %q", socketPath)
	return agent.NewClient(conn)
}

// Username returns the current user's username
func Username() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return u.Username, nil
}

// AskOTP prompts the user to enter the OTP token.
func (tc *TeleportClient) AskOTP() (token string, err error) {
	fmt.Printf("Enter your OTP token:\n")
	token, err = lineFromConsole()
	if err != nil {
		fmt.Fprintln(tc.Stderr, err)
		return "", trace.Wrap(err)
	}
	return token, nil
}

// AskPassword prompts the user to enter the password
func (tc *TeleportClient) AskPassword() (pwd string, err error) {
	fmt.Printf("Enter password for Teleport user %v:\n", tc.Config.Username)
	pwd, err = passwordFromConsole()
	if err != nil {
		fmt.Fprintln(tc.Stderr, err)
		return "", trace.Wrap(err)
	}

	return pwd, nil
}

// DELETE IN: 4.1.0
//
// useLegacyID returns true if an old style (UUIDv1) session ID should be
// generated because the client is talking with a older server.
func (tc *TeleportClient) useLegacyID(nodeClient *NodeClient) bool {
	_, err := tc.getServerVersion(nodeClient)
	return trace.IsNotFound(err)
}

type serverResponse struct {
	version string
	err     error
}

// getServerVersion makes a SSH global request to the server to request the
// version.
func (tc *TeleportClient) getServerVersion(nodeClient *NodeClient) (string, error) {
	responseCh := make(chan serverResponse)

	go func() {
		ok, payload, err := nodeClient.Client.SendRequest(teleport.VersionRequest, true, nil)
		if err != nil {
			responseCh <- serverResponse{err: trace.NotFound(err.Error())}
		} else if !ok {
			responseCh <- serverResponse{err: trace.NotFound("server does not support version request")}
		}
		responseCh <- serverResponse{version: string(payload)}
	}()

	select {
	case resp := <-responseCh:
		if resp.err != nil {
			return "", trace.Wrap(resp.err)
		}
		return resp.version, nil
	case <-time.After(500 * time.Millisecond):
		return "", trace.NotFound("timed out waiting for server response")
	}
}

// passwordFromConsole reads from stdin without echoing typed characters to stdout
func passwordFromConsole() (string, error) {
	// syscall.Stdin is not an int on windows. The linter will complain only on
	// linux where syscall.Stdin is an int.
	//
	// nolint:unconvert
	fd := int(syscall.Stdin)
	state, err := terminal.GetState(fd)

	// intercept Ctr+C and restore terminal
	sigCh := make(chan os.Signal, 1)
	closeCh := make(chan int)
	if err != nil {
		log.Warnf("failed reading terminal state: %v", err)
	} else {
		signal.Notify(sigCh, syscall.SIGINT)
		go func() {
			select {
			case <-sigCh:
				terminal.Restore(fd, state)
				os.Exit(1)
			case <-closeCh:
			}
		}()
	}
	defer func() {
		close(closeCh)
	}()

	bytes, err := terminal.ReadPassword(fd)
	return string(bytes), err
}

// lineFromConsole reads a line from stdin
func lineFromConsole() (string, error) {
	bytes, _, err := bufio.NewReader(os.Stdin).ReadLine()
	return string(bytes), err
}

// ParseLabelSpec parses a string like 'name=value,"long name"="quoted value"` into a map like
// { "name" -> "value", "long name" -> "quoted value" }
func ParseLabelSpec(spec string) (map[string]string, error) {
	tokens := []string{}
	var openQuotes = false
	var tokenStart, assignCount int
	var specLen = len(spec)
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

// Executes the given command on the client machine (localhost). If no command is given,
// executes shell
func runLocalCommand(command []string) error {
	if len(command) == 0 {
		user, err := user.Current()
		if err != nil {
			return trace.Wrap(err)
		}
		shell, err := shell.GetLoginShell(user.Username)
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
	const errTemplate = "Invalid port forwarding spec: '%s'. Could be like `80:remote.host:80`"
	ports = make([]ForwardedPort, len(spec))

	for i, str := range spec {
		parts := strings.Split(str, ":")
		if len(parts) < 3 || len(parts) > 4 {
			return nil, fmt.Errorf(errTemplate, str)
		}
		if len(parts) == 3 {
			parts = append([]string{"127.0.0.1"}, parts...)
		}
		p := &ports[i]
		p.SrcIP = parts[0]
		p.SrcPort, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf(errTemplate, str)
		}
		p.DestHost = parts[2]
		p.DestPort, err = strconv.Atoi(parts[3])
		if err != nil {
			return nil, fmt.Errorf(errTemplate, str)
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
