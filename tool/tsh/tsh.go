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

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	kubeclient "github.com/gravitational/teleport/lib/kube/client"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	gops "github.com/google/gops/agent"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTSH,
})

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
	NodePort int32
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
	// DynamicForwardedPorts is port forwarding using SOCKS5. It is similar to
	// "ssh -D 8080 example.com".
	DynamicForwardedPorts []string
	// ForwardAgent agent to target node. Equivalent of -A for OpenSSH.
	ForwardAgent bool
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
	// IdentityFileIn is an argument to -i flag (path to the private key+cert file)
	IdentityFileIn string
	// Compatibility flags, --compat, specifies OpenSSH compatibility flags.
	Compatibility string
	// CertificateFormat defines the format of the user SSH certificate.
	CertificateFormat string
	// IdentityFileOut is an argument to -out flag
	IdentityFileOut string
	// IdentityFormat (used for --format flag for 'tsh login') defines which
	// format to use with --out to store a fershly retreived certificate
	IdentityFormat client.IdentityFileFormat

	// AuthConnector is the name of the connector to use.
	AuthConnector string

	// SkipVersionCheck skips version checking for client and server
	SkipVersionCheck bool

	// Options is a list of OpenSSH options in the format used in the
	// configuration file.
	Options []string

	// Verbose is used to print extra output.
	Verbose bool
}

func main() {
	cmd_line_orig := os.Args[1:]
	cmd_line := []string{}

	// lets see: if the executable name is 'ssh' or 'scp' we convert
	// that to "tsh ssh" or "tsh scp"
	switch path.Base(os.Args[0]) {
	case "ssh":
		cmd_line = append([]string{"ssh"}, cmd_line_orig...)
	case "scp":
		cmd_line = append([]string{"scp"}, cmd_line_orig...)
	default:
		cmd_line = cmd_line_orig
	}
	Run(cmd_line, false)
}

const (
	clusterEnvVar = "TELEPORT_SITE"
	clusterHelp   = "Specify the cluster to connect"
)

