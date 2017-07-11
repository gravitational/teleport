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

package common

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/Sirupsen/logrus"
	"github.com/buger/goterm"
	gops "github.com/google/gops/agent"
)

// CLIConf stores command line arguments and flags:
type CLIConf struct {
	// UserHost contains "[login]@hostname" argument to SSH command
	UserHost string
	// Commands to execute on a remote host
	RemoteCommand []string
	// Username is the Teleport user's username (to login into proxies)
	Username string
	// Proxy keeps the hostname:port of the SSH proxy to use
	Proxy string
	// TTL defines how long a session must be active (in minutes)
	MinsToLive int32
	// SSH Port on a remote SSH host
	NodePort int16
	// Login on a remote SSH host
	NodeLogin string
	// InsecureSkipVerify bypasses verification of HTTPS certificate when talking to web proxy
	InsecureSkipVerify bool
	// IsUnderTest is set to true for unit testing
	IsUnderTest bool
	// AgentSocketAddr is address for agent listeing socket
	AgentSocketAddr utils.NetAddrVal
	// Remote SSH session to join
	SessionID string
	// Src:dest parameter for SCP
	CopySpec []string
	// -r flag for scp
	RecursiveCopy bool
	// -L flag for ssh. Local port forwarding like 'ssh -L 80:remote.host:80 -L 443:remote.host:443'
	LocalForwardPorts []string
	// --local flag for ssh
	LocalExec bool
	// SiteName specifies remote site go login to
	SiteName string
	// Interactive, when set to true, launches remote command with the terminal attached
	Interactive bool
	// Quiet mode, -q command (disables progress printing)
	Quiet bool
	// Namespace is used to select cluster namespace
	Namespace string
	// NoCache is used to turn off client cache for nodes discovery
	NoCache bool
	// LoadSystemAgentOnly when set to true will cause tsh agent to load keys into the system agent and
	// then exit. This is useful when calling tsh agent from a script (for example ~/.bash_profile)
	// to load keys into your system agent.
	LoadSystemAgentOnly bool
	// BenchThreads is amount of concurrent threads to run
	BenchThreads int
	// BenchDuration is a duration for the benchmark
	BenchDuration time.Duration
	// BenchRate is a requests per second rate to mantain
	BenchRate int
	// BenchInteractive indicates that we should create interactive session
	BenchInteractive bool
	// Context is a context to control execution
	Context context.Context
	// Gops starts gops agent on a specified address
	// if not specified, gops won't start
	Gops bool
	// GopsAddr specifies to gops addr to listen on
	GopsAddr string
	// IdentityFile is an argument to -i flag (path to the private key+cert file)
	IdentityFile string
	// Compatibility flags, --compat, specifies OpenSSH compatibility flags.
	Compatibility string
}

