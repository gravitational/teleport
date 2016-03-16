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
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
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

// Config is a client config
type Config struct {
	// Login is a teleport user login
	Login string

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
}

// ProxyHostPort returns a full host:port address of the proxy or an empty string if no
// proxy is given. If 'forWeb' flag is set, returns HTTPS port, otherwise
// returns SSH port (proxy servers listen on both)
func (c *Config) ProxyHostPort(defaultPort int) string {
	if c.ProxySpecified() {
		port := fmt.Sprintf("%d", defaultPort)
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
	localAgent  agent.Agent
	authMethods []ssh.AuthMethod
}

// NewClient creates a TeleportClient object and fully configures it
func NewClient(c *Config) (tc *TeleportClient, err error) {
	// validate configuration
	if c.Login == "" {
		c.Login = Username()
		log.Infof("no teleport login given. defaulting to %s", c.Login)
	}
	if c.ProxyHost == "" {
		return nil, trace.Wrap(teleport.BadParameter("proxy", "no proxy address specified"))
	}
	if c.HostLogin == "" {
		c.HostLogin = c.Login
		log.Infof("no host login given. defaulting to %s", c.HostLogin)
	}
	if c.KeyTTL == 0 {
		c.KeyTTL = defaults.CertDuration
	} else if c.KeyTTL > defaults.MaxCertDuration || c.KeyTTL < defaults.MinCertDuration {
		return nil, trace.Errorf("invalid requested cert TTL")
	}

	tc = &TeleportClient{
		Config:      *c,
		authMethods: make([]ssh.AuthMethod, 0),
	}

	// first, see if we can authenticate with credentials stored in
	// a local SSH agent:
	if sshAgent := connectToSSHAgent(); sshAgent != nil {
		tc.authMethods = append(tc.authMethods, authMethodFromAgent(sshAgent))
	}

	// then, we can authenticate via a locally stored cert previously
	// signed by the CA:
	localAgent, err := GetLocalAgent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if localAgent != nil {
		tc.localAgent = localAgent
		tc.authMethods = append(tc.authMethods, authMethodFromAgent(localAgent))
	} else {
		log.Errorf("unable to obtain locally stored credentials")
	}

	// finally, interactive auth (via password + HTOP 2nd factor):
	tc.authMethods = append(tc.authMethods, tc.makeInteractiveAuthMethod())
	return tc, nil
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
func (tc *TeleportClient) SSH(command string) (err error) {
	// connect to proxy first:
	if !tc.Config.ProxySpecified() {
		return trace.Wrap(teleport.BadParameter("server", "proxy server is not specified"))
	}
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	// which nodes are we executing this commands on?
	nodeAddrs, err := tc.getTargetNodes(proxyClient)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(nodeAddrs) == 0 {
		return trace.Wrap(teleport.BadParameter("host", "no target host specified"))
	}

	// execute non-interactive SSH command:
	if len(command) > 0 {
		return tc.runCommand(nodeAddrs, proxyClient, command)
	}

	// more than one node for an interactive shell?
	// that can't be!
	if len(nodeAddrs) != 1 {
		return trace.Errorf("Cannot launch shell on multiple nodes: %v", nodeAddrs)
	}

	// execute SSH shell on a single node:
	nodeClient, err := proxyClient.ConnectToNode(nodeAddrs[0], tc.Config.HostLogin)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nodeClient.Close()
	return tc.runShell(nodeClient, "")
}

// Join connects to the existing/active SSH session
func (tc *TeleportClient) Join(sessionID string) (err error) {
	var notFoundError = &teleport.NotFoundError{Message: "Session not found or it has ended"}
	// connect to proxy:
	if !tc.Config.ProxySpecified() {
		return trace.Wrap(teleport.BadParameter("server", "proxy server is not specified"))
	}
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()
	// connect to the first site via proxy:
	sites, err := proxyClient.GetSites()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(sites) == 0 {
		return trace.Wrap(notFoundError)
	}
	site, err := proxyClient.ConnectToSite(sites[0].Name, tc.Config.HostLogin)
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
		return trace.Wrap(notFoundError)
	}
	// pick the 1st party of the session and use his server ID to connect to
	if len(session.Parties) == 0 {
		return trace.Wrap(notFoundError)
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
		return trace.Wrap(notFoundError)
	}
	// connect to server:
	nc, err := proxyClient.ConnectToNode(node.Addr, tc.Config.HostLogin)
	if err != nil {
		return trace.Wrap(err)
	}
	return tc.runShell(nc, session.ID)
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
		return trace.Errorf("Making local copies is not supported")
	}

	if !tc.Config.ProxySpecified() {
		return trace.Wrap(teleport.BadParameter("server", "proxy server is not specified"))
	}
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	// upload:
	if isRemoteDest(last) {
		host, dest := parseSCPDestination(last)
		addr := net.JoinHostPort(host, strconv.Itoa(port))

		client, err := proxyClient.ConnectToNode(addr, tc.HostLogin)
		if err != nil {
			return trace.Wrap(err)
		}
		// copy everything except the last arg (that's destination)
		for _, src := range args[:len(args)-1] {
			err = client.Upload(src, dest)
			if err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("uploaded %s\n", src)
		}
		// download:
	} else {
		host, src := parseSCPDestination(first)
		addr := net.JoinHostPort(host, strconv.Itoa(port))

		client, err := proxyClient.ConnectToNode(addr, tc.HostLogin)
		if err != nil {
			return trace.Wrap(err)
		}
		// copy everything except the last arg (that's destination)
		for _, dest := range args[1:] {
			err = client.Download(src, dest, recursive)
			if err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("downloaded %s\n", src)
		}
	}
	return nil
}