// Run executes TSH client. same as main() but easier to test
func Run(args []string, underTest bool) {
	var cf CLIConf
	cf.IsUnderTest = underTest
	utils.InitLogger(utils.LoggingForCLI, logrus.WarnLevel)

	// configure CLI argument parser:
	app := utils.InitCLIParser("tsh", "TSH: Teleport SSH client").Interspersed(false)
	app.Flag("login", "Remote host login").Short('l').Envar("TELEPORT_LOGIN").StringVar(&cf.NodeLogin)
	localUser, _ := client.Username()
	app.Flag("proxy", "SSH proxy address").Envar("TELEPORT_PROXY").StringVar(&cf.Proxy)
	app.Flag("nocache", "do not cache cluster discovery locally").Hidden().BoolVar(&cf.NoCache)
	app.Flag("user", fmt.Sprintf("SSH proxy user [%s]", localUser)).Envar("TELEPORT_USER").StringVar(&cf.Username)

	app.Flag("ttl", "Minutes to live for a SSH session").Int32Var(&cf.MinsToLive)
	app.Flag("identity", "Identity file").Short('i').StringVar(&cf.IdentityFileIn)
	app.Flag("compat", "OpenSSH compatibility flag").Hidden().StringVar(&cf.Compatibility)
	app.Flag("cert-format", "SSH certificate format").StringVar(&cf.CertificateFormat)
	app.Flag("insecure", "Do not verify server's certificate and host name. Use only in test environments").Default("false").BoolVar(&cf.InsecureSkipVerify)
	app.Flag("auth", "Specify the type of authentication connector to use.").StringVar(&cf.AuthConnector)
	app.Flag("namespace", "Namespace of the cluster").Default(defaults.Namespace).Hidden().StringVar(&cf.Namespace)
	app.Flag("gops", "Start gops endpoint on a given address").Hidden().BoolVar(&cf.Gops)
	app.Flag("gops-addr", "Specify gops addr to listen on").Hidden().StringVar(&cf.GopsAddr)
	app.Flag("skip-version-check", "Skip version checking between server and client.").BoolVar(&cf.SkipVersionCheck)
	debugMode := app.Flag("debug", "Verbose logging to stdout").Short('d').Bool()
	app.HelpFlag.Short('h')
	ver := app.Command("version", "Print the version")
	// ssh
	ssh := app.Command("ssh", "Run shell or execute a command on a remote SSH node")
	ssh.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cf.UserHost)
	ssh.Arg("command", "Command to execute on a remote host").StringsVar(&cf.RemoteCommand)
	ssh.Flag("port", "SSH port on a remote host").Short('p').Int32Var(&cf.NodePort)
	ssh.Flag("forward-agent", "Forward agent to target node").Short('A').BoolVar(&cf.ForwardAgent)
	ssh.Flag("forward", "Forward localhost connections to remote server").Short('L').StringsVar(&cf.LocalForwardPorts)
	ssh.Flag("dynamic-forward", "Forward localhost connections to remote server using SOCKS5").Short('D').StringsVar(&cf.DynamicForwardedPorts)
	ssh.Flag("local", "Execute command on localhost after connecting to SSH node").Default("false").BoolVar(&cf.LocalExec)
	ssh.Flag("tty", "Allocate TTY").Short('t').BoolVar(&cf.Interactive)
	ssh.Flag("cluster", clusterHelp).Envar(clusterEnvVar).StringVar(&cf.SiteName)
	ssh.Flag("option", "OpenSSH options in the format used in the configuration file").Short('o').StringsVar(&cf.Options)

	// join
	join := app.Command("join", "Join the active SSH session")
	join.Flag("cluster", clusterHelp).Envar(clusterEnvVar).StringVar(&cf.SiteName)
	join.Arg("session-id", "ID of the session to join").Required().StringVar(&cf.SessionID)
	// play
	play := app.Command("play", "Replay the recorded SSH session")
	play.Flag("cluster", clusterHelp).Envar(clusterEnvVar).StringVar(&cf.SiteName)
	play.Arg("session-id", "ID of the session to play").Required().StringVar(&cf.SessionID)
	// scp
	scp := app.Command("scp", "Secure file copy")
	scp.Flag("cluster", clusterHelp).Envar(clusterEnvVar).StringVar(&cf.SiteName)
	scp.Arg("from, to", "Source and destination to copy").Required().StringsVar(&cf.CopySpec)
	scp.Flag("recursive", "Recursive copy of subdirectories").Short('r').BoolVar(&cf.RecursiveCopy)
	scp.Flag("port", "Port to connect to on the remote host").Short('P').Int32Var(&cf.NodePort)
	scp.Flag("quiet", "Quiet mode").Short('q').BoolVar(&cf.Quiet)
	// ls
	ls := app.Command("ls", "List remote SSH nodes")
	ls.Flag("cluster", clusterHelp).Envar(clusterEnvVar).StringVar(&cf.SiteName)
	ls.Arg("labels", "List of labels to filter node list").StringVar(&cf.UserHost)
	ls.Flag("verbose", clusterHelp).Short('v').BoolVar(&cf.Verbose)
	// clusters
	clusters := app.Command("clusters", "List available Teleport clusters")
	clusters.Flag("quiet", "Quiet mode").Short('q').BoolVar(&cf.Quiet)

	// login logs in with remote proxy and obtains a "session certificate" which gets
	// stored in ~/.tsh directory
	login := app.Command("login", "Log in to a cluster and retrieve the session certificate")
	login.Flag("out", "Identity output").Short('o').StringVar(&cf.IdentityFileOut)
	login.Flag("format", fmt.Sprintf("Identity format [%s] or %s (for OpenSSH compatibility)",
		client.DefaultIdentityFormat,
		client.IdentityFormatOpenSSH)).Default(string(client.DefaultIdentityFormat)).StringVar((*string)(&cf.IdentityFormat))
	login.Arg("cluster", clusterHelp).StringVar(&cf.SiteName)
	login.Alias(loginUsageFooter)

	// logout deletes obtained session certificates in ~/.tsh
	logout := app.Command("logout", "Delete a cluster certificate")

	// bench
	bench := app.Command("bench", "Run shell or execute a command on a remote SSH node").Hidden()
	bench.Flag("cluster", clusterHelp).Envar(clusterEnvVar).StringVar(&cf.SiteName)
	bench.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cf.UserHost)
	bench.Arg("command", "Command to execute on a remote host").Required().StringsVar(&cf.RemoteCommand)
	bench.Flag("port", "SSH port on a remote host").Short('p').Int32Var(&cf.NodePort)
	bench.Flag("threads", "Concurrent threads to run").Default("10").IntVar(&cf.BenchThreads)
	bench.Flag("duration", "Test duration").Default("1s").DurationVar(&cf.BenchDuration)
	bench.Flag("rate", "Requests per second rate").Default("10").IntVar(&cf.BenchRate)
	bench.Flag("interactive", "Create interactive SSH session").BoolVar(&cf.BenchInteractive)

	// show key
	show := app.Command("show", "Read an identity from file and print to stdout").Hidden()
	show.Arg("identity_file", "The file containing a public key or a certificate").Required().StringVar(&cf.IdentityFileIn)

	// The status command shows which proxy the user is logged into and metadata
	// about the certificate.
	status := app.Command("status", "Display the list of proxy servers and retrieved certificates")

	// On Windows, hide the "ssh", "join", "play", "scp", and "bench" commands
	// because they all use a terminal.
	if runtime.GOOS == teleport.WindowsOS {
		ssh.Hidden()
		join.Hidden()
		play.Hidden()
		scp.Hidden()
		bench.Hidden()
	}

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
			log.Debugf("signal: %v", sig)
			cancel()
		}
	}()
	cf.Context = ctx

	if cf.Gops {
		log.Debugf("Starting gops agent.")
		err = gops.Listen(&gops.Options{Addr: cf.GopsAddr})
		if err != nil {
			log.Warningf("Failed to start gops agent %v.", err)
		}
	}

	switch command {
	case ver.FullCommand():
		utils.PrintVersion()
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
	case login.FullCommand():
		onLogin(&cf)
	case logout.FullCommand():
		refuseArgs(logout.FullCommand(), args)
		onLogout(&cf)
	case show.FullCommand():
		onShow(&cf)
	case status.FullCommand():
		onStatus(&cf)
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
	var (
		err error
		tc  *client.TeleportClient
		key *client.Key
	)

	if cf.IdentityFileIn != "" {
		utils.FatalError(trace.BadParameter("-i flag cannot be used here"))
	}

	if cf.IdentityFormat != client.IdentityFormatOpenSSH && cf.IdentityFormat != client.IdentityFormatFile {
		utils.FatalError(trace.BadParameter("invalid identity format: %s", cf.IdentityFormat))
	}

	// Get the status of the active profile ~/.tsh/profile as well as the status
	// of any other proxies the user is logged into.
	profile, profiles, err := client.Status("", cf.Proxy)
	if err != nil {
		if !trace.IsNotFound(err) {
			utils.FatalError(err)
		}
	}

	// make the teleport client and retrieve the certificate from the proxy:
	tc, err = makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}

	// client is already logged in and profile is not expired
	if profile != nil && !profile.IsExpired(clockwork.NewRealClock()) {
		switch {
		// in case if nothing is specified, print current status
		case cf.Proxy == "" && cf.SiteName == "":
			printProfiles(profile, profiles)
			return
		// in case if parameters match, print current status
		case host(cf.Proxy) == host(profile.ProxyURL.Host) && cf.SiteName == profile.Cluster:
			printProfiles(profile, profiles)
			return
		// proxy is unspecified or the same as the currently provided proxy,
		// but cluster is specified, treat this as selecting a new cluster
		// for the same proxy
		case (cf.Proxy == "" || host(cf.Proxy) == host(profile.ProxyURL.Host)) && cf.SiteName != "":
			tc.SaveProfile("")
			if err := kubeclient.UpdateKubeconfig(tc); err != nil {
				utils.FatalError(err)
			}
			onStatus(cf)
			return
		// otherwise just passthrough to standard login
		default:
		}
	}

	if cf.Username == "" {
		cf.Username = tc.Username
	}

	// -i flag specified? save the retreived cert into an identity file
	makeIdentityFile := (cf.IdentityFileOut != "")
	activateKey := !makeIdentityFile

	if key, err = tc.Login(cf.Context, activateKey); err != nil {
		utils.FatalError(err)
	}

	if makeIdentityFile {
		if err := setupNoninteractiveClient(tc, key); err != nil {
			utils.FatalError(err)
		}
		authorities, err := tc.GetTrustedCA(cf.Context, key.ClusterName)
		if err != nil {
			utils.FatalError(err)
		}
		client.MakeIdentityFile(cf.IdentityFileOut, key, cf.IdentityFormat, authorities)
		fmt.Printf("\nThe certificate has been written to %s\n", cf.IdentityFileOut)
		return
	}

	// If the proxy is advertising that it supports Kubernetes, update kubeconfig.
	if tc.KubeProxyAddr != "" {
		if err := kubeclient.UpdateKubeconfig(tc); err != nil {
			utils.FatalError(err)
		}
	}

	// Regular login without -i flag.
	tc.SaveProfile("")

	// Connect to the Auth Server and fetch the known hosts for this cluster.
	err = tc.UpdateTrustedCA(cf.Context, key.ClusterName)
	if err != nil {
		utils.FatalError(err)
	}

	// Print status to show information of the logged in user. Update the
	// command line flag (used to print status) for the proxy to make sure any
	// advertised settings are picked up.
	webProxyHost, _ := tc.WebProxyHostPort()
	cf.Proxy = webProxyHost
	onStatus(cf)
}

