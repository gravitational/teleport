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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
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
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/shell"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/agentconn"
	"github.com/gravitational/teleport/lib/utils/proxy"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
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

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentClient,
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

	// SkipLocalAuth tells the client to use AuthMethods parameter for authentication and NOT
	// use its own SSH agent or ask user for passwords. This is used by external programs linking
	// against Teleport client and obtaining credentials from elsewhere.
	SkipLocalAuth bool

	// Agent is used when SkipLocalAuth is true
	Agent agent.Agent

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

	// HomePath is where tsh stores profiles
	HomePath string

	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool

	// ExtraProxyHeaders is a collection of http headers to be included in requests to the WebProxy.
	ExtraProxyHeaders map[string]string
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
	}
}

// ProfileStatus combines metadata from the logged in profile and associated
// SSH certificate.
type ProfileStatus struct {
	// Name is the profile name.
	Name string

	// Dir is the directory where profile is located.
	Dir string

	// ProxyURL is the URL the web client is accessible at.
	ProxyURL url.URL

	// Username is the Teleport username.
	Username string

	// Roles is a list of Teleport Roles this user has been assigned.
	Roles []string

	// Logins are the Linux accounts, also known as principals in OpenSSH terminology.
	Logins []string

	// KubeEnabled is true when this profile is configured to connect to a
	// kubernetes cluster.
	KubeEnabled bool

	// KubeUsers are the kubernetes users used by this profile.
	KubeUsers []string

	// KubeGroups are the kubernetes groups used by this profile.
	KubeGroups []string

	// Databases is a list of database services this profile is logged into.
	Databases []tlsca.RouteToDatabase

	// Apps is a list of apps this profile is logged into.
	Apps []tlsca.RouteToApp

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

	// AWSRoleARNs is a list of allowed AWS role ARNs user can assume.
	AWSRolesARNs []string
}

// IsExpired returns true if profile is not expired yet
func (p *ProfileStatus) IsExpired(clock clockwork.Clock) bool {
	return p.ValidUntil.Sub(clock.Now()) <= 0
}

// CACertPath returns path to the CA certificate for this profile.
//
// It's stored in <profile-dir>/keys/<proxy>/certs.pem by default.
func (p *ProfileStatus) CACertPath() string {
	return keypaths.TLSCAsPath(p.Dir, p.Name)
}

// KeyPath returns path to the private key for this profile.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>.
func (p *ProfileStatus) KeyPath() string {
	return keypaths.UserKeyPath(p.Dir, p.Name, p.Username)
}

// DatabaseCertPathForCluster returns path to the specified database access
// certificate for this profile, for the specified cluster.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-db/<cluster>/<name>-x509.pem
//
// If the input cluster name is an empty string, the selected cluster in the
// profile will be used.
func (p *ProfileStatus) DatabaseCertPathForCluster(clusterName string, databaseName string) string {
	if clusterName == "" {
		clusterName = p.Cluster
	}
	return keypaths.DatabaseCertPath(p.Dir, p.Name, p.Username, clusterName, databaseName)
}

// AppCertPath returns path to the specified app access certificate
// for this profile.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-app/<cluster>/<name>-x509.pem
func (p *ProfileStatus) AppCertPath(name string) string {
	return keypaths.AppCertPath(p.Dir, p.Name, p.Username, p.Cluster, name)

}

// KubeConfigPath returns path to the specified kubeconfig for this profile.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-kube/<cluster>/<name>-kubeconfig
func (p *ProfileStatus) KubeConfigPath(name string) string {
	return keypaths.KubeConfigPath(p.Dir, p.Name, p.Username, p.Cluster, name)
}

// DatabaseServices returns a list of database service names for this profile.
func (p *ProfileStatus) DatabaseServices() (result []string) {
	for _, db := range p.Databases {
		result = append(result, db.ServiceName)
	}
	return result
}

