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
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
)

type Config struct {
	// teleport user login
	Login string

	// Remote host to connect
	Host string

	// Host Labels
	Labels map[string]string

	// User login on a remote host
	HostLogin string

	// Remote host port
	HostPort int

	// host or IP of the proxy (with optional ":port")
	ProxyHost string

	// TTL for the temporary SSH keypair to remain valid:
	KeyTTL time.Duration

	// InsecureSkipVerify is an option to skip HTTPS cert check
	InsecureSkipVerify bool
}

// Returns a full host:port address of the proxy or an empty string if no
// proxy is given. If 'forWeb' flag is set, returns HTTPS port, otherwise
// returns SSH port (proxy servers listen on both)
func (c *Config) ProxyHostPort(defaultPort int) string {
	if c.ProxySpecified() {
		port := fmt.Sprintf("%d", defaultPort)
		return net.JoinHostPort(c.ProxyHost, port)
	} else {
		return ""
	}
}

func (c *Config) NodeHostPort() string {
	return net.JoinHostPort(c.Host, strconv.FormatInt(int64(c.HostPort), 10))
}

func (c *Config) ProxySpecified() bool {
	return len(c.ProxyHost) > 0
}

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
		return nil, trace.Errorf("no proxy address specified")
	}
	if c.Host == "" {
		return nil, trace.Errorf("no remote host specified")
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
		tc.authMethods = append(tc.authMethods, AuthMethodFromAgent(sshAgent))
	}

	// then, we can authenticate via a locally stored cert previously
	// signed by the CA:
	localAgent, err := GetLocalAgent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if localAgent != nil {
		tc.localAgent = localAgent
		tc.authMethods = append(tc.authMethods, AuthMethodFromAgent(localAgent))
	} else {
		log.Errorf("unable to obtain locally stored credentials")
	}

	// finally, interactive auth (via password + HTOP 2nd factor):
	tc.authMethods = append(tc.authMethods, tc.makeInteractiveAuthMethod())
	return tc, nil
}

// SSH connects to a node and, if 'command' is specified, executes the command on it,
// otherwise runs interactive shell
func (tc *TeleportClient) SSH(command string) (err error) {
	// connecting via proxy?
	if !tc.Config.ProxySpecified() {
		return trace.Wrap(fmt.Errorf("proxy server is not specified"))
	}
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()
	if len(command) > 0 {
		return tc.runCommand(proxyClient, command)
	}
	nodeAddr := tc.NodeHostPort()
	log.Debugf("connecting to node %v via proxy %v", nodeAddr, proxyClient.proxyAddress)
	nodeClient, err := proxyClient.ConnectToNode(nodeAddr,
		tc.authMethods, tc.makeHostKeyCallback(), tc.Config.HostLogin)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nodeClient.Close()
	return tc.runShell(nodeClient, "")
}

// ListNodes returns a list of nodes connected to a proxy
func (tc *TeleportClient) ListNodes() ([]services.Server, error) {
	// connect to the proxy and ask it to return a full list of servers
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()
	var servers []services.Server
	servers, err = proxyClient.GetServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return filterByLabels(servers, tc.Config.Labels), nil
}

// filterByLabels takes a list of nodes and returns only nodes that have matching labels.
// If no labels are given, returns the original list
func filterByLabels(nodes []services.Server, labels map[string]string) (output []services.Server) {
	if labels == nil || len(nodes) == 0 {
		return nodes
	}
	output = make([]services.Server, 0)
	// go over all nodes
	for _, node := range nodes {
		match := false
		// look at each node's labels
		if node.Labels != nil {
			for ln, lv := range labels {
				if node.Labels[ln] == lv {
					match = true
					break
				}
			}
		}
		// look at each node's command labels
		if !match && node.CmdLabels != nil {
			for cln, clv := range node.CmdLabels {
				if node.Labels[cln] == clv.Result {
					match = true
					break
				}
			}
		}
		if match {
			output = append(output, node)
		}
	}
	return output
}