// setupNoninteractiveClient sets up existing client to use
// non-interactive authentication methods
func setupNoninteractiveClient(tc *client.TeleportClient, key *client.Key) error {
	certUsername, err := key.CertUsername()
	if err != nil {
		return trace.Wrap(err)
	}
	tc.Username = certUsername

	// Extract and set the HostLogin to be the first principal. It doesn't
	// matter what the value is, but some valid principal has to be set
	// otherwise the certificate won't be validated.
	certPrincipals, err := key.CertPrincipals()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(certPrincipals) == 0 {
		return trace.BadParameter("no principals found")
	}
	tc.HostLogin = certPrincipals[0]

	identityAuth, err := authFromIdentity(key)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.TLS, err = key.ClientTLSConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AuthMethods = []ssh.AuthMethod{identityAuth}
	tc.Interactive = false
	tc.SkipLocalAuth = true
	return nil
}

// onLogout deletes a "session certificate" from ~/.tsh for a given proxy
func onLogout(cf *CLIConf) {
	// Extract all clusters the user is currently logged into.
	active, available, err := client.Status("", "")
	if err != nil {
		utils.FatalError(err)
		return
	}
	profiles := append(available, active)

	// Unlink the current profile.
	client.UnlinkCurrentProfile()

	// Extract the proxy name.
	proxyHost, _, err := net.SplitHostPort(cf.Proxy)
	if err != nil {
		proxyHost = cf.Proxy
	}

	switch {
	// Proxy and username for key to remove.
	case proxyHost != "" && cf.Username != "":
		tc, err := makeClient(cf, true)
		if err != nil {
			utils.FatalError(err)
			return
		}

		// Remove keys for this user from disk and running agent.
		err = tc.Logout()
		if err != nil {
			if trace.IsNotFound(err) {
				fmt.Printf("User %v already logged out from %v.\n", cf.Username, proxyHost)
				os.Exit(1)
			}
			utils.FatalError(err)
			return
		}

		// Get the address of the active Kubernetes proxy to find AuthInfos,
		// Clusters, and Contexts in kubeconfig.
		clusterName, _ := tc.KubeProxyHostPort()
		if tc.SiteName != "" {
			clusterName = fmt.Sprintf("%v.%v", tc.SiteName, clusterName)
		}

		// Remove Teleport related entries from kubeconfig.
		log.Debugf("Removing Teleport related entries for '%v' from kubeconfig.", clusterName)
		err = kubeclient.RemoveKubeconifg(tc, clusterName)
		if err != nil {
			utils.FatalError(err)
			return
		}

		fmt.Printf("Logged out %v from %v.\n", cf.Username, proxyHost)
	// Remove all keys.
	case proxyHost == "" && cf.Username == "":
		// The makeClient function requires a proxy. However this value is not used
		// because the user will be logged out from all proxies. Pass a dummy value
		// to allow creation of the TeleportClient.
		cf.Proxy = "dummy:1234"
		tc, err := makeClient(cf, true)
		if err != nil {
			utils.FatalError(err)
			return
		}

		// Remove Teleport related entries from kubeconfig for all clusters.
		for _, profile := range profiles {
			log.Debugf("Removing Teleport related entries for '%v' from kubeconfig.", profile.Cluster)
			err = kubeclient.RemoveKubeconifg(tc, profile.Cluster)
			if err != nil {
				utils.FatalError(err)
				return
			}
		}

		// Remove all keys from disk and the running agent.
		err = tc.LogoutAll()
		if err != nil {
			utils.FatalError(err)
			return
		}

		fmt.Printf("Logged out all users from all proxies.\n")
	default:
		fmt.Printf("Specify --proxy and --user to remove keys for specific user ")
		fmt.Printf("from a proxy or neither to log out all users from all proxies.\n")
	}
}