func parseSCPDestination(s string) (host, dest string) {
	parts := strings.Split(s, ":")
	return parts[0], strings.Join(parts[1:], ":")
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
func (tc *TeleportClient) runCommand(nodeAddresses []string, proxyClient *ProxyClient, command string) error {
	stdoutMutex := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	for _, address := range nodeAddresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			nodeClient, err := proxyClient.ConnectToNode(address, tc.Config.Login)
			if err != nil {
				fmt.Println(err)
			}
			defer nodeClient.Close()

			// run the command on one node:
			out := bytes.Buffer{}
			err = nodeClient.Run(command, &out)
			if err != nil {
				fmt.Println(err)
			}

			stdoutMutex.Lock()
			defer stdoutMutex.Unlock()
			fmt.Printf("Running command on %v:\n", address)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf(out.String())
			}
		}(address)
	}
	wg.Wait()
	return nil
}

// runShell starts an interactive SSH session/shell
func (tc *TeleportClient) runShell(nodeClient *NodeClient, sessionID string) error {
	defer nodeClient.Close()
	address := tc.NodeHostPort()

	// If typing on the terminal, we do not want the terminal to echo the
	// password that is typed (so it doesn't display)
	if term.IsTerminal(0) {
		state, err := term.SetRawTerminal(0)
		if err != nil {
			return trace.Wrap(err)
		}
		defer term.RestoreTerminal(0, state)
	}

	broadcastClose := utils.NewCloseBroadcaster()

	// Catch term signals
	exitSignals := make(chan os.Signal, 1)
	signal.Notify(exitSignals, syscall.SIGTERM)
	go func() {
		defer broadcastClose.Close()
		<-exitSignals
		fmt.Printf("\nConnection to %s closed\n", address)
	}()

	winSize, err := term.GetWinsize(0)
	if err != nil {
		return trace.Wrap(err)
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
		_, err := io.Copy(os.Stdout, shell)
		if err != nil {
			log.Errorf(err.Error())
		}
		fmt.Printf("\nConnection to %s closed from the remote side\n", address)
	}()

	// copy from the local shell to the remote
	go func() {
		defer broadcastClose.Close()
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				fmt.Println(trace.Wrap(err))
				return
			}
			if n > 0 {
				// catch Ctrl-D
				if buf[0] == 4 {
					fmt.Printf("\nConnection to %s closed\n", address)
					return
				}
				_, err = shell.Write(buf[:n])
				if err != nil {
					fmt.Println(trace.Wrap(err))
					return
				}
			}
		}
	}()

	<-broadcastClose.C
	return nil
}