// Run executes TSH client. same as main() but easier to test
func Run(args []string, underTest bool) {
	var cf CLIConf
	cf.IsUnderTest = underTest
	utils.InitLogger(utils.LoggingForCLI, logrus.WarnLevel)

	// configure CLI argument parser:
	app := utils.InitCLIParser("tsh", "TSH: Teleport SSH client").Interspersed(false)
	app.Flag("login", "Remote host login").Short('l').Envar("TELEPORT_LOGIN").StringVar(&cf.NodeLogin)
	localUser, _ := client.Username()
	app.Flag("proxy", "SSH proxy host or IP address").Envar("TELEPORT_PROXY").StringVar(&cf.Proxy)
	app.Flag("nocache", "do not cache cluster discovery locally").Hidden().BoolVar(&cf.NoCache)
	app.Flag("user", fmt.Sprintf("SSH proxy user [%s]", localUser)).Envar("TELEPORT_USER").StringVar(&cf.Username)
	app.Flag("cluster", "Specify the cluster to connect").Envar("TELEPORT_SITE").StringVar(&cf.SiteName)
	app.Flag("ttl", "Minutes to live for a SSH session").Int32Var(&cf.MinsToLive)
	app.Flag("identity", "Identity file").Short('i').StringVar(&cf.IdentityFile)
	app.Flag("compat", "OpenSSH compatibility flag").StringVar(&cf.Compatibility)

	app.Flag("insecure", "Do not verify server's certificate and host name. Use only in test environments").Default("false").BoolVar(&cf.InsecureSkipVerify)
	app.Flag("namespace", "Namespace of the cluster").Default(defaults.Namespace).Hidden().StringVar(&cf.Namespace)
	app.Flag("gops", "Start gops endpoint on a given address").Hidden().BoolVar(&cf.Gops)
	app.Flag("gops-addr", "Specify gops addr to listen on").Hidden().StringVar(&cf.GopsAddr)
	debugMode := app.Flag("debug", "Verbose logging to stdout").Short('d').Bool()
	app.HelpFlag.Short('h')
	ver := app.Command("version", "Print the version")
	// ssh
	ssh := app.Command("ssh", "Run shell or execute a command on a remote SSH node")
	ssh.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cf.UserHost)
	ssh.Arg("command", "Command to execute on a remote host").StringsVar(&cf.RemoteCommand)
	ssh.Flag("port", "SSH port on a remote host").Short('p').Int16Var(&cf.NodePort)
	ssh.Flag("forward", "Forward localhost connections to remote server").Short('L').StringsVar(&cf.LocalForwardPorts)
	ssh.Flag("local", "Execute command on localhost after connecting to SSH node").Default("false").BoolVar(&cf.LocalExec)
	ssh.Flag("", "Allocate TTY").Short('t').BoolVar(&cf.Interactive)
	// join
	join := app.Command("join", "Join the active SSH session")
	join.Arg("session-id", "ID of the session to join").Required().StringVar(&cf.SessionID)
	// play
	play := app.Command("play", "Replay the recorded SSH session")
	play.Arg("session-id", "ID of the session to play").Required().StringVar(&cf.SessionID)
	// scp
	scp := app.Command("scp", "Secure file copy")
	scp.Arg("from, to", "Source and destination to copy").Required().StringsVar(&cf.CopySpec)
	scp.Flag("recursive", "Recursive copy of subdirectories").Short('r').BoolVar(&cf.RecursiveCopy)
	scp.Flag("port", "Port to connect to on the remote host").Short('P').Int16Var(&cf.NodePort)
	scp.Flag("quiet", "Quiet mode").Short('q').BoolVar(&cf.Quiet)
	// ls
	ls := app.Command("ls", "List remote SSH nodes")
	ls.Arg("labels", "List of labels to filter node list").StringVar(&cf.UserHost)
	// clusters
	clusters := app.Command("clusters", "List available Teleport clusters")
	clusters.Flag("quiet", "Quiet mode").Short('q').BoolVar(&cf.Quiet)
	// agent (SSH agent listening on unix socket)
	agent := app.Command("agent", "Start SSH agent on unix socket [deprecating soon]")
	agent.Flag("socket", "SSH agent listening socket address, e.g. unix:///tmp/teleport.agent.sock").SetValue(&cf.AgentSocketAddr)
	agent.Flag("load", "When set to true, the tsh agent will load the external system agent and then exit.").BoolVar(&cf.LoadSystemAgentOnly)

	// login logs in with remote proxy and obtains a "session certificate" which gets
	// stored in ~/.tsh directory
	login := app.Command("login", "Log in to the cluster and store the session certificate to avoid login prompts")

	// logout deletes obtained session certificates in ~/.tsh
	logout := app.Command("logout", "Delete a cluster certificate")

	// bench
	bench := app.Command("bench", "Run shell or execute a command on a remote SSH node").Hidden()
	bench.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cf.UserHost)
	bench.Arg("command", "Command to execute on a remote host").Required().StringsVar(&cf.RemoteCommand)
	bench.Flag("port", "SSH port on a remote host").Short('p').Int16Var(&cf.NodePort)
	bench.Flag("threads", "Concurrent threads to run").Default("10").IntVar(&cf.BenchThreads)
	bench.Flag("duration", "Test duration").Default("1s").DurationVar(&cf.BenchDuration)
	bench.Flag("rate", "Requests per second rate").Default("10").IntVar(&cf.BenchRate)
	bench.Flag("interactive", "Create interactive SSH session").BoolVar(&cf.BenchInteractive)

	// parse CLI commands+flags:
	command, err := app.Parse(args)
	if err != nil {
		utils.FatalError(err)
	}

	// apply -d flag:
	if *debugMode {
		utils.InitLogger(utils.LoggingForCLI, logrus.DebugLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		exitSignals := make(chan os.Signal, 1)
		signal.Notify(exitSignals, syscall.SIGTERM, syscall.SIGINT)

		select {
		case sig := <-exitSignals:
			logrus.Debugf("signal: %v", sig)
			cancel()
		}
	}()
	cf.Context = ctx

	if cf.Gops {
		logrus.Debugf("starting gops agent")
		err = gops.Listen(&gops.Options{Addr: cf.GopsAddr})
		if err != nil {
			logrus.Warningf("failed to start gops agent %v", err)
		}
	}

	switch command {
	case ver.FullCommand():
		onVersion()
	case ssh.FullCommand():
		onSSH(&cf)
	case bench.FullCommand():
		onBenchmark(&cf)
	case join.FullCommand():
		onJoin(&cf)
	case scp.FullCommand():
		onSCP(&cf)
	case play.FullCommand():
		onPlay(&cf)
	case ls.FullCommand():
		onListNodes(&cf)
	case clusters.FullCommand():
		onListSites(&cf)
	case agent.FullCommand():
		onAgentStart(&cf)
	case login.FullCommand():
		refuseArgs(login.FullCommand(), args)
		onLogin(&cf)
	case logout.FullCommand():
		refuseArgs(logout.FullCommand(), args)
		onLogout(&cf)
	}
}