// onListNodes executes 'tsh ls' command.
func onListNodes(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	nodes, err := tc.ListNodes(context.TODO())
	if err != nil {
		utils.FatalError(err)
	}

	switch cf.Verbose {
	// In verbose mode, print everything on a single line and include the Node
	// ID (UUID). Useful for machines that need to parse the output of "tsh ls".
	case true:
		t := asciitable.MakeTable([]string{"Node Name", "Node ID", "Address", "Labels"})
		for _, n := range nodes {
			t.AddRow([]string{
				n.GetHostname(), n.GetName(), n.GetAddr(), n.LabelsString(),
			})
		}
		fmt.Println(t.AsBuffer().String())
	// In normal mode chunk the labels and print two per line and allow multiple
	// lines per node.
	case false:
		t := asciitable.MakeTable([]string{"Node Name", "Address", "Labels"})
		for _, n := range nodes {
			labelChunks := chunkLabels(n.GetAllLabels(), 2)
			for i, v := range labelChunks {
				var hostname string
				var addr string
				if i == 0 {
					hostname = n.GetHostname()
					addr = n.GetAddr()
				}
				t.AddRow([]string{hostname, addr, strings.Join(v, ", ")})
			}
		}
		fmt.Println(t.AsBuffer().String())
	}
}

// chunkLabels breaks labels into sized chunks. Used to improve readability
// of "tsh ls".
func chunkLabels(labels map[string]string, chunkSize int) [][]string {
	// First sort labels so they always occur in the same order.
	sorted := make([]string, 0, len(labels))
	for k, v := range labels {
		sorted = append(sorted, fmt.Sprintf("%v=%v", k, v))
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// Then chunk labels into sized chunks.
	var chunks [][]string
	for chunkSize < len(sorted) {
		sorted, chunks = sorted[chunkSize:], append(chunks, sorted[0:chunkSize:chunkSize])
	}
	chunks = append(chunks, sorted)

	return chunks
}

// onListSites executes 'tsh sites' command
func onListSites(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	proxyClient, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		utils.FatalError(err)
	}
	defer proxyClient.Close()

	sites, err := proxyClient.GetSites()
	if err != nil {
		utils.FatalError(err)
	}
	var t asciitable.Table
	if cf.Quiet {
		t = asciitable.MakeHeadlessTable(2)
	} else {
		t = asciitable.MakeTable([]string{"Cluster Name", "Status"})
	}
	if len(sites) == 0 {
		return
	}
	for _, site := range sites {
		t.AddRow([]string{site.Name, site.Status})
	}
	fmt.Println(t.AsBuffer().String())
}

