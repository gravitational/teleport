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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/term"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
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

	// HostLogin is a user login on a remote host
	HostLogin string

	// HostPort is a remote host port to connect to
	HostPort int

	// ProxyHost is a host or IP of the proxy (with optional ":port")
	ProxyHost string

	// KeyTTL is a time to live for the temporary SSH keypair to remain valid:
	KeyTTL time.Duration

	// InsecureSkipVerify is an option to skip HTTPS cert check
	InsecureSkipVerify bool

	// SkipLocalAuth will not try to connect to local SSH agent
	// or use any local certs, and not use interactive logins
	SkipLocalAuth bool

	// AuthMethods to use to login into cluster. If left empty, teleport will
	// use its own session store,
	AuthMethods []ssh.AuthMethod

	Stdout io.Writer
	Stderr io.Writer

	// ExitStatus carries the returned value (exit status) of the remote
	// process execution (via SSh exec)
	ExitStatus int

	// SiteName specifies site to execute operation,
	// if omitted, first available site will be selected
	SiteName string

	// Locally forwarded ports (parameters to -L ssh flag)
	LocalForwardPorts []ForwardedPort

	// HostKeyCallback will be called to check host keys of the remote
	// node, if not specified will be using CheckHostSignature function
	// that uses local cache to validate hosts
	HostKeyCallback HostKeyCallback

	// ConnectorID is used to authenticate user via OpenID Connect
	// registered connector
	ConnectorID string

	// KeyDir defines where temporary session keys will be stored.
	// if empty, they'll go to ~/.tsh
	KeysDir string
}

// ProxyHostPort returns a full host:port address of the proxy or an empty string if no
// proxy is given. If 'forWeb' flag is set, returns HTTPS port, otherwise
// returns SSH port (proxy servers listen on both)
func (c *Config) ProxyHostPort(defaultPort int) string {
	if c.ProxySpecified() {
		host, port, err := net.SplitHostPort(c.ProxyHost)
		if err == nil && len(port) > 0 {
			// c.ProxyHost was already specified as "host:port"
			return net.JoinHostPort(host, port)
		}
		// need to default to the given 'defaultPort'
		port = fmt.Sprintf("%d", defaultPort)
		return net.JoinHostPort(c.ProxyHost, port)
	}
	return ""
}

// NodeHostPort returns host:port string based on user supplied data
// either if user has set host:port in the connection string,
// or supplied the -p flag. If user has set both, -p flag data is ignored
func (c *Config) NodeHostPort() string {
	if strings.Contains(c.Host, ":") {
		return c.Host
	}
	return net.JoinHostPort(c.Host, strconv.Itoa(c.HostPort))
}

// ProxySpecified returns true if proxy has been specified
func (c *Config) ProxySpecified() bool {
	return len(c.ProxyHost) > 0
}

// TeleportClient is a wrapper around SSH client with teleport specific
// workflow built in
type TeleportClient struct {
	Config
	localAgent *LocalKeyAgent
}

func (tc *TeleportClient) authMethods() []ssh.AuthMethod {
	return tc.Config.AuthMethods
}