// onPlay replays a session with a given ID
func onPlay(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	if err := tc.Play(context.TODO(), cf.Namespace, cf.SessionID); err != nil {
		utils.FatalError(err)
	}
}

// onLogin logs in with remote proxy and gets signed certificates
func onLogin(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}

	if _, err := tc.Login(); err != nil {
		utils.FatalError(err)
	}
	tc.SaveProfile("")

	if tc.SiteName != "" {
		fmt.Printf("\nYou are now logged into %s as %s\n", tc.SiteName, tc.Username)
	} else {
		fmt.Printf("\nYou are now logged in\n")
	}
}

// onLogout deletes a "session certificate" from ~/.tsh for a given proxy
func onLogout(cf *CLIConf) {
	client.UnlinkCurrentProfile()

	// logout from all
	if cf.Proxy == "" {
		client.LogoutFromEverywhere(cf.Username)
	} else {
		tc, err := makeClient(cf, true)
		if err != nil {
			utils.FatalError(err)
		}
		if err = tc.Logout(); err != nil {
			if trace.IsNotFound(err) {
				utils.FatalError(trace.Errorf("you are not logged into proxy '%s'", cf.Proxy))
			}
			utils.FatalError(err)
		}
		fmt.Printf("%s has logged out of %s\n", tc.Username, cf.SiteName)
	}
}

// onListNodes executes 'tsh ls' command
func onListNodes(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	servers, err := tc.ListNodes(context.TODO())
	if err != nil {
		utils.FatalError(err)
	}
	nodesView := func(nodes []services.Server) string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Node Name", "Node ID", "Address", "Labels"})
		if len(nodes) == 0 {
			return t.String()
		}
		for _, n := range nodes {
			fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", n.GetHostname(), n.GetName(), n.GetAddr(), n.LabelsString())
		}
		return t.String()
	}
	fmt.Printf(nodesView(servers))
}

