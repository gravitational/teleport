/*
Copyright 2016 Gravitational, Inc.

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
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/dir"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/shell"
	"github.com/gravitational/teleport/lib/state"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/moby/moby/pkg/term"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	// Directory location where tsh profiles (and session keys) are stored
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

type ForwardedPorts []ForwardedPort

// ToString() returns a string representation of a forwarded port spec, compatible
// with OpenSSH's -L  flag, i.e. "src_host:src_port:dest_host:dest_port"
func (p *ForwardedPort) ToString() string {
	sport := strconv.Itoa(p.SrcPort)
	dport := strconv.Itoa(p.DestPort)
	if utils.IsLocalhost(p.SrcIP) {
		return sport + ":" + net.JoinHostPort(p.DestHost, dport)
	}
	return net.JoinHostPort(p.SrcIP, sport) + ":" + net.JoinHostPort(p.DestHost, dport)
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

	// ProxyHostPort is a host or IP of the proxy (with optional ":ssh_port,https_port").
	// The value is taken from the --proxy flag and can look like --proxy=host:5025,5080
	ProxyHostPort string

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

	// Locally forwarded ports (parameters to -L ssh flag)
	LocalForwardPorts ForwardedPorts

	// HostKeyCallback will be called to check host keys of the remote
	// node, if not specified will be using CheckHostSignature function
	// that uses local cache to validate hosts
	HostKeyCallback HostKeyCallback

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

	// Compatibility specifies OpenSSH compatibility flags.
	Compatibility string

	// AuthConnector is the name of the authentication connector to use.
	AuthConnector string
}

// CachePolicy defines cache policy for local clients
type CachePolicy struct {
	// CacheTTL defines cache TTL
	CacheTTL time.Duration
	// NeverExpire never expires local cache information
	NeverExpires bool
}

func MakeDefaultConfig() *Config {
	return &Config{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}
}

// LoadProfile populates Config with the values stored in the given
// profiles directory. If profileDir is an empty string, the default profile
// directory ~/.tsh is used.
func (c *Config) LoadProfile(profileDir string, proxyName string) error {
	profileDir = FullProfilePath(profileDir)
	// read the profile:
	cp, err := ProfileFromDir(profileDir, proxyName)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// apply the profile to the current configuration:
	c.SetProxy(cp.ProxyHost, cp.ProxyWebPort, cp.ProxySSHPort)
	c.Username = cp.Username
	c.SiteName = cp.SiteName
	c.LocalForwardPorts, err = ParsePortForwardSpec(cp.ForwardedPorts)
	if err != nil {
		log.Warnf("Error parsing user profile: %v", err)
	}
	return nil
}

// SaveProfile updates the given profiles directory with the current configuration
// If profileDir is an empty string, the default ~/.tsh is used
func (c *Config) SaveProfile(profileDir string) error {
	if c.ProxyHostPort == "" {
		return nil
	}
	profileDir = FullProfilePath(profileDir)
	profilePath := path.Join(profileDir, c.ProxyHost()) + ".yaml"

	var cp ClientProfile
	cp.ProxyHost = c.ProxyHost()
	cp.Username = c.Username
	cp.ProxySSHPort = c.ProxySSHPort()
	cp.ProxyWebPort = c.ProxyWebPort()
	cp.ForwardedPorts = c.LocalForwardPorts.ToStringSpec()
	cp.SiteName = c.SiteName

	// create a profile file:
	if err := cp.SaveTo(profilePath, ProfileMakeCurrent); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Config) SetProxy(host string, webPort, sshPort int) {
	c.ProxyHostPort = fmt.Sprintf("%s:%d,%d", host, webPort, sshPort)
}

// ProxyHost returns the hostname of the proxy server (without any port numbers)
func (c *Config) ProxyHost() string {
	host, _, err := net.SplitHostPort(c.ProxyHostPort)
	if err != nil {
		return c.ProxyHostPort
	}
	return host
}

func (c *Config) ProxySSHHostPort() string {
	return net.JoinHostPort(c.ProxyHost(), strconv.Itoa(c.ProxySSHPort()))
}

func (c *Config) ProxyWebHostPort() string {
	return net.JoinHostPort(c.ProxyHost(), strconv.Itoa(c.ProxyWebPort()))
}

// ProxyWebPort returns the port number of teleport HTTP proxy stored in the config
// usually 3080 by default.
func (c *Config) ProxyWebPort() (retval int) {
	retval = defaults.HTTPListenPort
	_, port, err := net.SplitHostPort(c.ProxyHostPort)
	if err == nil && len(port) > 0 && port[0] != ',' {
		ports := strings.Split(port, ",")
		if len(ports) > 0 {
			retval, err = strconv.Atoi(ports[0])
			if err != nil {
				log.Warnf("invalid proxy web port: '%v': %v", ports, err)
			}
		}
	}
	return retval
}

// ProxySSHPort returns the port number of teleport SSH proxy stored in the config
// usually 3023 by default.
func (c *Config) ProxySSHPort() (retval int) {
	retval = defaults.SSHProxyListenPort
	_, port, err := net.SplitHostPort(c.ProxyHostPort)
	if err == nil && len(port) > 0 {
		ports := strings.Split(port, ",")
		if len(ports) > 1 {
			retval, err = strconv.Atoi(ports[1])
			if err != nil {
				log.Warnf("invalid proxy SSH port: '%v': %v", ports, err)
			}
		}
	}
	return retval
}

// ProxySpecified returns true if proxy has been specified
func (c *Config) ProxySpecified() bool {
	return len(c.ProxyHostPort) > 0
}

// TeleportClient is a wrapper around SSH client with teleport specific
// workflow built in
type TeleportClient struct {
	Config
	localAgent *LocalKeyAgent

	// OnShellCreated gets called when the shell is created. It's
	// safe to keep it nil
	OnShellCreated ShellCreatedCallback
}

// ShellCreatedCallback can be supplied for every teleport client. It will
// be called right after the remote shell is created, but the session
// hasn't begun yet.
//
// It allows clients to cancel SSH action
type ShellCreatedCallback func(s *ssh.Session, c *ssh.Client, terminal io.ReadWriteCloser) (exit bool, err error)

// NewClient creates a TeleportClient object and fully configures it
func NewClient(c *Config) (tc *TeleportClient, err error) {
	// validate configuration
	if c.Username == "" {
		c.Username, err = Username()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.Infof("no teleport login given. defaulting to %s", c.Username)
	}
	if c.ProxyHostPort == "" {
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
	} else if c.KeyTTL > defaults.MaxCertDuration || c.KeyTTL < defaults.MinCertDuration {
		return nil, trace.BadParameter("invalid requested cert TTL")
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

	// sometimes we need to use external auth without using local auth
	// methods, e.g. in automation daemons
	if c.SkipLocalAuth {
		if len(c.AuthMethods) == 0 {
			return nil, trace.BadParameter("SkipLocalAuth is true but no AuthMethods provided")
		}
		if c.TLS == nil {
			return nil, trace.BadParameter("SkipLocalAuth is true but no TLS config is provided")
		}
		// if the client was passed an agent in the configuration and skip local auth, use
		// the passed in agent.
		if c.Agent != nil {
			tc.localAgent = &LocalKeyAgent{Agent: c.Agent}
		}
	} else {
		// initialize the local agent (auth agent which uses local SSH keys signed by the CA):
		tc.localAgent, err = NewLocalAgent(c.KeysDir, c.Username)
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
	if tc.CachePolicy == nil {
		log.Debugf("not using caching access point")
		return clt, nil
	}
	dirPath, err := initKeysDir(tc.KeysDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	path := filepath.Join(dirPath, "cache", proxyHostPort, clusterName)

	log.Debugf("using caching access point %v", path)
	cacheBackend, err := dir.New(backend.Params{"path": path})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// make a caching auth client for the auth server:
	return state.NewCachingAuthClient(state.Config{
		SkipPreload:  true,
		AccessPoint:  clt,
		Backend:      cacheBackend,
		CacheTTL:     tc.CachePolicy.CacheTTL,
		NeverExpires: tc.CachePolicy.NeverExpires,
	})
}

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
			retval = append(retval, nodes[i].GetAddr())
		}
	}
	if len(nodes) == 0 {
		retval = append(retval, net.JoinHostPort(tc.Host, strconv.Itoa(tc.HostPort)))
	}
	return retval, nil
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
	proxyClient, err := tc.ConnectToProxy()
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
	// more than one node for an interactive shell?
	// that can't be!
	if len(nodeAddrs) != 1 {
		fmt.Printf(
			"\x1b[1mWARNING\x1b[0m: multiple nodes match the label selector. Picking %v (first)\n",
			nodeAddrs[0])
	}
	nodeClient, err := proxyClient.ConnectToNode(
		ctx,
		nodeAddrs[0]+"@"+tc.Namespace+"@"+siteInfo.Name,
		tc.Config.HostLogin,
		false)
	if err != nil {
		tc.ExitStatus = 1
		return trace.Wrap(err)
	}
	// proxy local ports (forward incoming connections to remote host ports)
	tc.startPortForwarding(nodeClient)

	// local execution?
	if runLocally {
		if len(tc.Config.LocalForwardPorts) == 0 {
			fmt.Println("Executing command locally without connecting to any servers. This makes no sense.")
		}
		return runLocalCommand(command)
	}
	// execute command(s) or a shell on remote node(s)
	if len(command) > 0 {
		return tc.runCommand(ctx, siteInfo.Name, nodeAddrs, proxyClient, command)
	}
	return tc.runShell(nodeClient, nil)
}

func (tc *TeleportClient) startPortForwarding(nodeClient *NodeClient) error {
	if len(tc.Config.LocalForwardPorts) > 0 {
		for _, fp := range tc.Config.LocalForwardPorts {
			socket, err := net.Listen("tcp", net.JoinHostPort(fp.SrcIP, strconv.Itoa(fp.SrcPort)))
			if err != nil {
				return trace.Wrap(err)
			}
			go nodeClient.listenAndForward(socket, net.JoinHostPort(fp.DestHost, strconv.Itoa(fp.DestPort)))
		}
	}
	return nil
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
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()
	site, err := proxyClient.ConnectToSite(ctx, false)
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
	nodes, err := site.GetNodes(namespace)
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
	// connect to server:
	fullNodeAddr := node.GetAddr()
	if tc.SiteName != "" {
		fullNodeAddr = fmt.Sprintf("%s@%s@%s", node.GetAddr(), tc.Namespace, tc.SiteName)
	}
	nc, err := proxyClient.ConnectToNode(ctx, fullNodeAddr, tc.Config.HostLogin, false)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nc.Close()

	// start forwarding ports, if configured:
	tc.startPortForwarding(nc)

	// running shell with a given session means "join" it:
	return tc.runShell(nc, session)
}

// Play replays the recorded session
func (tc *TeleportClient) Play(ctx context.Context, namespace, sessionId string) (err error) {
	if namespace == "" {
		return trace.BadParameter(auth.MissingNamespaceError)
	}
	sid, err := session.ParseID(sessionId)
	if err != nil {
		return fmt.Errorf("'%v' is not a valid session ID (must be GUID)", sid)
	}
	// connect to the auth server (site) who made the recording
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	site, err := proxyClient.ConnectToSite(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// request events for that session (to get timing data)
	sessionEvents, err := site.GetSessionEvents(namespace, *sid, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	// read the stream into a buffer:
	var stream []byte
	for err == nil {
		tmp, err := site.GetSessionChunk(namespace, *sid, len(stream), events.MaxChunkBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(tmp) == 0 {
			err = io.EOF
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
	proxyClient, err := tc.ConnectToProxy()
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
		return proxyClient.ConnectToNode(ctx, addr+"@"+tc.Namespace+"@"+siteInfo.Name, tc.HostLogin, false)
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
		login, host, dest := parseSCPDestination(last)
		if login != "" {
			tc.HostLogin = login
		}
		addr := net.JoinHostPort(host, strconv.Itoa(port))

		client, err := connectToNode(addr)
		if err != nil {
			return trace.Wrap(err)
		}
		// copy everything except the last arg (that's destination)
		for _, src := range args[:len(args)-1] {
			err = client.Upload(src, dest, recursive, tc.Stderr, progressWriter)
			if err != nil {
				return onError(err)
			}
		}
		// download:
	} else {
		login, host, src := parseSCPDestination(first)
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		if login != "" {
			tc.HostLogin = login
		}
		client, err := connectToNode(addr)
		if err != nil {
			return trace.Wrap(err)
		}
		// copy everything except the last arg (that's destination)
		for _, dest := range args[1:] {
			err = client.Download(src, dest, recursive, tc.Stderr, progressWriter)
			if err != nil {
				return onError(err)
			}
		}
	}
	return nil
}

// parseSCPDestination takes a string representing a remote resource for SCP
// to download/upload, like "user@host:/path/to/resource.txt" and returns
// 3 components of it
func parseSCPDestination(s string) (login, host, dest string) {
	parts := strings.SplitN(s, "@", 2)
	if len(parts) > 1 {
		login = parts[0]
		host = parts[1]
	} else {
		host = parts[0]
	}
	parts = strings.SplitN(host, ":", 2)
	if len(parts) > 1 {
		host = parts[0]
		dest = parts[1]
	}
	if len(dest) == 0 {
		dest = "."
	}
	return login, host, dest
}

func isRemoteDest(name string) bool {
	return strings.IndexRune(name, ':') >= 0
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
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer proxyClient.Close()
	return proxyClient.FindServersByLabels(ctx, tc.Namespace, tc.Labels)
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
			nodeClient, err = proxyClient.ConnectToNode(ctx, address+"@"+tc.Namespace+"@"+siteName, tc.Config.HostLogin, false)
			if err != nil {
				fmt.Fprintln(tc.Stderr, err)
				return
			}
			defer nodeClient.Close()

			// run the command on one node:
			if len(nodeAddresses) > 1 {
				fmt.Printf("Running command on %v:\n", address)
			}
			nodeSession, err = newSession(nodeClient, nil, tc.Config.Env, tc.Stdin, tc.Stdout, tc.Stderr)
			if err != nil {
				log.Error(err)
				return
			}
			defer nodeSession.Close()
			if err = nodeSession.runCommand(command, tc.OnShellCreated, tc.Config.Interactive); err != nil {
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
	nodeSession, err := newSession(nodeClient, sessToJoin, tc.Env, tc.Stdin, tc.Stdout, tc.Stderr)
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
	// see if we already have a signed key in the cache, we'll use that instead
	if !tc.Config.SkipLocalAuth && tc.LocalAgent() != nil {
		signers, err := tc.LocalAgent().Signers()
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
	if tc.LocalAgent() != nil {
		m = append(m, tc.LocalAgent().AuthMethods()...)
	}
	return m
}

// ConnectToProxy dials the proxy server and returns ProxyClient if successful
func (tc *TeleportClient) ConnectToProxy() (*ProxyClient, error) {
	var err error

	proxyPrincipal := tc.getProxySSHPrincipal()
	proxyAddr := tc.Config.ProxySSHHostPort()
	sshConfig := &ssh.ClientConfig{
		User:            proxyPrincipal,
		HostKeyCallback: tc.HostKeyCallback,
	}

	// helper to create a ProxyClient struct
	makeProxyClient := func(sshClient *ssh.Client, m ssh.AuthMethod) *ProxyClient {
		return &ProxyClient{
			teleportClient:  tc,
			Client:          sshClient,
			proxyAddress:    proxyAddr,
			proxyPrincipal:  proxyPrincipal,
			hostKeyCallback: sshConfig.HostKeyCallback,
			authMethod:      m,
			hostLogin:       tc.Config.HostLogin,
			siteName:        tc.Config.SiteName,
			clientAddr:      tc.ClientAddr,
		}
	}
	successMsg := fmt.Sprintf("[CLIENT] successful auth with proxy %v", proxyAddr)
	// try to authenticate using every non interactive auth method we have:
	for i, m := range tc.authMethods() {
		log.Infof("[CLIENT] connecting proxy=%v login='%v' method=%d", proxyAddr, sshConfig.User, i)
		var sshClient *ssh.Client

		sshConfig.Auth = []ssh.AuthMethod{m}
		sshClient, err = ssh.Dial("tcp", proxyAddr, sshConfig)
		if err != nil {
			if utils.IsHandshakeFailedError(err) {
				log.Warn(err)
				continue
			}
			return nil, trace.Wrap(err)
		}
		log.Infof(successMsg)
		return makeProxyClient(sshClient, m), nil
	}
	// we have exhausted all auth existing auth methods and local login
	// is disabled in configuration, or the user refused connecting to untrusted hosts
	if tc.Config.SkipLocalAuth || tc.localAgent.UserRefusedHosts() {
		if err == nil {
			err = trace.BadParameter("failed to authenticate with proxy %v", proxyAddr)
		}
		return nil, trace.Wrap(err)
	}
	// if we get here, it means we failed to authenticate using stored keys
	// and we need to ask for the login information
	key, err := tc.Login(true)
	if err != nil {
		// we need to communicate directly to user here,
		// otherwise user will see endless loop with no explanation
		if trace.IsTrustError(err) {
			fmt.Printf("Refusing to connect to untrusted proxy %v without --insecure flag\n", proxyAddr)
		}
		return nil, trace.Wrap(err)
	}
	authMethod, err := key.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// After successful login we have local agent updated with latest
	// and greatest auth information, try it now
	sshConfig.Auth = []ssh.AuthMethod{authMethod}
	sshConfig.User = proxyPrincipal
	sshClient, err := ssh.Dial("tcp", proxyAddr, sshConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf(successMsg)
	proxyClient := makeProxyClient(sshClient, authMethod)
	// get (and remember) the site info:
	site, err := proxyClient.currentCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc.SiteName = site.Name
	return proxyClient, nil
}

// Logout locates a certificate stored for a given proxy and deletes it
func (tc *TeleportClient) Logout() error {
	return trace.Wrap(tc.localAgent.DeleteKey(tc.ProxyHost(), tc.Config.Username))
}

// Login logs the user into a Teleport cluster by talking to a Teleport proxy.
//
// If 'activateKey' is true, saves the received session cert into the local
// keystore (and into the ssh-agent) for future use.
//
func (tc *TeleportClient) Login(activateKey bool) (*Key, error) {
	httpsProxyHostPort := tc.Config.ProxyWebHostPort()
	certPool := loopbackPool(httpsProxyHostPort)

	// ping the endpoint to see if it's up and find the type of authentication supported
	pr, err := Ping(httpsProxyHostPort, tc.InsecureSkipVerify, certPool, tc.AuthConnector)
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
		response, err = tc.localLogin(pr.Auth.SecondFactor, key.Pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.OIDC:
		response, err = tc.ssoLogin(pr.Auth.OIDC.Name, key.Pub, teleport.OIDC)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// in this case identity is returned by the proxy
		tc.Username = response.Username
	case teleport.SAML:
		response, err = tc.ssoLogin(pr.Auth.SAML.Name, key.Pub, teleport.SAML)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// in this case identity is returned by the proxy
		tc.Username = response.Username
	case teleport.Github:
		response, err = tc.ssoLogin(pr.Auth.Github.Name, key.Pub, teleport.Github)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// in this case identity is returned by the proxy
		tc.Username = response.Username
	default:
		return nil, trace.BadParameter("unsupported authentication type: %q", pr.Auth.Type)
	}

	// extract the new certificate out of the response
	key.Cert = response.Cert
	key.TLSCert = response.TLSCert

	if activateKey {
		// save the list of CAs client trusts to ~/.tsh/known_hosts
		err = tc.localAgent.AddHostSignersToCache(response.HostSigners)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// save the list of TLS CAs client trusts
		err = tc.localAgent.SaveCerts(tc.ProxyHost(), response.HostSigners)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// save the cert to the local storage (~/.tsh usually):
		_, err = tc.localAgent.AddKey(tc.ProxyHost(), tc.Config.Username, key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return key, nil
}

func (tc *TeleportClient) localLogin(secondFactor string, pub []byte) (*auth.SSHLoginResponse, error) {
	var err error
	var response *auth.SSHLoginResponse

	switch secondFactor {
	case teleport.OFF, teleport.OTP, teleport.TOTP, teleport.HOTP:
		response, err = tc.directLogin(secondFactor, pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case teleport.U2F:
		response, err = tc.u2fLogin(pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unsupported second factor type: %q", secondFactor)
	}

	return response, nil
}

// Adds a new CA as trusted CA for this client, used in tests
func (tc *TeleportClient) AddTrustedCA(ca services.CertAuthority) error {
	err := tc.LocalAgent().AddHostSignersToCache(auth.AuthoritiesToTrustedCerts([]services.CertAuthority{ca}))
	if err != nil {
		return trace.Wrap(err)
	}

	// only host CA has TLS certificates, user CA will overwrite trusted certs
	// to empty file if called
	if ca.GetType() == services.HostCA {
		err = tc.LocalAgent().SaveCerts(tc.ProxyHost(), auth.AuthoritiesToTrustedCerts([]services.CertAuthority{ca}))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (tc *TeleportClient) AddKey(host string, key *Key) (*agent.AddedKey, error) {
	return tc.localAgent.AddKey(host, tc.Username, key)
}

// directLogin asks for a password + HOTP token, makes a request to CA via proxy
func (tc *TeleportClient) directLogin(secondFactorType string, pub []byte) (*auth.SSHLoginResponse, error) {
	var err error

	httpsProxyHostPort := tc.Config.ProxyWebHostPort()
	certPool := loopbackPool(httpsProxyHostPort)

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
	response, err := SSHAgentLogin(
		httpsProxyHostPort,
		tc.Config.Username,
		password,
		otpToken,
		pub,
		tc.KeyTTL,
		tc.InsecureSkipVerify,
		certPool,
		tc.Compatibility)

	return response, trace.Wrap(err)
}

// samlLogin opens browser window and uses OIDC or SAML redirect cycle with browser
func (tc *TeleportClient) ssoLogin(connectorID string, pub []byte, protocol string) (*auth.SSHLoginResponse, error) {
	log.Debugf("samlLogin start")
	// ask the CA (via proxy) to sign our public key:
	webProxyAddr := tc.Config.ProxyWebHostPort()
	response, err := SSHAgentSSOLogin(
		webProxyAddr,
		connectorID,
		pub,
		tc.KeyTTL,
		tc.InsecureSkipVerify,
		loopbackPool(webProxyAddr),
		protocol,
		tc.Compatibility)
	return response, trace.Wrap(err)
}

// directLogin asks for a password and performs the challenge-response authentication
func (tc *TeleportClient) u2fLogin(pub []byte) (*auth.SSHLoginResponse, error) {
	// U2F login requires the official u2f-host executable
	_, err := exec.LookPath("u2f-host")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpsProxyHostPort := tc.Config.ProxyWebHostPort()
	certPool := loopbackPool(httpsProxyHostPort)

	password, err := tc.AskPassword()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := SSHAgentU2FLogin(
		httpsProxyHostPort,
		tc.Config.Username,
		password,
		pub,
		tc.KeyTTL,
		tc.InsecureSkipVerify,
		certPool,
		tc.Compatibility)

	return response, trace.Wrap(err)
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

// connects to a local SSH agent
func connectToSSHAgent() agent.Agent {
	socketPath := os.Getenv(teleport.SSHAuthSock)
	if socketPath == "" {
		log.Infof("[KEY AGENT] %v is not set. Try running eval `ssh-agent` and trying again.", teleport.SSHAuthSock)
		return nil
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Errorf("[KEY AGENT] Unable to connect to SSH agent on socket: %q.", socketPath)
		return nil
	}

	log.Infof("[KEY AGENT] Conneced to System Agent: %q", socketPath)
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

// passwordFromConsole reads from stdin without echoing typed characters to stdout
func passwordFromConsole() (string, error) {
	fd := syscall.Stdin
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
		if i+1 == specLen {
			i++
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

// ToString() returns the same string spec which can be parsed by ParsePortForwardSpec
func (fp ForwardedPorts) ToStringSpec() (retval []string) {
	for _, p := range fp {
		retval = append(retval, p.ToString())
	}
	return retval
}

// ParsePortForwardSpec parses parameter to -L flag, i.e. strings like "[ip]:80:remote.host:3000"
// The opposite of this function (spec generation) is ForwardedPorts.ToString()
func ParsePortForwardSpec(spec []string) (ports ForwardedPorts, err error) {
	if len(spec) == 0 {
		return ports, nil
	}
	const errTemplate = "Invalid port forwarding spec: '%s'. Could be like `80:remote.host:80`"
	ports = make([]ForwardedPort, len(spec), len(spec))

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