// onSSH executes 'tsh ssh' command
func onSSH(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}

	tc.Stdin = os.Stdin
	if err = tc.SSH(cf.Context, cf.RemoteCommand, cf.LocalExec); err != nil {
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
	t := asciitable.MakeTable([]string{"Percentile", "Duration"})
	for _, quantile := range []float64{25, 50, 75, 90, 95, 99, 100} {
		t.AddRow([]string{fmt.Sprintf("%v", quantile),
			fmt.Sprintf("%v ms", result.Histogram.ValueAtQuantile(quantile)),
		})
	}
	io.Copy(os.Stdout, t.AsBuffer())
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
			fmt.Fprintln(os.Stderr, utils.UserMessageFromError(err))
			os.Exit(tc.ExitStatus)
		} else {
			utils.FatalError(err)
		}
	}
}

// makeClient takes the command-line configuration and constructs & returns
// a fully configured TeleportClient object
func makeClient(cf *CLIConf, useProfileLogin bool) (tc *client.TeleportClient, err error) {
	// Parse OpenSSH style options.
	options, err := parseOptions(cf.Options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

	dPorts, err := client.ParseDynamicPortForwardSpec(cf.DynamicForwardedPorts)
	if err != nil {
		return nil, err
	}

	// 1: start with the defaults
	c := client.MakeDefaultConfig()

	// Look if a user identity was given via -i flag
	if cf.IdentityFileIn != "" {
		// Ignore local authentication methods when identity file is provided
		c.SkipLocalAuth = true
		var (
			key          *client.Key
			identityAuth ssh.AuthMethod
			expiryDate   time.Time
			hostAuthFunc ssh.HostKeyCallback
		)
		// read the ID file and create an "auth method" from it:
		key, hostAuthFunc, err = loadIdentity(cf.IdentityFileIn)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if hostAuthFunc != nil {
			c.HostKeyCallback = hostAuthFunc
		} else {
			return nil, trace.BadParameter("missing trusted certificate authorities in the identity, upgrade to newer version of tctl, export identity and try again")
		}
		certUsername, err := key.CertUsername()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.Debugf("Extracted username %q from the identity file %v.", certUsername, cf.IdentityFileIn)
		c.Username = certUsername

		identityAuth, err = authFromIdentity(key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		c.AuthMethods = []ssh.AuthMethod{identityAuth}

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
		err = c.ParseProxyHost(cf.Proxy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if len(fPorts) > 0 {
		c.LocalForwardPorts = fPorts
	}
	if len(dPorts) > 0 {
		c.DynamicForwardedPorts = dPorts
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

	// If a TTY was requested, make sure to allocate it. Note this applies to
	// "exec" command because a shell always has a TTY allocated.
	if cf.Interactive || options.RequestTTY {
		c.Interactive = true
	}

	if !cf.NoCache {
		c.CachePolicy = &client.CachePolicy{}
	}

	// check version compatibility of the server and client
	c.CheckVersions = !cf.SkipVersionCheck

	// parse compatibility parameter
	certificateFormat, err := parseCertificateCompatibilityFlag(cf.Compatibility, cf.CertificateFormat)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.CertificateFormat = certificateFormat

	// copy the authentication connector over
	if cf.AuthConnector != "" {
		c.AuthConnector = cf.AuthConnector
	}

	// If agent forwarding was specified on the command line enable it.
	if cf.ForwardAgent || options.ForwardAgent {
		c.ForwardAgent = true
	}

	// If the caller does not want to check host keys, pass in a insecure host
	// key checker.
	if options.StrictHostKeyChecking == false {
		c.HostKeyCallback = client.InsecureSkipHostKeyChecking
	}

	return client.NewClient(c)
}

func parseCertificateCompatibilityFlag(compatibility string, certificateFormat string) (string, error) {
	switch {
	// if nothing is passed in, the role will decide
	case compatibility == "" && certificateFormat == "":
		return teleport.CertificateFormatUnspecified, nil
	// supporting the old --compat format for backward compatibility
	case compatibility != "" && certificateFormat == "":
		return utils.CheckCertificateFormatFlag(compatibility)
	// new documented flag --cert-format
	case compatibility == "" && certificateFormat != "":
		return utils.CheckCertificateFormatFlag(certificateFormat)
	// can not use both
	default:
		return "", trace.BadParameter("--compat or --cert-format must be specified")
	}
}

// refuseArgs helper makes sure that 'args' (list of CLI arguments)
// does not contain anything other than command
func refuseArgs(command string, args []string) {
	for _, arg := range args {
		if arg == command || strings.HasPrefix(arg, "-") {
			continue
		} else {
			utils.FatalError(trace.BadParameter("unexpected argument: %s", arg))
		}

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
func loadIdentity(idFn string) (*client.Key, ssh.HostKeyCallback, error) {
	log.Infof("Reading identity file: %v", idFn)

	f, err := os.Open(idFn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer f.Close()
	var (
		keyBuf  bytes.Buffer
		state   int // 0: not found, 1: found beginning, 2: found ending
		cert    []byte
		caCerts [][]byte
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
				caCerts = append(caCerts, []byte(line))
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
		log.Infof("Certificate not found in %s. Looking in %s.", idFn, certFn)
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
	var hostAuthFunc ssh.HostKeyCallback = nil
	// validate CA (cluster) cert
	if len(caCerts) > 0 {
		var trustedKeys []ssh.PublicKey
		for _, caCert := range caCerts {
			_, _, publicKey, _, _, err := ssh.ParseKnownHosts(caCert)
			if err != nil {
				return nil, nil, trace.BadParameter("CA cert parsing error: %v. cert line :%v",
					err.Error(), string(caCert))
			}
			trustedKeys = append(trustedKeys, publicKey)
		}

		// found CA cert in the indentity file? construct the host key checking function
		// and return it:
		hostAuthFunc = func(host string, a net.Addr, hostKey ssh.PublicKey) error {
			clusterCert, ok := hostKey.(*ssh.Certificate)
			if ok {
				hostKey = clusterCert.SignatureKey
			}
			for _, trustedKey := range trustedKeys {
				if sshutils.KeysEqual(trustedKey, hostKey) {
					return nil
				}
			}
			err = trace.AccessDenied("host %v is untrusted", host)
			log.Error(err)
			return err
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

// onShow reads an identity file (a public SSH key or a cert) and dumps it to stdout
func onShow(cf *CLIConf) {
	key, _, err := loadIdentity(cf.IdentityFileIn)

	// unmarshal certificate bytes into a ssh.PublicKey
	cert, _, _, _, err := ssh.ParseAuthorizedKey(key.Cert)
	if err != nil {
		utils.FatalError(err)
	}

	// unmarshal private key bytes into a *rsa.PrivateKey
	priv, err := ssh.ParseRawPrivateKey(key.Priv)
	if err != nil {
		utils.FatalError(err)
	}

	pub, err := ssh.ParsePublicKey(key.Pub)
	if err != nil {
		utils.FatalError(err)
	}

	fmt.Printf("Cert: %#v\nPriv: %#v\nPub: %#v\n",
		cert, priv, pub)

	fmt.Printf("Fingerprint: %s\n", ssh.FingerprintSHA256(pub))
}

// printStatus prints the status of the profile.
func printStatus(p *client.ProfileStatus, isActive bool) {
	var prefix string
	if isActive {
		prefix = "> "
	} else {
		prefix = "  "
	}
	duration := p.ValidUntil.Sub(time.Now())
	humanDuration := "EXPIRED"
	if duration.Nanoseconds() > 0 {
		humanDuration = fmt.Sprintf("valid for %v", duration.Round(time.Minute))
	}

	fmt.Printf("%vProfile URL:  %v\n", prefix, p.ProxyURL.String())
	fmt.Printf("  Logged in as: %v\n", p.Username)
	if p.Cluster != "" {
		fmt.Printf("  Cluster:      %v\n", p.Cluster)
	}
	fmt.Printf("  Roles:        %v*\n", strings.Join(p.Roles, ", "))
	fmt.Printf("  Logins:       %v\n", strings.Join(p.Logins, ", "))
	fmt.Printf("  Valid until:  %v [%v]\n", p.ValidUntil, humanDuration)
	fmt.Printf("  Extensions:   %v\n\n", strings.Join(p.Extensions, ", "))
}

// onStatus command shows which proxy the user is logged into and metadata
// about the certificate.
func onStatus(cf *CLIConf) {
	// Get the status of the active profile ~/.tsh/profile as well as the status
	// of any other proxies the user is logged into.
	profile, profiles, err := client.Status("", cf.Proxy)
	if err != nil {
		if trace.IsNotFound(err) {
			fmt.Printf("Not logged in.\n")
			return
		}
		utils.FatalError(err)
	}
	printProfiles(profile, profiles)
}

func printProfiles(profile *client.ProfileStatus, profiles []*client.ProfileStatus) {
	// Print the active profile.
	if profile != nil {
		printStatus(profile, true)
	}

	// Print all other profiles.
	for _, p := range profiles {
		printStatus(p, false)
	}

	// If we are printing profile, add a note that even though roles are listed
	// here, they are only available in Enterprise.
	if profile != nil || len(profiles) > 0 {
		fmt.Printf("\n* RBAC is only available in Teleport Enterprise\n")
		fmt.Printf("  https://gravitational.com/teleport/docs/enterprise\n")
	}
}

// host is a utility function that extracts
// host from the host:port pair, in case of any error
// returns the original value
func host(in string) string {
	out, err := utils.Host(in)
	if err != nil {
		return in
	}
	return out
}