// onListSites executes 'tsh sites' command
func onListSites(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		utils.FatalError(err)
	}
	defer proxyClient.Close()

	sites, err := proxyClient.GetSites()
	if err != nil {
		utils.FatalError(err)
	}
	sitesView := func() string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Cluster Name", "Status"})
		if len(sites) == 0 {
			return t.String()
		}
		for _, site := range sites {
			fmt.Fprintf(t, "%v\t%v\n", site.Name, site.Status)
		}
		return t.String()
	}
	quietSitesView := func() string {
		names := make([]string, 0)
		for _, site := range sites {
			names = append(names, site.Name)
		}
		return strings.Join(names, "\n")
	}
	if cf.Quiet {
		sitesView = quietSitesView
	}
	fmt.Printf(sitesView())
}

// onSSH executes 'tsh ssh' command
func onSSH(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}

	tc.Stdin = os.Stdin
	if err = tc.SSH(context.TODO(), cf.RemoteCommand, cf.LocalExec); err != nil {
		// exit with the same exit status as the failed command:
		if tc.ExitStatus != 0 {
			fmt.Fprintln(os.Stderr, utils.UserMessageFromError(err))
			os.Exit(tc.ExitStatus)
		} else {
			utils.FatalError(err)
		}
	}
}

// onBenchmark executes benchmark
func onBenchmark(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}
	result, err := tc.Benchmark(cf.Context, client.Benchmark{
		Command:  cf.RemoteCommand,
		Threads:  cf.BenchThreads,
		Duration: cf.BenchDuration,
		Rate:     cf.BenchRate,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, utils.UserMessageFromError(err))
		os.Exit(255)
	}
	fmt.Printf("\n")
	fmt.Printf("* Requests originated: %v\n", result.RequestsOriginated)
	fmt.Printf("* Requests failed: %v\n", result.RequestsFailed)
	if result.LastError != nil {
		fmt.Printf("* Last error: %v\n", result.LastError)
	}
	fmt.Printf("\nHistogram\n\n")
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	printHeader(t, []string{"Percentile", "Duration"})
	for _, quantile := range []float64{25, 50, 75, 90, 95, 99, 100} {
		fmt.Fprintf(t, "%v\t%v ms\n", quantile, result.Histogram.ValueAtQuantile(quantile))
	}

	fmt.Fprintf(os.Stdout, t.String())
	fmt.Printf("\n")
}

// onJoin executes 'ssh join' command
func onJoin(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	sid, err := session.ParseID(cf.SessionID)
	if err != nil {
		utils.FatalError(fmt.Errorf("'%v' is not a valid session ID (must be GUID)", cf.SessionID))
	}
	if err = tc.Join(context.TODO(), cf.Namespace, *sid, nil); err != nil {
		utils.FatalError(err)
	}
}

// onSCP executes 'tsh scp' command
func onSCP(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}
	if err := tc.SCP(context.TODO(), cf.CopySpec, int(cf.NodePort), cf.RecursiveCopy, cf.Quiet); err != nil {
		// exit with the same exit status as the failed command:
		if tc.ExitStatus != 0 {
			os.Exit(tc.ExitStatus)
		} else {
			utils.FatalError(err)
		}
	}
}

// onAgentStart start ssh agent on a socket
func onAgentStart(cf *CLIConf) {
	const warning = "\x1b[1mWARNING:\x1b[0m 'tsh agent' will be deprecated in the next Teleport release.\n" +
		"Use 'ssh-agent' supplied by your operating system instead. \n" +
		"'tsh login' now saves the session keys in ssh-agent automatically.\n"

	fmt.Fprintln(os.Stderr, warning)

	// create a client, a side effect of this is that it creates a client.LocalAgent.
	// creation of a client.LocalAgent has a side effect of loading all keys into
	// client.LocalAgent and the system agent.
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}

	// check if we are only loading keys and exiting. this is useful
	// when calling tsh agent from a script like ~/.bash_profile.
	if cf.LoadSystemAgentOnly {
		return
	}

	// we're starting tsh agent, build the socket address
	socketAddr := utils.NetAddr(cf.AgentSocketAddr)
	pid := os.Getpid()
	if socketAddr.IsEmpty() {
		socketAddr = utils.NetAddr{
			AddrNetwork: "unix",
			Addr:        filepath.Join(os.TempDir(), fmt.Sprintf("teleport-%d.socket", pid)),
		}
	}

	// This makes teleport agent behave exactly like ssh-agent command,
	// the output and behavior matches the openssh behavior,
	// so users can do 'eval $(tsh agent --proxy=<addr>&)
	fmt.Printf(`# Keep this agent process running in the background.
# Set these environment variables:
export SSH_AUTH_SOCK=%v;
export SSH_AGENT_PID=%v

# you can redirect this output into a file and call 'source' on it
`, socketAddr.Addr, pid)

	// create a new teleport agent and start listening
	agentServer := teleagent.AgentServer{
		Agent: tc.LocalAgent(),
	}
	if err := agentServer.ListenAndServe(socketAddr); err != nil {
		utils.FatalError(err)
	}
}