// NewClient creates a TeleportClient object and fully configures it
func NewClient(c *Config) (tc *TeleportClient, err error) {
	// validate configuration
	if c.Username == "" {
		c.Username = Username()
		log.Infof("no teleport login given. defaulting to %s", c.Username)
	}
	if c.ProxyHost == "" {
		return nil, trace.Errorf("No proxy address specified, missed --proxy flag?")
	}
	if c.HostLogin == "" {
		c.HostLogin = Username()
		log.Infof("no host login given. defaulting to %s", c.HostLogin)
	}
	if c.KeyTTL == 0 {
		c.KeyTTL = defaults.CertDuration
	} else if c.KeyTTL > defaults.MaxCertDuration || c.KeyTTL < defaults.MinCertDuration {
		return nil, trace.Errorf("invalid requested cert TTL")
	}

	tc = &TeleportClient{Config: *c}

	// initialize the local agent (auth agent which uses local SSH keys signed by the CA):
	tc.localAgent, err = NewLocalAgent("", c.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tc.Stdout == nil {
		tc.Stdout = os.Stdout
	}
	if tc.Stderr == nil {
		tc.Stderr = os.Stderr
	}
	if tc.HostKeyCallback == nil {
		tc.HostKeyCallback = tc.localAgent.CheckHostSignature
	}

	// sometimes we need to use external auth without using local auth
	// methods, e.g. in automation daemons
	if c.SkipLocalAuth {
		if len(c.AuthMethods) == 0 {
			return nil, trace.BadParameter("SkipLocalAuth is true but no AuthMethods provided")
		}
		return tc, nil
	}

	// first, see if we can authenticate with credentials stored in
	// a local SSH agent:
	if sshAgent := connectToSSHAgent(); sshAgent != nil {
		tc.Config.AuthMethods = append(tc.Config.AuthMethods, authMethodFromAgent(sshAgent))
	}
	// then, we'll auth with the local agent keys:
	tc.Config.AuthMethods = append(tc.Config.AuthMethods, authMethodFromAgent(tc.localAgent))

	return tc, nil
}

func (tc *TeleportClient) LocalAgent() *LocalKeyAgent {
	return tc.localAgent
}

// getTargetNodes returns a list of node addresses this SSH command needs to
// operate on.
func (tc *TeleportClient) getTargetNodes(proxy *ProxyClient) ([]string, error) {
	var (
		err    error
		nodes  []services.Server
		retval = make([]string, 0)
	)
	if tc.Labels != nil && len(tc.Labels) > 0 {
		nodes, err = proxy.FindServersByLabels(tc.Labels)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for i := 0; i < len(nodes); i++ {
			retval = append(retval, nodes[i].Addr)
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
func (tc *TeleportClient) SSH(command []string, runLocally bool, input io.Reader) error {
	// connect to proxy first:
	if !tc.Config.ProxySpecified() {
		return trace.BadParameter("proxy server is not specified")
	}
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	siteInfo, err := proxyClient.getSite()
	if err != nil {
		return trace.Wrap(err)
	}

	// which nodes are we executing this commands on?
	nodeAddrs, err := tc.getTargetNodes(proxyClient)
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

	// proxy local ports (forward incoming connections to remote host ports)
	if len(tc.Config.LocalForwardPorts) > 0 {
		nodeClient, err := proxyClient.ConnectToNode(nodeAddrs[0]+"@"+siteInfo.Name, tc.Config.HostLogin)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, fp := range tc.Config.LocalForwardPorts {
			socket, err := net.Listen("tcp", net.JoinHostPort(fp.SrcIP, strconv.Itoa(fp.SrcPort)))
			if err != nil {
				return trace.Wrap(err)
			}
			go nodeClient.listenAndForward(socket, net.JoinHostPort(fp.DestHost, strconv.Itoa(fp.DestPort)))
		}
	}

	// local execution?
	if runLocally {
		if len(tc.Config.LocalForwardPorts) == 0 {
			fmt.Println("Executing command locally without connecting to any servers. This makes no sense.")
		}
		return runLocalCommand(command)
	}

	// execute command(s) or a shell on remote node(s)
	if len(command) > 0 {
		return tc.runCommand(siteInfo.Name, nodeAddrs, proxyClient, command)
	}

	nodeClient, err := proxyClient.ConnectToNode(nodeAddrs[0]+"@"+siteInfo.Name, tc.Config.HostLogin)
	if err != nil {
		return trace.Wrap(err)
	}
	return tc.runShell(nodeClient, "", input)
}

// Join connects to the existing/active SSH session
func (tc *TeleportClient) Join(sessionID session.ID, input io.Reader) (err error) {
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
	site, err := proxyClient.ConnectToSite()
	if err != nil {
		return trace.Wrap(err)
	}

	// find the session ID on the site:
	sessions, err := site.GetSessions()
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
	nodes, err := site.GetNodes()
	if err != nil {
		return trace.Wrap(err)
	}
	var node *services.Server
	for _, n := range nodes {
		if n.ID == serverID {
			node = &n
			break
		}
	}
	if node == nil {
		return trace.NotFound(notFoundErrorMessage)
	}
	// connect to server:
	nc, err := proxyClient.ConnectToNode(node.Addr, tc.Config.HostLogin)
	if err != nil {
		return trace.Wrap(err)
	}
	return tc.runShell(nc, session.ID, input)
}

// Play replays the recorded session
func (tc *TeleportClient) Play(sessionId string) (err error) {
	sid, err := session.ParseID(sessionId)
	if err != nil {
		return fmt.Errorf("'%v' is not a valid session ID (must be GUID)", sid)
	}
	// connect to the auth server (site) who made the recording
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	site, err := proxyClient.ConnectToSite()
	if err != nil {
		return trace.Wrap(err)
	}
	// request events for that session (to get timing data)
	sessionEvents, err := site.GetSessionEvents(*sid, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	// read the stream into a buffer:
	var stream []byte
	for err == nil {
		tmp, err := site.GetSessionChunk(*sid, len(stream), events.MaxChunkBytes)
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
func (tc *TeleportClient) SCP(args []string, port int, recursive bool) (err error) {
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
	log.Infof("Connecting to proxy...")
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

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

		client, err := proxyClient.ConnectToNode(addr, tc.HostLogin)
		if err != nil {
			return trace.Wrap(err)
		}
		// copy everything except the last arg (that's destination)
		for _, src := range args[:len(args)-1] {
			err = client.Upload(src, dest, tc.Stderr)
			if err != nil {
				return onError(err)
			}
			fmt.Printf("uploaded %s\n", src)
		}
		// download:
	} else {
		login, host, src := parseSCPDestination(first)
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		if login != "" {
			tc.HostLogin = login
		}
		client, err := proxyClient.ConnectToNode(addr, tc.HostLogin)
		if err != nil {
			return trace.Wrap(err)
		}
		// copy everything except the last arg (that's destination)
		for _, dest := range args[1:] {
			err = client.Download(src, dest, recursive, tc.Stderr)
			if err != nil {
				return onError(err)
			}
			fmt.Printf("downloaded %s\n", src)
		}
	}
	return nil
}

// parseSCPDestination takes a string representing a remote resource for SCP
// to download/upload, like "user@host:/path/to/resource.txt" and returns
// 3 components of it
func parseSCPDestination(s string) (login, host, dest string) {
	i := strings.IndexRune(s, '@')
	if i > 0 && i < len(s) {
		login = s[:i]
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	return login, parts[0], strings.Join(parts[1:], ":")
}

func isRemoteDest(name string) bool {
	return strings.IndexRune(name, ':') >= 0
}

// ListNodes returns a list of nodes connected to a proxy
func (tc *TeleportClient) ListNodes() ([]services.Server, error) {
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
	return proxyClient.FindServersByLabels(tc.Labels)
}

// runCommand executes a given bash command on a bunch of remote nodes
func (tc *TeleportClient) runCommand(siteName string, nodeAddresses []string, proxyClient *ProxyClient, command []string) error {
	resultsC := make(chan error, len(nodeAddresses))
	for _, address := range nodeAddresses {
		go func(address string) {
			var err error
			defer func() {
				resultsC <- err
			}()
			var nodeClient *NodeClient
			nodeClient, err = proxyClient.ConnectToNode(address+"@"+siteName, tc.Config.HostLogin)
			if err != nil {
				fmt.Fprintln(tc.Stderr, err)
				return
			}
			defer nodeClient.Close()

			// run the command on one node:
			if len(nodeAddresses) > 1 {
				fmt.Printf("Running command on %v:\n", address)
			}
			err = nodeClient.Run(command, tc.Stdout, tc.Stderr)
			if err != nil {
				exitErr, ok := err.(*ssh.ExitError)
				if ok {
					tc.ExitStatus = exitErr.ExitStatus()
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
// stdin  : standard input to use. if nil, uses os.Stdin
func (tc *TeleportClient) runShell(nodeClient *NodeClient, sessionID session.ID, stdin io.Reader) error {
	defer nodeClient.Close()
	address := tc.NodeHostPort()

	if stdin == nil {
		stdin = os.Stdin
	}
	if tc.Stdout == nil {
		tc.Stdout = os.Stdout
	}
	if tc.Stderr == nil {
		tc.Stderr = os.Stderr
	}
	// terminal must be in raw mode
	var (
		state   *term.State
		err     error
		exitMsg string
	)
	if stdin == os.Stdin && term.IsTerminal(0) {
		state, err = term.SetRawTerminal(0)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	defer func() {
		if state != nil {
			term.RestoreTerminal(0, state)
		}
		if exitMsg != "" {
			fmt.Println(exitMsg)
		}
	}()

	broadcastClose := utils.NewCloseBroadcaster()

	// Catch term signals
	exitSignals := make(chan os.Signal, 1)
	signal.Notify(exitSignals, syscall.SIGTERM)
	go func() {
		defer broadcastClose.Close()
		<-exitSignals
		exitMsg = fmt.Sprintf("Connection to %s closed\n", address)
	}()

	winSize, err := term.GetWinsize(0)
	if err != nil {
		log.Error(err)
		winSize = &term.Winsize{Width: 80, Height: 25}
	}

	shell, err := nodeClient.Shell(int(winSize.Width), int(winSize.Height), sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Catch Ctrl-C signal
	ctrlCSignal := make(chan os.Signal, 1)
	signal.Notify(ctrlCSignal, syscall.SIGINT)
	go func() {
		for {
			<-ctrlCSignal
			_, err := shell.Write([]byte{3})
			if err != nil {
				log.Errorf(err.Error())
			}
		}
	}()

	// Catch Ctrl-Z signal
	ctrlZSignal := make(chan os.Signal, 1)
	signal.Notify(ctrlZSignal, syscall.SIGTSTP)
	go func() {
		for {
			<-ctrlZSignal
			_, err := shell.Write([]byte{26})
			if err != nil {
				log.Errorf(err.Error())
			}
		}
	}()

	// copy from the remote shell to the local
	go func() {
		defer broadcastClose.Close()
		_, err := io.Copy(tc.Stdout, shell)
		if err != nil {
			log.Errorf(err.Error())
		}
		exitMsg = fmt.Sprintf("Connection to %s closed from the remote side", address)
	}()

	// copy from the local shell to the remote
	go func() {
		defer broadcastClose.Close()
		buf := make([]byte, 1)
		for {
			n, err := stdin.Read(buf)
			if err != nil {
				fmt.Println(trace.Wrap(err))
				return
			}
			if n > 0 {
				_, err = shell.Write(buf[:n])
				if err != nil {
					exitMsg = err.Error()
					return
				}
			}
		}
	}()

	<-broadcastClose.C
	return nil
}

// getProxyLogin determines which SSH login to use when connecting to proxy.
func (tc *TeleportClient) getProxyLogin() string {
	// we'll fall back to using the target host login
	proxyLogin := tc.Config.HostLogin
	// see if we already have a signed key in the cache, we'll use that instead
	keys, err := tc.GetKeys()
	if err == nil && len(keys) > 0 {
		principals := keys[0].Certificate.ValidPrincipals
		if len(principals) > 0 {
			proxyLogin = principals[0]
		}
	}
	return proxyLogin
}

// GetKeys returns a list of stored local keys/certs for this Teleport
// user
func (tc *TeleportClient) GetKeys() ([]agent.AddedKey, error) {
	return tc.LocalAgent().GetKeys(tc.Username)
}

// ConnectToProxy dials the proxy server and returns ProxyClient if successful
func (tc *TeleportClient) ConnectToProxy() (*ProxyClient, error) {
	proxyAddr := tc.Config.ProxyHostPort(defaults.SSHProxyListenPort)
	sshConfig := &ssh.ClientConfig{
		User:            tc.getProxyLogin(),
		HostKeyCallback: tc.HostKeyCallback,
	}

	log.Infof("connecting to proxy: %v with host login %v", proxyAddr, sshConfig.User)

	// try to authenticate using every non interactive auth method we have:
	for _, m := range tc.authMethods() {
		sshConfig.Auth = []ssh.AuthMethod{m}
		proxyClient, err := ssh.Dial("tcp", proxyAddr, sshConfig)
		log.Infof("ssh.Dial error: %v", err)
		if err != nil {
			if utils.IsHandshakeFailedError(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		log.Infof("Successfully authenticated with %v", proxyAddr)
		return &ProxyClient{
			Client:          proxyClient,
			proxyAddress:    proxyAddr,
			hostKeyCallback: sshConfig.HostKeyCallback,
			authMethods:     tc.authMethods(),
			hostLogin:       tc.Config.HostLogin,
			siteName:        tc.Config.SiteName,
		}, nil
	}
	// we have exhausted all auth existing auth methods and local login
	// is disabled in configuration
	if tc.Config.SkipLocalAuth {
		return nil, trace.BadParameter("failed to authenticate with proxy %v", proxyAddr)
	}
	// if we get here, it means we failed to authenticate using stored keys
	// and we need to ask for the login information
	err := tc.Login()
	if err != nil {
		// we need to communicate directly to user here,
		// otherwise user will see endless loop with no explanation
		if trace.IsTrustError(err) {
			fmt.Printf("Refusing to connect to untrusted proxy %v without --insecure flag\n", proxyAddr)
		}
		return nil, trace.Wrap(err)
	}
	log.Debugf("Received a new set of keys from %v", proxyAddr)
	// After successfull login we have local agent updated with latest
	// and greatest auth information, try it now
	sshConfig.Auth = []ssh.AuthMethod{authMethodFromAgent(tc.localAgent)}
	proxyClient, err := ssh.Dial("tcp", proxyAddr, sshConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Successfully authenticated with %v", proxyAddr)
	return &ProxyClient{
		Client:          proxyClient,
		proxyAddress:    proxyAddr,
		hostKeyCallback: sshConfig.HostKeyCallback,
		authMethods:     tc.authMethods(),
		hostLogin:       tc.Config.HostLogin,
		siteName:        tc.Config.SiteName,
	}, nil
}

// Login logs user in using proxy's local 2FA auth access
// or used OIDC external authentication, it later
// saves the generated credentials into local keystore for future use
func (tc *TeleportClient) Login() error {
	// generate a new keypair. the public key will be signed via proxy if our password+HOTP  are legit
	key, err := tc.MakeKey()
	if err != nil {
		return trace.Wrap(err)
	}

	var response *web.SSHLoginResponse
	if tc.ConnectorID == "" {
		response, err = tc.directLogin(key.Pub)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		response, err = tc.oidcLogin(tc.ConnectorID, key.Pub)
		if err != nil {
			return trace.Wrap(err)
		}
		// in this case identity is returned by the proxy
		tc.Username = response.Username
	}
	key.Cert = response.Cert
	// save the key:
	if err = tc.localAgent.AddKey(tc.ProxyHost, tc.Config.Username, key); err != nil {
		return trace.Wrap(err)
	}
	// save the list of CAs we trust to the cache file
	err = tc.localAgent.AddHostSignersToCache(response.HostSigners)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Adds a new CA as trusted CA for this client
func (tc *TeleportClient) AddTrustedCA(ca *services.CertAuthority) error {
	return tc.LocalAgent().AddHostSignersToCache([]services.CertAuthority{*ca})
}

// MakeKey generates a new unsigned key. It's useless by itself until a
// trusted CA signs it
func (tc *TeleportClient) MakeKey() (key *Key, err error) {
	key = &Key{}
	keygen := native.New()
	defer keygen.Close()
	key.Priv, key.Pub, err = keygen.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (tc *TeleportClient) AddKey(host string, key *Key) error {
	return tc.localAgent.AddKey(host, tc.Username, key)
}

// directLogin asks for a password + HOTP token, makes a request to CA via proxy
func (tc *TeleportClient) directLogin(pub []byte) (*web.SSHLoginResponse, error) {
	password, hotpToken, err := tc.AskPasswordAndHOTP()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// ask the CA (via proxy) to sign our public key:
	response, err := web.SSHAgentLogin(tc.Config.ProxyHostPort(defaults.HTTPListenPort), tc.Config.Username,
		password, hotpToken, pub, tc.KeyTTL, tc.InsecureSkipVerify, loopbackPool(tc.Config.ProxyHostPort(defaults.HTTPListenPort)))

	return response, trace.Wrap(err)
}

// oidcLogin opens browser window and uses OIDC redirect cycle with browser
func (tc *TeleportClient) oidcLogin(connectorID string, pub []byte) (*web.SSHLoginResponse, error) {
	log.Infof("oidcLogin start")
	// ask the CA (via proxy) to sign our public key:
	response, err := web.SSHAgentOIDCLogin(tc.Config.ProxyHostPort(defaults.HTTPListenPort),
		connectorID, pub, tc.KeyTTL, tc.InsecureSkipVerify, loopbackPool(tc.Config.ProxyHostPort(defaults.HTTPListenPort)))
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
	socketPath := os.Getenv("SSH_AUTH_SOCK")
	if socketPath == "" {
		log.Infof("SSH_AUTH_SOCK is not set. Is local SSH agent running?")
		return nil
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Errorf("Failed connecting to local SSH agent via %s", socketPath)
		return nil
	}
	return agent.NewClient(conn)
}

// Username returns the current user's username
func Username() string {
	u, err := user.Current()
	if err != nil {
		utils.FatalError(err)
	}
	return u.Username
}

// AskPasswordAndHOTP prompts the user to enter the password + HTOP 2nd factor
func (tc *TeleportClient) AskPasswordAndHOTP() (pwd string, token string, err error) {
	fmt.Printf("Enter password for Teleport user %v:\n", tc.Config.Username)
	pwd, err = passwordFromConsole()
	if err != nil {
		fmt.Println(err)
		return "", "", trace.Wrap(err)
	}

	fmt.Printf("Enter your HOTP token:\n")
	token, err = lineFromConsole()
	if err != nil {
		fmt.Println(err)
		return "", "", trace.Wrap(err)
	}
	return pwd, token, nil
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

func authMethodFromAgent(ag agent.Agent) ssh.AuthMethod {
	return ssh.PublicKeysCallback(ag.Signers)
}

// Executes the given command on the client machine (localhost). If no command is given,
// executes shell
func runLocalCommand(command []string) error {
	if len(command) == 0 {
		user, err := user.Current()
		if err != nil {
			return trace.Wrap(err)
		}
		shell, err := utils.GetLoginShell(user.Username)
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