// runCommand executes a given bash command on a bunch of remote nodes
func (tc *TeleportClient) runCommand(proxyClient *ProxyClient, command string) error {
	stdoutMutex := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	// build a list of nodes we should be executing this command on:
	var nodeAddresses []string
	if proxyClient != nil && tc.Config.Labels != nil {
		nodeAddresses = make([]string, 0)
		servers, err := proxyClient.GetServers()
		if err != nil {
			return trace.Wrap(err)
		}
		for _, server := range filterByLabels(servers, tc.Config.Labels) {
			nodeAddresses = append(nodeAddresses, server.Addr)
		}
	} else {
		nodeAddresses = []string{tc.NodeHostPort()}
	}

	for _, address := range nodeAddresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			nodeClient, err := ConnectToNode(proxyClient, address, tc.authMethods,
				tc.makeHostKeyCallback(), tc.Config.Login)
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

	// disable input buffering
	exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
	// do not display entered characters on the screen
	exec.Command("stty", "-F", "/dev/tty", "-echo").Run()

	// Catch term signals
	exitSignals := make(chan os.Signal, 1)
	signal.Notify(exitSignals, syscall.SIGTERM)
	go func() {
		<-exitSignals
		fmt.Printf("\nConnection to %s closed\n", address)
		// restore the console echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		os.Exit(0)
	}()

	width, height, err := getTerminalSize()
	if err != nil {
		// restore the console echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		return trace.Wrap(err)
	}

	shell, err := nodeClient.Shell(width, height, sessionID)
	if err != nil {
		// restore the console echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
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

	wg := &sync.WaitGroup{}
	wg.Add(2)

	// copy from the remote shell to the local
	go func() {
		_, err := io.Copy(os.Stdout, shell)
		if err != nil {
			log.Errorf(err.Error())
		}
		fmt.Printf("\nConnection to %s closed from the remote side\n", address)
		// restore the console echoing state when exiting
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		os.Exit(0)
		wg.Done()
	}()

	// copy from the local shell to the remote
	go func() {
		defer wg.Done()
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
					// restore the console echoing state when exiting
					exec.Command("stty", "-F", "/dev/tty", "echo").Run()
					os.Exit(0)
				}
				_, err = shell.Write(buf[:n])
				if err != nil {
					fmt.Println(trace.Wrap(err))
					return
				}
			}
		}
	}()

	wg.Wait()
	// restore the console echoing state when exiting
	exec.Command("stty", "-F", "/dev/tty", "echo").Run()
	return nil
}

// getTerminalSize() returns current local terminal size
func getTerminalSize() (width int, height int, e error) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	n, err := fmt.Sscan(string(out), &height, &width)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}
	if n != 2 {
		return 0, 0, trace.Errorf("Can't get terminal size")
	}

	return width, height, nil
}

// ConnectToProxy dials the proxy server and returns ProxyClient if successful
func (tc *TeleportClient) ConnectToProxy() (*ProxyClient, error) {
	proxyAddr := tc.Config.ProxyHostPort(defaults.SSHProxyListenPort)
	sshConfig := &ssh.ClientConfig{
		User:            tc.Config.Login,
		HostKeyCallback: tc.makeHostKeyCallback(),
	}
	log.Debugf("connecting to proxy: %v", proxyAddr)

	if len(tc.authMethods) == 0 {
		return nil, trace.Errorf("no authentication methods provided")
	}

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
			Client:       proxyClient,
			proxyAddress: proxyAddr,
		}, nil
	}
	return nil, trace.Errorf("could not connect to proxy %v. all authentication methods failed", proxyAddr)
}

// makeHostKeyCallback creates and returns a function suitable to be passed into
// ssh.ClientConfig
func (tc *TeleportClient) makeHostKeyCallback() utils.HostKeyCallback {
	return func(hostID string, remote net.Addr, key ssh.PublicKey) error {
		err := CheckHostSignerFromCache(hostID, remote, key)
		if err != nil {
			err = tc.Login()
			if err != nil {
				log.Error(err)
				return trace.Wrap(err)
			}
			return CheckHostSignerFromCache(hostID, remote, key)
		}
		return nil
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
	keyPath := filepath.Join(KeysDir, KeyFilePrefix+strconv.FormatInt(rand.Int63n(100), 16)+KeyFileSuffix)
	err = saveKey(key, keyPath)
	if err != nil {
		return trace.Wrap(err)
	}
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
	// tokenize the label spec:
	for i, ch := range spec {
		endOfToken := (i+1 == len(spec))
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
	// simple validation of tokenization: must have even number of tokens (because they're pairs)
	// and the number of such pairs must be equal the number of assignments
	if len(tokens)%2 != 0 || assignCount != len(tokens)/2 {
		return nil, fmt.Errorf("invalid label spec: '%s'", spec)
	}
	// break tokens in pairs and put into a map:
	labels := make(map[string]string)
	for i := 0; i < len(tokens); i += 2 {
		labels[tokens[i]] = tokens[i+1]
	}
	return labels, nil
}