// ConnectToProxy dials the proxy server and returns ProxyClient if successful
func (tc *TeleportClient) ConnectToProxy() (*ProxyClient, error) {
	proxyAddr := tc.Config.ProxyHostPort(defaults.SSHProxyListenPort)
	sshConfig := &ssh.ClientConfig{
		User:            tc.Config.Login,
		HostKeyCallback: CheckHostSignature,
	}
	if len(tc.authMethods) == 0 {
		return nil, trace.Errorf("no authentication methods provided")
	}
	log.Debugf("connecting to proxy: %v", proxyAddr)

	for {
		// try to authenticate using every auth method we have:
		for _, m := range tc.authMethods {
			sshConfig.Auth = []ssh.AuthMethod{m}
			proxyClient, err := ssh.Dial("tcp", proxyAddr, sshConfig)
			if err != nil {
				if utils.IsHandshakeFailedError(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}
			return &ProxyClient{
				Client:          proxyClient,
				proxyAddress:    proxyAddr,
				hostKeyCallback: sshConfig.HostKeyCallback,
				authMethods:     tc.authMethods,
				hostLogin:       tc.Config.HostLogin,
			}, nil
		}
		// if we get here, it means we failed to authenticate using stored keys
		// and we need to ask for the login information
		tc.Login()
	}
}

// makeInteractiveAuthMethod creates an 'ssh.AuthMethod' which authenticates
// interactively by asknig a Teleport password + HTOP token
func (tc *TeleportClient) makeInteractiveAuthMethod() ssh.AuthMethod {
	callbackFunc := func() (signers []ssh.Signer, err error) {
		if err = tc.Login(); err != nil {
			log.Error(err)
			return nil, trace.Wrap(err)
		}
		return tc.localAgent.Signers()
	}
	return ssh.PublicKeysCallback(callbackFunc)
}

// login asks for a password + HOTP token, makes a request to CA via proxy and
// saves the generated credentials into local keystore for future use
func (tc *TeleportClient) Login() error {
	password, hotpToken, err := tc.AskPasswordAndHOTP()
	if err != nil {
		return trace.Wrap(err)
	}

	// generate a new keypair. the public key will be signed via proxy if our password+HOTP
	// are legit
	priv, pub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}

	// ask the CA (via proxy) to sign our public key:
	response, err := web.SSHAgentLogin(tc.Config.ProxyHostPort(defaults.HTTPListenPort), tc.Config.Login,
		password, hotpToken, pub, tc.KeyTTL, tc.InsecureSkipVerify)
	if err != nil {
		return trace.Wrap(err)
	}

	// parse the returned&signed key:
	pcert, _, _, _, err := ssh.ParseAuthorizedKey(response.Cert)
	if err != nil {
		return trace.Wrap(err)
	}
	pk, err := ssh.ParseRawPrivateKey(priv)
	if err != nil {
		return trace.Wrap(err)
	}

	// store the newly generated key in the local key store
	addedKey := agent.AddedKey{
		PrivateKey:       pk,
		Certificate:      pcert.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}
	if err := tc.localAgent.Add(addedKey); err != nil {
		return trace.Wrap(err)
	}
	key := Key{
		Priv:     priv,
		Cert:     response.Cert,
		Deadline: time.Now().Add(tc.KeyTTL),
	}
	keyPath := filepath.Join(getKeysDir(), KeyFilePrefix+strconv.FormatInt(rand.Int63n(100), 16)+KeyFileSuffix)
	err = saveKey(key, keyPath)
	if err != nil {
		return trace.Wrap(err)
	}
	// save the list of CAs we trust to the cache file
	err = AddHostSignersToCache(response.HostSigners)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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

// username returns the current user's username
func Username() string {
	u, err := user.Current()
	if err != nil {
		utils.FatalError(err)
	}
	return u.Username
}

// AskPasswordAndHOTP prompts the user to enter the password + HTOP 2nd factor
func (tc *TeleportClient) AskPasswordAndHOTP() (pwd string, token string, err error) {
	fmt.Printf("Enter password for %v:\n", tc.Config.Login)
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

// parseLabelSpec parses a string like 'name=value,"long name"="quoted value"` into a map like
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
					assignCount += 1
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