// DatabasesForCluster returns a list of databases for this profile, for the
// specified cluster name.
func (p *ProfileStatus) DatabasesForCluster(clusterName string) ([]tlsca.RouteToDatabase, error) {
	if clusterName == "" || clusterName == p.Cluster {
		return p.Databases, nil
	}

	idx := KeyIndex{
		ProxyHost:   p.Name,
		Username:    p.Username,
		ClusterName: clusterName,
	}

	store, err := NewFSLocalKeyStore(p.Dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := store.GetKey(idx, WithDBCerts{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return findActiveDatabases(key)
}

// AppNames returns a list of app names this profile is logged into.
func (p *ProfileStatus) AppNames() (result []string) {
	for _, app := range p.Apps {
		result = append(result, app.Name)
	}
	return result
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
		return trace.Wrap(err)
	}
	// Don't try to login when using an identity file.
	if tc.SkipLocalAuth {
		return trace.Wrap(err)
	}
	log.Debugf("Activating relogin on %v.", err)

	key, err := tc.Login(ctx)
	if err != nil {
		if trace.IsTrustError(err) {
			return trace.Wrap(err, "refusing to connect to untrusted proxy %v without --insecure flag\n", tc.Config.SSHProxyAddr)
		}
		return trace.Wrap(err)
	}
	if err := tc.ActivateKey(ctx, key); err != nil {
		return trace.Wrap(err)
	}
	// Save profile to record proxy credentials
	if err := tc.SaveProfile(tc.HomePath, true); err != nil {
		log.Warningf("Failed to save profile: %v", err)
		return trace.Wrap(err)
	}
	return fn()
}

// readProfile reads in the profile as well as the associated certificate
// and returns a *ProfileStatus which can be used to print the status of the
// profile.
func readProfile(profileDir string, profileName string) (*ProfileStatus, error) {
	var err error

	if profileDir == "" {
		return nil, trace.BadParameter("profileDir cannot be empty")
	}

	// Read in the profile for this proxy.
	profile, err := profile.FromDir(profileDir, profileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Read in the SSH certificate for the user logged into this proxy.
	store, err := NewFSLocalKeyStore(profileDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	idx := KeyIndex{
		ProxyHost:   profile.Name(),
		Username:    profile.Username,
		ClusterName: profile.SiteName,
	}
	key, err := store.GetKey(idx, WithAllCerts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshCert, err := key.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract from the certificate how much longer it will be valid for.
	validUntil := time.Unix(int64(sshCert.ValidBefore), 0)

	// Extract roles from certificate. Note, if the certificate is in old format,
	// this will be empty.
	var roles []string
	rawRoles, ok := sshCert.Extensions[teleport.CertExtensionTeleportRoles]
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
	rawTraits, ok := sshCert.Extensions[teleport.CertExtensionTeleportTraits]
	if ok {
		err = wrappers.UnmarshalTraits([]byte(rawTraits), &traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var activeRequests services.RequestIDs
	rawRequests, ok := sshCert.Extensions[teleport.CertExtensionTeleportActiveRequests]
	if ok {
		if err := activeRequests.Unmarshal([]byte(rawRequests)); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Extract extensions from certificate. This lists the abilities of the
	// certificate (like can the user request a PTY, port forwarding, etc.)
	var extensions []string
	for ext := range sshCert.Extensions {
		if ext == teleport.CertExtensionTeleportRoles ||
			ext == teleport.CertExtensionTeleportTraits ||
			ext == teleport.CertExtensionTeleportRouteToCluster ||
			ext == teleport.CertExtensionTeleportActiveRequests {
			continue
		}
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)

	tlsCert, err := key.TeleportTLSCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsID, err := tlsca.FromSubject(tlsCert.Subject, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databases, err := findActiveDatabases(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
		if tlsID.RouteToApp.PublicAddr != "" {
			apps = append(apps, tlsID.RouteToApp)
		}
	}

	return &ProfileStatus{
		Name: profileName,
		Dir:  profileDir,
		ProxyURL: url.URL{
			Scheme: "https",
			Host:   profile.WebProxyAddr,
		},
		Username:       profile.Username,
		Logins:         sshCert.ValidPrincipals,
		ValidUntil:     validUntil,
		Extensions:     extensions,
		Roles:          roles,
		Cluster:        profile.SiteName,
		Traits:         traits,
		ActiveRequests: activeRequests,
		KubeEnabled:    profile.KubeProxyAddr != "",
		KubeUsers:      tlsID.KubernetesUsers,
		KubeGroups:     tlsID.KubernetesGroups,
		Databases:      databases,
		Apps:           apps,
		AWSRolesARNs:   tlsID.AWSRoleARNs,
	}, nil
}

// StatusCurrent returns the active profile status.
func StatusCurrent(profileDir, proxyHost string) (*ProfileStatus, error) {
	active, _, err := Status(profileDir, proxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if active == nil {
		return nil, trace.NotFound("not logged in")
	}
	return active, nil
}

// StatusFor returns profile for the specified proxy/user.
func StatusFor(profileDir, proxyHost, username string) (*ProfileStatus, error) {
	active, others, err := Status(profileDir, proxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, profile := range append(others, active) {
		if profile != nil && profile.Username == username {
			return profile, nil
		}
	}
	return nil, trace.NotFound("no profile for proxy %v and user %v found",
		proxyHost, username)
}

// Status returns the active profile as well as a list of available profiles.
// If no profile is active, Status returns a nil error and nil profile.
func Status(profileDir, proxyHost string) (*ProfileStatus, []*ProfileStatus, error) {
	var err error
	var profileStatus *ProfileStatus
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
	profileDir = profile.FullProfilePath(profileDir)
	stat, err := os.Stat(profileDir)
	if err != nil {
		log.Debugf("Failed to stat file: %v.", err)
		if os.IsNotExist(err) {
			return nil, nil, trace.NotFound(err.Error())
		} else if os.IsPermission(err) {
			return nil, nil, trace.AccessDenied(err.Error())
		} else {
			return nil, nil, trace.Wrap(err)
		}
	}
	if !stat.IsDir() {
		return nil, nil, trace.BadParameter("profile path not a directory")
	}

	// use proxyHost as default profile name, or the current profile if
	// no proxyHost was supplied.
	profileName := proxyHost
	if profileName == "" {
		profileName, err = profile.GetCurrentProfileName(profileDir)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, nil, trace.NotFound("not logged in")
			}
			return nil, nil, trace.Wrap(err)
		}
	}

	// Read in the target profile first. If readProfile returns trace.NotFound,
	// that means the profile may have been corrupted (for example keys were
	// deleted but profile exists), treat this as the user not being logged in.
	profileStatus, err = readProfile(profileDir, profileName)
	if err != nil {
		log.Debug(err)
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}
		// Make sure the profile is nil, which tsh uses to detect that no
		// active profile exists.
		profileStatus = nil
	}

	// load the rest of the profiles
	profiles, err := profile.ListProfileNames(profileDir)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	for _, name := range profiles {
		if name == profileName {
			// already loaded this one
			continue
		}
		ps, err := readProfile(profileDir, name)
		if err != nil {
			log.Debug(err)
			// parts of profile are missing?
			// status skips these files
			if trace.IsNotFound(err) {
				continue
			}
			return nil, nil, trace.Wrap(err)
		}
		others = append(others, ps)
	}

	return profileStatus, others, nil
}

// LoadProfile populates Config with the values stored in the given
// profiles directory. If profileDir is an empty string, the default profile
// directory ~/.tsh is used.
func (c *Config) LoadProfile(profileDir string, proxyName string) error {
	// read the profile:
	cp, err := profile.FromDir(profileDir, ProxyHost(proxyName))
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
	c.PostgresProxyAddr = cp.PostgresProxyAddr
	c.MySQLProxyAddr = cp.MySQLProxyAddr
	c.MongoProxyAddr = cp.MongoProxyAddr
	c.TLSRoutingEnabled = cp.TLSRoutingEnabled

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
func (c *Config) SaveProfile(dir string, makeCurrent bool) error {
	if c.WebProxyAddr == "" {
		return nil
	}

	dir = profile.FullProfilePath(dir)

	var cp profile.Profile
	cp.Username = c.Username
	cp.WebProxyAddr = c.WebProxyAddr
	cp.SSHProxyAddr = c.SSHProxyAddr
	cp.KubeProxyAddr = c.KubeProxyAddr
	cp.PostgresProxyAddr = c.PostgresProxyAddr
	cp.MySQLProxyAddr = c.MySQLProxyAddr
	cp.MongoProxyAddr = c.MongoProxyAddr
	cp.ForwardedPorts = c.LocalForwardPorts.String()
	cp.SiteName = c.SiteName
	cp.TLSRoutingEnabled = c.TLSRoutingEnabled

	if err := cp.SaveToDir(dir, makeCurrent); err != nil {
		return trace.Wrap(err)
	}
	return nil
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
//   proxy_web_addr:<proxy_web_port>,<proxy_ssh_port>
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
	lastPing *webclient.PingResponse
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
		c.KeyTTL = apidefaults.CertDuration
	}
	c.Namespace = types.ProcessNamespace(c.Namespace)

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
			tc.localAgent = &LocalKeyAgent{Agent: c.Agent, keyStore: noLocalKeyStore{}}
		}
	} else {
		// initialize the local agent (auth agent which uses local SSH keys signed by the CA):
		webProxyHost, _ := tc.WebProxyHostPort()

		var keystore LocalKeyStore
		if c.AddKeysToAgent != AddKeysToAgentOnly {
			keystore, err = NewFSLocalKeyStore(c.KeysDir)
		} else {
			keystore, err = NewMemLocalKeyStore(c.KeysDir)
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tc.localAgent, err = NewLocalAgent(LocalAgentConfig{
			Keystore:   keystore,
			ProxyHost:  webProxyHost,
			Username:   c.Username,
			KeysOption: c.AddKeysToAgent,
			Insecure:   c.InsecureSkipVerify,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if tc.HostKeyCallback == nil {
			tc.HostKeyCallback = tc.localAgent.CheckHostSignature
		}
	}

	return tc, nil
}

// LoadKeyForCluster fetches a cluster-specific SSH key and loads it into the
// SSH agent.
func (tc *TeleportClient) LoadKeyForCluster(clusterName string) error {
	if tc.localAgent == nil {
		return trace.BadParameter("TeleportClient.LoadKeyForCluster called on a client without localAgent")
	}
	_, err := tc.localAgent.LoadKeyForCluster(clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// LoadKeyForCluster fetches a cluster-specific SSH key and loads it into the
// SSH agent.  If the key is not found, it is requested to be reissued.
func (tc *TeleportClient) LoadKeyForClusterWithReissue(ctx context.Context, clusterName string) error {
	err := tc.LoadKeyForCluster(clusterName)
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

// LocalAgent is a getter function for the client's local agent
func (tc *TeleportClient) LocalAgent() *LocalKeyAgent {
	return tc.localAgent
}

// getTargetNodes returns a list of node addresses this SSH command needs to
// operate on.
func (tc *TeleportClient) getTargetNodes(ctx context.Context, proxy *ProxyClient) ([]string, error) {
	var (
		err    error
		nodes  []types.Server
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

// ReissueUserCerts issues new user certs based on params and stores them in
// the local key agent (usually on disk in ~/.tsh).
func (tc *TeleportClient) ReissueUserCerts(ctx context.Context, cachePolicy CertCachePolicy, params ReissueParams) error {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	return proxyClient.ReissueUserCerts(ctx, cachePolicy, params)
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
func (tc *TeleportClient) IssueUserCertsWithMFA(ctx context.Context, params ReissueParams) (*Key, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	key, err := proxyClient.IssueUserCertsWithMFA(ctx, params,
		func(ctx context.Context, proxyAddr string, c *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
			return PromptMFAChallenge(ctx, proxyAddr, c, "")
		})

	return key, err
}

// CreateAccessRequest registers a new access request with the auth server.
func (tc *TeleportClient) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	return proxyClient.CreateAccessRequest(ctx, req)
}

// GetAccessRequests loads all access requests matching the supplied filter.
func (tc *TeleportClient) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	return proxyClient.GetAccessRequests(ctx, filter)
}

// GetRole loads a role resource by name.
func (tc *TeleportClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	return proxyClient.GetRole(ctx, name)
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
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	watcher, err := proxyClient.NewWatcher(ctx, watch)
	if err != nil {
		proxyClient.Close()
		return nil, trace.Wrap(err)
	}

	return watchCloser{
		Watcher: watcher,
		Closer:  proxyClient,
	}, nil
}

// WithRootClusterClient provides a functional interface for making calls
// against the root cluster's auth server.
func (tc *TeleportClient) WithRootClusterClient(ctx context.Context, do func(clt auth.ClientI) error) error {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	clt, err := proxyClient.ConnectToRootCluster(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clt.Close()

	return trace.Wrap(do(clt))
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
	defer nodeClient.Close()

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
			return tc.runCommandOnNodes(ctx, siteInfo.Name, nodeAddrs, proxyClient, command)
		}
		// Reuse the existing nodeClient we connected above.
		return tc.runCommand(ctx, nodeClient, command)
	}

	// Issue "shell" request to run single node.
	if len(nodeAddrs) > 1 {
		fmt.Printf("\x1b[1mWARNING\x1b[0m: Multiple nodes match the label selector, picking first: %v\n", nodeAddrs[0])
	}
	return tc.runShell(ctx, nodeClient, nil)
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
	nodes, err := site.GetNodes(ctx, namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	var node types.Server
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
	return tc.runShell(ctx, nc, session)
}

// Play replays the recorded session
func (tc *TeleportClient) Play(ctx context.Context, namespace, sessionID string) (err error) {
	var sessionEvents []events.EventFields
	var stream []byte
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
	defer proxyClient.Close()

	site, err := proxyClient.ConnectToCurrentCluster(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// request events for that session (to get timing data)
	sessionEvents, err = site.GetSessionEvents(namespace, *sid, 0, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// read the stream into a buffer:
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

	return playSession(sessionEvents, stream)
}

func (tc *TeleportClient) GetSessionEvents(ctx context.Context, namespace, sessionID string) ([]events.EventFields, error) {
	if namespace == "" {
		return nil, trace.BadParameter(auth.MissingNamespaceError)
	}
	sid, err := session.ParseID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("'%v' is not a valid session ID (must be GUID)", sid)
	}
	// connect to the auth server (site) who made the recording
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	site, err := proxyClient.ConnectToCurrentCluster(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	events, err := site.GetSessionEvents(namespace, *sid, 0, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return events, nil
}

// PlayFile plays the recorded session from a tar file
func PlayFile(ctx context.Context, tarFile io.Reader, sid string) error {
	var sessionEvents []events.EventFields
	var stream []byte
	protoReader := events.NewProtoReader(tarFile)
	playbackDir, err := ioutil.TempDir("", "playback")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(playbackDir)
	w, err := events.WriteForPlayback(ctx, session.ID(sid), protoReader, playbackDir)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionEvents, err = w.SessionEvents()
	if err != nil {
		return trace.Wrap(err)
	}
	stream, err = w.SessionChunks()
	if err != nil {
		return trace.Wrap(err)
	}

	return playSession(sessionEvents, stream)
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

	err = nodeClient.ExecuteSCP(ctx, cmd)
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
func (tc *TeleportClient) SCP(ctx context.Context, args []string, port int, flags scp.Flags, quiet bool) (err error) {
	if len(args) < 2 {
		return trace.Errorf("need at least two arguments for scp")
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
	log.Infof("Connecting to proxy to copy (recursively=%v)...", flags.Recursive)
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	var progressWriter io.Writer
	if !quiet {
		progressWriter = tc.Stdout
	}

	// helper function connects to the src/target node:
	connectToNode := func(addr, hostLogin string) (*NodeClient, error) {
		// determine which cluster we're connecting to:
		siteInfo, err := proxyClient.currentCluster()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if hostLogin == "" {
			hostLogin = tc.Config.HostLogin
		}
		return proxyClient.ConnectToNode(ctx,
			NodeAddr{Addr: addr, Namespace: tc.Namespace, Cluster: siteInfo.Name},
			hostLogin, false)
	}

	// gets called to convert SSH error code to tc.ExitStatus
	onError := func(err error) error {
		exitError, _ := trace.Unwrap(err).(*ssh.ExitError)
		if exitError != nil {
			tc.ExitStatus = exitError.ExitStatus()
		}
		return err
	}

	tpl := scp.Config{
		User:           tc.Username,
		ProgressWriter: progressWriter,
		Flags:          flags,
	}

	var config *scpConfig
	// upload:
	if isRemoteDest(last) {
		config, err = tc.uploadConfig(ctx, tpl, port, args)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		config, err = tc.downloadConfig(ctx, tpl, port, args)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	client, err := connectToNode(config.addr, config.hostLogin)
	if err != nil {
		return trace.Wrap(err)
	}

	return onError(client.ExecuteSCP(ctx, config.cmd))
}

func (tc *TeleportClient) uploadConfig(ctx context.Context, tpl scp.Config, port int, args []string) (config *scpConfig, err error) {
	// args are guaranteed to have len(args) > 1
	filesToUpload := args[:len(args)-1]
	// copy everything except the last arg (the destination)
	destPath := args[len(args)-1]

	// If more than a single file were provided, scp must be in directory mode
	// and the target on the remote host needs to be a directory.
	var directoryMode bool
	if len(filesToUpload) > 1 {
		directoryMode = true
	}

	dest, addr, err := getSCPDestination(destPath, port)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tpl.RemoteLocation = dest.Path
	tpl.Flags.Target = filesToUpload
	tpl.Flags.DirectoryMode = directoryMode

	cmd, err := scp.CreateUploadCommand(tpl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scpConfig{
		cmd:       cmd,
		addr:      addr,
		hostLogin: dest.Login,
	}, nil
}

func (tc *TeleportClient) downloadConfig(ctx context.Context, tpl scp.Config, port int, args []string) (config *scpConfig, err error) {
	// args are guaranteed to have len(args) > 1
	src, addr, err := getSCPDestination(args[0], port)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tpl.RemoteLocation = src.Path
	tpl.Flags.Target = args[1:]

	cmd, err := scp.CreateDownloadCommand(tpl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scpConfig{
		cmd:       cmd,
		addr:      addr,
		hostLogin: src.Login,
	}, nil
}

type scpConfig struct {
	cmd       scp.Command
	addr      string
	hostLogin string
}

func getSCPDestination(target string, port int) (dest *scp.Destination, addr string, err error) {
	dest, err = scp.ParseSCPDestination(target)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	addr = net.JoinHostPort(dest.Host.Host(), strconv.Itoa(port))
	return dest, addr, nil
}

func isRemoteDest(name string) bool {
	return strings.ContainsRune(name, ':')
}

// ListNodes returns a list of nodes connected to a proxy
func (tc *TeleportClient) ListNodes(ctx context.Context) ([]types.Server, error) {
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

// ListAppServers returns a list of application servers.
func (tc *TeleportClient) ListAppServers(ctx context.Context) ([]types.AppServer, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	return proxyClient.GetAppServers(ctx, tc.Namespace)
}

// ListApps returns all registered applications.
func (tc *TeleportClient) ListApps(ctx context.Context) ([]types.Application, error) {
	servers, err := tc.ListAppServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var apps []types.Application
	for _, server := range servers {
		apps = append(apps, server.GetApp())
	}
	return types.DeduplicateApps(apps), nil
}

// CreateAppSession creates a new application access session.
func (tc *TeleportClient) CreateAppSession(ctx context.Context, req types.CreateAppSessionRequest) (types.WebSession, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()
	return proxyClient.CreateAppSession(ctx, req)
}

// DeleteAppSession removes the specified application access session.
func (tc *TeleportClient) DeleteAppSession(ctx context.Context, sessionID string) error {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()
	return proxyClient.DeleteAppSession(ctx, sessionID)
}

// ListDatabaseServers returns all registered database proxy servers.
func (tc *TeleportClient) ListDatabaseServers(ctx context.Context) ([]types.DatabaseServer, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()
	return proxyClient.GetDatabaseServers(ctx, tc.Namespace)
}

// ListDatabases returns all registered databases.
func (tc *TeleportClient) ListDatabases(ctx context.Context) ([]types.Database, error) {
	servers, err := tc.ListDatabaseServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var databases []types.Database
	for _, server := range servers {
		databases = append(databases, server.GetDatabase())
	}
	return types.DeduplicateDatabases(databases), nil
}

// ListAllNodes is the same as ListNodes except that it ignores labels.
func (tc *TeleportClient) ListAllNodes(ctx context.Context) ([]types.Server, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	return proxyClient.FindServersByLabels(ctx, tc.Namespace, nil)
}

// runCommandOnNodes executes a given bash command on a bunch of remote nodes.
func (tc *TeleportClient) runCommandOnNodes(
	ctx context.Context, siteName string, nodeAddresses []string, proxyClient *ProxyClient, command []string) error {

	resultsC := make(chan error, len(nodeAddresses))
	for _, address := range nodeAddresses {
		go func(address string) {
			var err error
			defer func() {
				resultsC <- err
			}()

			var nodeClient *NodeClient
			nodeClient, err = proxyClient.ConnectToNode(ctx,
				NodeAddr{Addr: address, Namespace: tc.Namespace, Cluster: siteName},
				tc.Config.HostLogin, false)
			if err != nil {
				// err is passed to resultsC in the defer above.
				fmt.Fprintln(tc.Stderr, err)
				return
			}
			defer nodeClient.Close()

			fmt.Printf("Running command on %v:\n", address)
			err = tc.runCommand(ctx, nodeClient, command)
			// err is passed to resultsC in the defer above.
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

// runCommand executes a given bash command on an established NodeClient.
func (tc *TeleportClient) runCommand(ctx context.Context, nodeClient *NodeClient, command []string) error {
	nodeSession, err := newSession(nodeClient, nil, tc.Config.Env, tc.Stdin, tc.Stdout, tc.Stderr, tc.useLegacyID(nodeClient), tc.EnableEscapeSequences)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nodeSession.Close()
	if err := nodeSession.runCommand(ctx, command, tc.OnShellCreated, tc.Config.Interactive); err != nil {
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

		return trace.Wrap(err)
	}

	return nil
}

// runShell starts an interactive SSH session/shell.
// sessToJoin : when empty, creates a new shell. otherwise it tries to join the existing session.
func (tc *TeleportClient) runShell(ctx context.Context, nodeClient *NodeClient, sessToJoin *session.Session) error {
	nodeSession, err := newSession(nodeClient, sessToJoin, tc.Env, tc.Stdin, tc.Stdout, tc.Stderr, tc.useLegacyID(nodeClient), tc.EnableEscapeSequences)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = nodeSession.runShell(ctx, tc.OnShellCreated); err != nil {
		switch e := trace.Unwrap(err).(type) {
		case *ssh.ExitError:
			tc.ExitStatus = e.ExitStatus()
		case *ssh.ExitMissingError:
			tc.ExitStatus = 1
		}

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
	sshProxyAddr := tc.Config.SSHProxyAddr

	hostKeyCallback := tc.HostKeyCallback
	authMethods := append([]ssh.AuthMethod{}, tc.Config.AuthMethods...)
	clusterName := func() string { return tc.SiteName }
	if len(tc.JumpHosts) > 0 {
		log.Debugf("Overriding SSH proxy to JumpHosts's address %q", tc.JumpHosts[0].Addr.String())
		sshProxyAddr = tc.JumpHosts[0].Addr.Addr

		if tc.localAgent != nil {
			// Wrap host key and auth callbacks using clusterGuesser.
			//
			// clusterGuesser will use the host key callback to guess the target
			// cluster based on the host certificate. It will then use the auth
			// callback to load the appropriate SSH certificate for that cluster.
			clusterGuesser := newProxyClusterGuesser(hostKeyCallback, tc.signersForCluster)
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
				if clusterGuesser.clusterName != rootClusterName {
					return clusterGuesser.clusterName
				}
				return tc.SiteName
			}
		}
	} else if tc.localAgent != nil {
		// tc.SiteName does not necessarily point to the cluster we're
		// connecting to (or that we have certs for). For example tsh login
		// leaf will set tc.SiteName as "leaf" even though we're connecting to
		// root proxy to fetch leaf certs.
		//
		// Instead, load SSH certs for all clusters we have (by passing an
		// empty string to certsForCluster).
		signers, err := tc.localAgent.certsForCluster("")
		// errNoLocalKeyStore is returned when running in the proxy. The proxy
		// should be passing auth methods via tc.Config.AuthMethods.
		if err != nil && !errors.Is(err, errNoLocalKeyStore) && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if len(signers) > 0 {
			authMethods = append(authMethods, ssh.PublicKeys(signers...))
		}
	}

	if len(authMethods) == 0 {
		return nil, trace.BadParameter("no SSH auth methods loaded, are you logged in?")
	}

	sshConfig := &ssh.ClientConfig{
		User:            tc.getProxySSHPrincipal(),
		HostKeyCallback: hostKeyCallback,
		Auth:            authMethods,
	}

	sshClient, err := makeProxySSHClient(ctx, tc, sshConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ProxyClient{
		teleportClient:  tc,
		Client:          sshClient,
		proxyAddress:    sshProxyAddr,
		proxyPrincipal:  sshConfig.User,
		hostKeyCallback: sshConfig.HostKeyCallback,
		authMethods:     sshConfig.Auth,
		hostLogin:       tc.HostLogin,
		siteName:        clusterName(),
		clientAddr:      tc.ClientAddr,
	}, nil
}

// makeProxySSHClient creates an SSH client by following steps:
// 1) If the current proxy supports TLS Routing and JumpHost address was not provided use TLSWrapper.
// 2) Check JumpHost raw SSH port or Teleport proxy address.
//    In case of proxy web address check if the proxy supports TLS Routing and connect to the proxy with TLSWrapper
// 3) Dial sshProxyAddr with raw SSH Dialer where sshProxyAddress is proxy ssh address or JumpHost address if
//    JumpHost address was provided.
func makeProxySSHClient(ctx context.Context, tc *TeleportClient, sshConfig *ssh.ClientConfig) (*ssh.Client, error) {
	// Use TLS Routing dialer only if proxy support TLS Routing and JumpHost was not set.
	if tc.Config.TLSRoutingEnabled && len(tc.JumpHosts) == 0 {
		log.Infof("Connecting to proxy=%v login=%q using TLS Routing", tc.Config.WebProxyAddr, sshConfig.User)
		c, err := makeProxySSHClientWithTLSWrapper(ctx, tc, sshConfig, tc.Config.WebProxyAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.Infof("Successful auth with proxy %v.", tc.Config.WebProxyAddr)
		return c, nil
	}

	sshProxyAddr := tc.Config.SSHProxyAddr

	// Handle situation where a Jump Host was set to proxy web address and Teleport supports TLS Routing.
	if len(tc.JumpHosts) > 0 {
		sshProxyAddr = tc.JumpHosts[0].Addr.Addr
		// Check if JumpHost address is a proxy web address.
		resp, err := webclient.Find(&webclient.Config{Context: ctx, ProxyAddr: sshProxyAddr, Insecure: tc.InsecureSkipVerify})
		// If JumpHost address is a proxy web port and proxy supports TLSRouting dial proxy with TLSWrapper.
		if err == nil && resp.Proxy.TLSRoutingEnabled {
			log.Infof("Connecting to proxy=%v login=%q using TLS Routing JumpHost", sshProxyAddr, sshConfig.User)
			c, err := makeProxySSHClientWithTLSWrapper(ctx, tc, sshConfig, sshProxyAddr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			log.Infof("Successful auth with proxy %v.", sshProxyAddr)
			return c, nil
		}
	}

	log.Infof("Connecting to proxy=%v login=%q", sshProxyAddr, sshConfig.User)
	client, err := makeProxySSHClientDirect(tc, sshConfig, sshProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err, "failed to authenticate with proxy %v", sshProxyAddr)
	}
	log.Infof("Successful auth with proxy %v.", sshProxyAddr)
	return client, nil
}

func makeProxySSHClientDirect(tc *TeleportClient, sshConfig *ssh.ClientConfig, proxyAddr string) (*ssh.Client, error) {
	dialer := proxy.DialerFromEnvironment(tc.Config.SSHProxyAddr)
	return dialer.Dial("tcp", proxyAddr, sshConfig)
}

func makeProxySSHClientWithTLSWrapper(ctx context.Context, tc *TeleportClient, sshConfig *ssh.ClientConfig, proxyAddr string) (*ssh.Client, error) {
	tlsConfig, err := tc.loadTLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig.NextProtos = []string{string(alpncommon.ProtocolProxySSH)}
	dialer := proxy.DialerFromEnvironment(tc.Config.WebProxyAddr, proxy.WithALPNDialer(tlsConfig))
	return dialer.Dial("tcp", proxyAddr, sshConfig)
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
	clusterName string

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
	g.clusterName = cert.Permissions.Extensions[utils.CertExtensionAuthority]
	if g.clusterName == "" {
		log.Debugf("Target SSH server %q does not have a cluster name embedded in their certificate; will use all available client certificates to authenticate", hostname)
	}
	if g.nextHostKeyCallback != nil {
		return g.nextHostKeyCallback(hostname, remote, key)
	}
	return nil
}

func (g *proxyClusterGuesser) authMethod(ctx context.Context) ssh.AuthMethod {
	return ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		return g.signersForCluster(ctx, g.clusterName)
	})
}

func (tc *TeleportClient) signersForCluster(ctx context.Context, clusterName string) ([]ssh.Signer, error) {
	err := tc.WithoutJumpHosts(func(tc *TeleportClient) error {
		return tc.LoadKeyForClusterWithReissue(ctx, clusterName)
	})
	if err != nil {
		log.WithError(err).Warnf("Failed to load/reissue keys for cluster %q.", clusterName)
		return nil, trace.Wrap(err)
	}
	return tc.localAgent.certsForCluster(clusterName)
}

// WithoutJumpHosts executes the given function with a Teleport client that has
// no JumpHosts set, i.e. presumably falling back to the proxy specified in the
// profile.
func (tc *TeleportClient) WithoutJumpHosts(fn func(tcNoJump *TeleportClient) error) error {
	storedJumpHosts := tc.JumpHosts
	tc.JumpHosts = nil
	err := fn(tc)
	tc.JumpHosts = storedJumpHosts
	return trace.Wrap(err)
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
	pr, err := tc.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if pr.Auth.HasMessageOfTheDay {
		err = tc.ShowMOTD(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return pr, nil
}

// Login logs the user into a Teleport cluster by talking to a Teleport proxy.
//
// The returned Key should typically be passed to ActivateKey in order to
// update local agent state.
//
func (tc *TeleportClient) Login(ctx context.Context) (*Key, error) {
	// Ping the endpoint to see if it's up and find the type of authentication
	// supported, also show the message of the day if available.
	pr, err := tc.PingAndShowMOTD(ctx)
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
	case constants.Local:
		response, err = tc.localLogin(ctx, pr.Auth.SecondFactor, key.Pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case constants.OIDC:
		response, err = tc.ssoLogin(ctx, pr.Auth.OIDC.Name, key.Pub, constants.OIDC)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// in this case identity is returned by the proxy
		tc.Username = response.Username
		if tc.localAgent != nil {
			tc.localAgent.username = response.Username
		}
	case constants.SAML:
		response, err = tc.ssoLogin(ctx, pr.Auth.SAML.Name, key.Pub, constants.SAML)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// in this case identity is returned by the proxy
		tc.Username = response.Username
		if tc.localAgent != nil {
			tc.localAgent.username = response.Username
		}
	case constants.Github:
		response, err = tc.ssoLogin(ctx, pr.Auth.Github.Name, key.Pub, constants.Github)
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

	// Check that a host certificate for at least one cluster was returned.
	if len(response.HostSigners) == 0 {
		return nil, trace.BadParameter("bad response from the server: expected at least one certificate, got 0")
	}

	// extract the new certificate out of the response
	key.Cert = response.Cert
	key.TLSCert = response.TLSCert
	if tc.KubernetesCluster != "" {
		key.KubeTLSCerts[tc.KubernetesCluster] = response.TLSCert
	}
	if tc.DatabaseService != "" {
		key.DBTLSCerts[tc.DatabaseService] = response.TLSCert
	}
	key.TrustedCA = response.HostSigners

	// Store the requested cluster name in the key.
	key.ClusterName = tc.SiteName
	if key.ClusterName == "" {
		rootClusterName := key.TrustedCA[0].ClusterName
		key.ClusterName = rootClusterName
		tc.SiteName = rootClusterName
	}

	return key, nil
}

// ActivateKey saves the target session cert into the local
// keystore (and into the ssh-agent) for future use.
func (tc *TeleportClient) ActivateKey(ctx context.Context, key *Key) error {
	if tc.localAgent == nil {
		// skip activation if no local agent is present
		return nil
	}
	// save the list of CAs client trusts to ~/.tsh/known_hosts
	err := tc.localAgent.AddHostSignersToCache(key.TrustedCA)
	if err != nil {
		return trace.Wrap(err)
	}

	// save the list of TLS CAs client trusts
	err = tc.localAgent.SaveTrustedCerts(key.TrustedCA)
	if err != nil {
		return trace.Wrap(err)
	}

	// save the cert to the local storage (~/.tsh usually):
	_, err = tc.localAgent.AddKey(key)
	if err != nil {
		return trace.Wrap(err)
	}

	// Connect to the Auth Server of the root cluster and fetch the known hosts.
	rootClusterName := key.TrustedCA[0].ClusterName
	if err := tc.UpdateTrustedCA(ctx, rootClusterName); err != nil {
		if len(tc.JumpHosts) == 0 {
			return trace.Wrap(err)
		}
		errViaJumphost := err
		// If JumpHosts was pointing at the leaf cluster (e.g. during 'tsh ssh
		// -J leaf.example.com'), this could've caused the above error. Try to
		// fetch CAs without JumpHosts to force it to use the root cluster.
		if err := tc.WithoutJumpHosts(func(tc *TeleportClient) error {
			return tc.UpdateTrustedCA(ctx, rootClusterName)
		}); err != nil {
			return trace.NewAggregate(errViaJumphost, err)
		}
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
		ExtraHeaders:  tc.ExtraProxyHeaders})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If version checking was requested and the server advertises a minimum version.
	if tc.CheckVersions && pr.MinClientVersion != "" {
		if err := utils.CheckVersion(teleport.Version, pr.MinClientVersion); err != nil && trace.IsBadParameter(err) {
			fmt.Printf(`
			WARNING
			Detected potentially incompatible client and server versions.
			Minimum client version supported by the server is %v but you are using %v.
			Please upgrade tsh to %v or newer or use the --skip-version-check flag to bypass this check.
			Future versions of tsh will fail when incompatible versions are detected.
			`, pr.MinClientVersion, teleport.Version, pr.MinClientVersion)
		}
	}

	// Update tc with proxy settings specified in Ping response.
	if err := tc.applyProxySettings(pr.Proxy); err != nil {
		return nil, trace.Wrap(err)
	}

	tc.lastPing = pr

	return pr, nil
}

// ShowMOTD fetches the cluster MotD, displays it (if any) and waits for
// confirmation from the user.
func (tc *TeleportClient) ShowMOTD(ctx context.Context) error {
	motd, err := webclient.GetMOTD(
		&webclient.Config{
			Context:      ctx,
			ProxyAddr:    tc.WebProxyAddr,
			Insecure:     tc.InsecureSkipVerify,
			Pool:         loopbackPool(tc.WebProxyAddr),
			ExtraHeaders: tc.ExtraProxyHeaders})

	if err != nil {
		return trace.Wrap(err)
	}

	if motd.Text != "" {
		fmt.Fprintf(tc.Stderr, "%s\nPress [ENTER] to continue.\n", motd.Text)
		// We're re-using the password reader for user acknowledgment for
		// aesthetic purposes, because we want to hide any garbage the
		// use might enter at the prompt. Whatever the user enters will
		// be simply discarded, and the user can still CTRL+C out if they
		// disagree.
		_, err := passwordFromConsoleFn()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetTrustedCA returns a list of host certificate authorities
// trusted by the cluster client is authenticated with.
func (tc *TeleportClient) GetTrustedCA(ctx context.Context, clusterName string) ([]types.CertAuthority, error) {
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
	return clt.GetCertAuthorities(ctx, types.HostCA, false)
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
		// ListenAddr is the second preference.
		case proxySettings.Kube.ListenAddr != "":
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
	case proxySettings.DB.PostgresListenAddr != "":
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
		// k8s requests by "kube." SNI prefix and route to the kube proxy service.
		tc.KubeProxyAddr = tc.WebProxyAddr
	}

	return nil
}

func (tc *TeleportClient) localLogin(ctx context.Context, secondFactor constants.SecondFactorType, pub []byte) (*auth.SSHLoginResponse, error) {
	var err error
	var response *auth.SSHLoginResponse

	// TODO(awly): mfa: ideally, clients should always go through mfaLocalLogin
	// (with a nop MFA challenge if no 2nd factor is required). That way we can
	// deprecate the direct login endpoint.
	switch secondFactor {
	case constants.SecondFactorOff, constants.SecondFactorOTP:
		response, err = tc.directLogin(ctx, secondFactor, pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case constants.SecondFactorU2F, constants.SecondFactorWebauthn, constants.SecondFactorOn, constants.SecondFactorOptional:
		response, err = tc.mfaLocalLogin(ctx, pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unsupported second factor type: %q", secondFactor)
	}

	return response, nil
}

// AddTrustedCA adds a new CA as trusted CA for this client, used in tests
func (tc *TeleportClient) AddTrustedCA(ca types.CertAuthority) error {
	if tc.localAgent == nil {
		return trace.BadParameter("TeleportClient.AddTrustedCA called on a client without localAgent")
	}
	err := tc.localAgent.AddHostSignersToCache(auth.AuthoritiesToTrustedCerts([]types.CertAuthority{ca}))
	if err != nil {
		return trace.Wrap(err)
	}

	// only host CA has TLS certificates, user CA will overwrite trusted certs
	// to empty file if called
	if ca.GetType() == types.HostCA {
		err = tc.localAgent.SaveTrustedCerts(auth.AuthoritiesToTrustedCerts([]types.CertAuthority{ca}))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// AddKey adds a key to the client's local agent, used in tests.
func (tc *TeleportClient) AddKey(key *Key) (*agent.AddedKey, error) {
	if tc.localAgent == nil {
		return nil, trace.BadParameter("TeleportClient.AddKey called on a client without localAgent")
	}
	if key.ClusterName == "" {
		key.ClusterName = tc.SiteName
	}
	return tc.localAgent.AddKey(key)
}

// directLogin asks for a password + HOTP token, makes a request to CA via proxy
func (tc *TeleportClient) directLogin(ctx context.Context, secondFactorType constants.SecondFactorType, pub []byte) (*auth.SSHLoginResponse, error) {
	password, err := tc.AskPassword()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// only ask for a second factor if it's enabled
	var otpToken string
	if secondFactorType == constants.SecondFactorOTP {
		otpToken, err = tc.AskOTP()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// ask the CA (via proxy) to sign our public key:
	response, err := SSHAgentLogin(ctx, SSHLoginDirect{
		SSHLogin: SSHLogin{
			ProxyAddr:         tc.WebProxyAddr,
			PubKey:            pub,
			TTL:               tc.KeyTTL,
			Insecure:          tc.InsecureSkipVerify,
			Pool:              loopbackPool(tc.WebProxyAddr),
			Compatibility:     tc.CertificateFormat,
			RouteToCluster:    tc.SiteName,
			KubernetesCluster: tc.KubernetesCluster,
		},
		User:     tc.Config.Username,
		Password: password,
		OTPToken: otpToken,
	})

	return response, trace.Wrap(err)
}

// SSOLoginFunc is a function used in tests to mock SSO logins.
type SSOLoginFunc func(ctx context.Context, connectorID string, pub []byte, protocol string) (*auth.SSHLoginResponse, error)

// samlLogin opens browser window and uses OIDC or SAML redirect cycle with browser
func (tc *TeleportClient) ssoLogin(ctx context.Context, connectorID string, pub []byte, protocol string) (*auth.SSHLoginResponse, error) {
	if tc.MockSSOLogin != nil {
		// sso login response is being mocked for testing purposes
		return tc.MockSSOLogin(ctx, connectorID, pub, protocol)
	}
	// ask the CA (via proxy) to sign our public key:
	response, err := SSHAgentSSOLogin(ctx, SSHLoginSSO{
		SSHLogin: SSHLogin{
			ProxyAddr:         tc.WebProxyAddr,
			PubKey:            pub,
			TTL:               tc.KeyTTL,
			Insecure:          tc.InsecureSkipVerify,
			Pool:              loopbackPool(tc.WebProxyAddr),
			Compatibility:     tc.CertificateFormat,
			RouteToCluster:    tc.SiteName,
			KubernetesCluster: tc.KubernetesCluster,
		},
		ConnectorID: connectorID,
		Protocol:    protocol,
		BindAddr:    tc.BindAddr,
		Browser:     tc.Browser,
	})
	return response, trace.Wrap(err)
}

// mfaLocalLogin asks for a password and performs the challenge-response authentication
func (tc *TeleportClient) mfaLocalLogin(ctx context.Context, pub []byte) (*auth.SSHLoginResponse, error) {
	password, err := tc.AskPassword()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := SSHAgentMFALogin(ctx, SSHLoginMFA{
		SSHLogin: SSHLogin{
			ProxyAddr:         tc.WebProxyAddr,
			PubKey:            pub,
			TTL:               tc.KeyTTL,
			Insecure:          tc.InsecureSkipVerify,
			Pool:              loopbackPool(tc.WebProxyAddr),
			Compatibility:     tc.CertificateFormat,
			RouteToCluster:    tc.SiteName,
			KubernetesCluster: tc.KubernetesCluster,
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
	if !apiutils.IsLoopback(proxyAddr) {
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

// connectToSSHAgent connects to the system SSH agent and returns an agent.Agent.
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
	pwd, err = passwordFromConsoleFn()
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

// loadTLS returns the user's TLS configuration for an external identity if the SkipLocalAuth flag was set
// or teleport core TLS certificate for the local agent.
func (tc *TeleportClient) loadTLSConfig() (*tls.Config, error) {
	// if SkipLocalAuth flag is set use an external identity file instead of loading cert from the local agent.
	if tc.SkipLocalAuth {
		return tc.TLS.Clone(), nil
	}

	tlsKey, err := tc.localAgent.GetCoreKey()
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch TLS key for %v", tc.Username)
	}
	tlsConfig, err := tlsKey.TeleportClientTLSConfig(nil)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate client TLS config")
	}
	return tlsConfig, nil
}

// passwordFromConsoleFn allows tests to replace the passwordFromConsole
// function.
var passwordFromConsoleFn = passwordFromConsole

// passwordFromConsole reads from stdin without echoing typed characters to stdout
func passwordFromConsole() (string, error) {
	// syscall.Stdin is not an int on windows. The linter will complain only on
	// linux where syscall.Stdin is an int.
	//
	// nolint:unconvert
	fd := int(syscall.Stdin)
	state, err := term.GetState(fd)

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
				term.Restore(fd, state)
				os.Exit(1)
			case <-closeCh:
			}
		}()
	}
	defer func() {
		close(closeCh)
	}()

	bytes, err := term.ReadPassword(fd)
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

// playSession plays session in the terminal
func playSession(sessionEvents []events.EventFields, stream []byte) error {
	term, err := terminal.New(nil, nil, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	defer term.Close()

	// configure terminal for direct unbuffered echo-less input:
	if term.IsAttached() {
		err := term.InitRaw(true)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	player := newSessionPlayer(sessionEvents, stream, term)
	errorCh := make(chan error)
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
		defer player.EndPlayback()
		var key [1]byte
		for {
			_, err := term.Stdin().Read(key[:])
			if err != nil {
				errorCh <- err
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
	select {
	case <-player.stopC:
		fmt.Println("\n\nend of session playback")
		return nil
	case err := <-errorCh:
		return trace.Wrap(err)
	}
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