// makeClient takes the command-line configuration and constructs & returns
// a fully configured TeleportClient object
func makeClient(cf *CLIConf, useProfileLogin bool) (tc *client.TeleportClient, err error) {
	// apply defaults
	if cf.MinsToLive == 0 {
		cf.MinsToLive = int32(defaults.CertDuration / time.Minute)
	}

	// split login & host
	hostLogin := cf.NodeLogin
	var labels map[string]string
	if cf.UserHost != "" {
		parts := strings.Split(cf.UserHost, "@")
		if len(parts) > 1 {
			hostLogin = parts[0]
			cf.UserHost = parts[1]
		}
		// see if remote host is specified as a set of labels
		if strings.Contains(cf.UserHost, "=") {
			labels, err = client.ParseLabelSpec(cf.UserHost)
			if err != nil {
				return nil, err
			}
		}
	}
	fPorts, err := client.ParsePortForwardSpec(cf.LocalForwardPorts)
	if err != nil {
		return nil, err
	}

	// 1: start with the defaults
	c := client.MakeDefaultConfig()

	// Look if a user identity was given via -i flag
	if cf.IdentityFile != "" {
		var (
			key          *client.Key
			identityAuth ssh.AuthMethod
			expiryDate   time.Time
			hostAuthFunc client.HostKeyCallback
		)
		// read the ID file and create an "auth method" from it:
		key, hostAuthFunc, err = loadIdentity(cf.IdentityFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		identityAuth, err = authFromIdentity(key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		c.AuthMethods = []ssh.AuthMethod{identityAuth}
		if hostAuthFunc != nil {
			c.HostKeyCallback = hostAuthFunc
		}

		// check the expiration date
		expiryDate, _ = key.CertValidBefore()
		if expiryDate.Before(time.Now()) {
			fmt.Fprintf(os.Stderr, "WARNING: the certificate has expired on %v\n", expiryDate)
		}
	} else {
		// load profile. if no --proxy is given use ~/.tsh/profile symlink otherwise
		// fetch profile for exact proxy we are trying to connect to.
		err = c.LoadProfile("", cf.Proxy)
		if err != nil {
			fmt.Printf("WARNING: Failed to load tsh profile for %q: %v\n", cf.Proxy, err)
		}
	}

	// 3: override with the CLI flags
	if cf.Namespace != "" {
		c.Namespace = cf.Namespace
	}
	if cf.Username != "" {
		c.Username = cf.Username
	}
	if cf.Proxy != "" {
		c.ProxyHostPort = cf.Proxy
	}
	if len(fPorts) > 0 {
		c.LocalForwardPorts = fPorts
	}
	if cf.SiteName != "" {
		c.SiteName = cf.SiteName
	}
	// if host logins stored in profiles must be ignored...
	if !useProfileLogin {
		c.HostLogin = ""
	}
	if hostLogin != "" {
		c.HostLogin = hostLogin
	}
	c.Host = cf.UserHost
	c.HostPort = int(cf.NodePort)
	c.Labels = labels
	c.KeyTTL = time.Minute * time.Duration(cf.MinsToLive)
	c.InsecureSkipVerify = cf.InsecureSkipVerify
	c.Interactive = cf.Interactive
	if !cf.NoCache {
		c.CachePolicy = &client.CachePolicy{}
	}

	// parse compatibility parameter
	compatibility, err := utils.CheckCompatibilityFlag(cf.Compatibility)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.Compatibility = compatibility

	return client.NewClient(c)
}

func onVersion() {
	utils.PrintVersion()
}

func printHeader(t *goterm.Table, cols []string) {
	dots := make([]string, len(cols))
	for i := range dots {
		dots[i] = strings.Repeat("-", len(cols[i]))
	}
	fmt.Fprint(t, strings.Join(cols, "\t")+"\n")
	fmt.Fprint(t, strings.Join(dots, "\t")+"\n")
}

// refuseArgs helper makes sure that 'args' (list of CLI arguments)
// does not contain anything other than command
func refuseArgs(command string, args []string) {
	if len(args) == 0 {
		return
	}
	lastArg := args[len(args)-1]
	if lastArg != command {
		utils.FatalError(trace.BadParameter("%s does not expect arguments", command))
	}
}

// loadIdentity loads the private key + certificate from a file
// Returns:
//	 - client key: user's private key+cert
//   - host auth callback: function to validate the host (may be null)
//   - error, if somthing happens when reading the identityf file
//
// If the "host auth callback" is not returned, user will be prompted to
// trust the proxy server.
func loadIdentity(idFn string) (*client.Key, client.HostKeyCallback, error) {
	logrus.Infof("Reading identity file: ", idFn)

	f, err := os.Open(idFn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer f.Close()
	var (
		keyBuf bytes.Buffer
		state  int // 0: not found, 1: found beginning, 2: found ending
		cert   []byte
		caCert []byte
	)
	// read the identity file line by line:
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if state != 1 {
			if strings.HasPrefix(line, "ssh") {
				cert = []byte(line)
				continue
			}
			if strings.HasPrefix(line, "@cert-authority") {
				caCert = []byte(line)
				continue
			}
		}
		if state == 0 && strings.HasPrefix(line, "-----BEGIN") {
			state = 1
			keyBuf.WriteString(line)
			keyBuf.WriteRune('\n')
			continue
		}
		if state == 1 {
			keyBuf.WriteString(line)
			if strings.HasPrefix(line, "-----END") {
				state = 2
			} else {
				keyBuf.WriteRune('\n')
			}
		}
	}
	// did not find the certificate in the file? look in a separate file with
	// -cert.pub prefix
	if len(cert) == 0 {
		certFn := idFn + "-cert.pub"
		logrus.Infof("certificate not found in %s. looking in %s", idFn, certFn)
		cert, err = ioutil.ReadFile(certFn)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	// validate both by parsing them:
	privKey, err := ssh.ParseRawPrivateKey(keyBuf.Bytes())
	if err != nil {
		return nil, nil, trace.BadParameter("invalid identity: %s. %v", idFn, err)
	}
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var hostAuthFunc client.HostKeyCallback = nil
	// validate CA (cluster) cert
	if len(caCert) > 0 {
		_, _, pkey, _, _, err := ssh.ParseKnownHosts(caCert)
		if err != nil {
			return nil, nil, trace.BadParameter("CA cert parsing error: %v. cert line :%v",
				err.Error(), string(caCert))
		}
		// found CA cert in the indentity file? construct the host key checking function
		// and return it:
		hostAuthFunc = func(host string, a net.Addr, hostKey ssh.PublicKey) error {
			clusterCert, ok := hostKey.(*ssh.Certificate)
			if ok {
				hostKey = clusterCert.SignatureKey
			}
			if !sshutils.KeysEqual(pkey, hostKey) {
				err = trace.AccessDenied("host %v is untrusted", host)
				logrus.Error(err)
				return err
			}
			return nil
		}
	}
	return &client.Key{
		Priv: keyBuf.Bytes(),
		Pub:  signer.PublicKey().Marshal(),
		Cert: cert,
	}, hostAuthFunc, nil
}

// authFromIdentity returns a standard ssh.Authmethod for a given identity file
func authFromIdentity(k *client.Key) (ssh.AuthMethod, error) {
	signer, err := sshutils.NewSigner(k.Priv, k.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.NewAuthMethodForCert(signer), nil
}
