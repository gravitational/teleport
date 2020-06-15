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

package main

import (
	"context"
	"fmt"
	"io"
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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tsh/common"

	"github.com/gravitational/kingpin"
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
	// DesiredRoles indicates one or more roles which should be requested.
	DesiredRoles string
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
	// ProxyJump is an optional -J flag pointing to the list of jumphosts,
	// it is an equivalent of --proxy flag in tsh interpretation
	ProxyJump string
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
	IdentityFormat identityfile.Format

	// BindAddr is an address in the form of host:port to bind to
	// during `tsh login` command
	BindAddr string

	// AuthConnector is the name of the connector to use.
	AuthConnector string

	// SkipVersionCheck skips version checking for client and server
	SkipVersionCheck bool

	// Options is a list of OpenSSH options in the format used in the
	// configuration file.
	Options []string

	// Verbose is used to print extra output.
	Verbose bool

	// NoRemoteExec will not execute a remote command after connecting to a host,
	// will block instead. Useful when port forwarding. Equivalent of -N for OpenSSH.
	NoRemoteExec bool

	// Debug sends debug logs to stdout.
	Debug bool

	// Browser can be used to pass the name of a browser to override the system default
	// (not currently implemented), or set to 'none' to suppress browser opening entirely.
	Browser string

	// UseLocalSSHAgent set to false will prevent this client from attempting to
	// connect to the local ssh-agent (or similar) socket at $SSH_AUTH_SOCK.
	UseLocalSSHAgent bool

	// EnableEscapeSequences will scan stdin for SSH escape sequences during
	// command/shell execution. This also requires stdin to be an interactive
	// terminal.
	EnableEscapeSequences bool
}

func main() {
	cmdLineOrig := os.Args[1:]
	var cmdLine []string

	// lets see: if the executable name is 'ssh' or 'scp' we convert
	// that to "tsh ssh" or "tsh scp"
	switch path.Base(os.Args[0]) {
	case "ssh":
		cmdLine = append([]string{"ssh"}, cmdLineOrig...)
	case "scp":
		cmdLine = append([]string{"scp"}, cmdLineOrig...)
	default:
		cmdLine = cmdLineOrig
	}
	Run(cmdLine)
}

const (
	clusterEnvVar          = "TELEPORT_SITE"
	clusterHelp            = "Specify the cluster to connect"
	bindAddrEnvVar         = "TELEPORT_LOGIN_BIND_ADDR"
	authEnvVar             = "TELEPORT_AUTH"
	browserHelp            = "Set to 'none' to suppress browser opening on login"
	useLocalSSHAgentEnvVar = "TELEPORT_USE_LOCAL_SSH_AGENT"
)

// Run executes TSH client. same as main() but easier to test
func Run(args []string) {
	var cf CLIConf
	utils.InitLogger(utils.LoggingForCLI, logrus.WarnLevel)

	// configure CLI argument parser:
	app := utils.InitCLIParser("tsh", "TSH: Teleport Authentication Gateway Client").Interspersed(false)
	app.Flag("login", "Remote host login").Short('l').Envar("TELEPORT_LOGIN").StringVar(&cf.NodeLogin)
	localUser, _ := client.Username()
	app.Flag("proxy", "SSH proxy address").Envar("TELEPORT_PROXY").StringVar(&cf.Proxy)
	app.Flag("nocache", "do not cache cluster discovery locally").Hidden().BoolVar(&cf.NoCache)
	app.Flag("user", fmt.Sprintf("SSH proxy user [%s]", localUser)).Envar("TELEPORT_USER").StringVar(&cf.Username)
	app.Flag("option", "").Short('o').Hidden().AllowDuplicate().PreAction(func(ctx *kingpin.ParseContext) error {
		return trace.BadParameter("invalid flag, perhaps you want to use this flag as tsh ssh -o?")
	}).String()

	app.Flag("ttl", "Minutes to live for a SSH session").Int32Var(&cf.MinsToLive)
	app.Flag("identity", "Identity file").Short('i').StringVar(&cf.IdentityFileIn)
	app.Flag("compat", "OpenSSH compatibility flag").Hidden().StringVar(&cf.Compatibility)
	app.Flag("cert-format", "SSH certificate format").StringVar(&cf.CertificateFormat)
	app.Flag("insecure", "Do not verify server's certificate and host name. Use only in test environments").Default("false").BoolVar(&cf.InsecureSkipVerify)
	app.Flag("auth", "Specify the type of authentication connector to use.").Envar(authEnvVar).StringVar(&cf.AuthConnector)
	app.Flag("namespace", "Namespace of the cluster").Default(defaults.Namespace).Hidden().StringVar(&cf.Namespace)
	app.Flag("gops", "Start gops endpoint on a given address").Hidden().BoolVar(&cf.Gops)
	app.Flag("gops-addr", "Specify gops addr to listen on").Hidden().StringVar(&cf.GopsAddr)
	app.Flag("skip-version-check", "Skip version checking between server and client.").BoolVar(&cf.SkipVersionCheck)
	app.Flag("debug", "Verbose logging to stdout").Short('d').BoolVar(&cf.Debug)
	app.Flag("use-local-ssh-agent", "Load generated SSH certificates into the local ssh-agent (specified via $SSH_AUTH_SOCK). You can also set TELEPORT_USE_LOCAL_SSH_AGENT environment variable. Default is true.").
		Envar(useLocalSSHAgentEnvVar).
		Default("true").
		BoolVar(&cf.UseLocalSSHAgent)
	app.Flag("enable-escape-sequences", "Enable support for SSH escape sequences. Type '~?' during an SSH session to list supported sequences. Default is enabled.").
		Default("true").
		BoolVar(&cf.EnableEscapeSequences)
	app.HelpFlag.Short('h')
	ver := app.Command("version", "Print the version")
	// ssh
	ssh := app.Command("ssh", "Run shell or execute a command on a remote SSH node")
	ssh.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cf.UserHost)
	ssh.Arg("command", "Command to execute on a remote host").StringsVar(&cf.RemoteCommand)
	app.Flag("jumphost", "SSH jumphost").Short('J').StringVar(&cf.ProxyJump)
	ssh.Flag("port", "SSH port on a remote host").Short('p').Int32Var(&cf.NodePort)
	ssh.Flag("forward-agent", "Forward agent to target node").Short('A').BoolVar(&cf.ForwardAgent)
	ssh.Flag("forward", "Forward localhost connections to remote server").Short('L').StringsVar(&cf.LocalForwardPorts)
	ssh.Flag("dynamic-forward", "Forward localhost connections to remote server using SOCKS5").Short('D').StringsVar(&cf.DynamicForwardedPorts)
	ssh.Flag("local", "Execute command on localhost after connecting to SSH node").Default("false").BoolVar(&cf.LocalExec)
	ssh.Flag("tty", "Allocate TTY").Short('t').BoolVar(&cf.Interactive)
	ssh.Flag("cluster", clusterHelp).Envar(clusterEnvVar).StringVar(&cf.SiteName)
	ssh.Flag("option", "OpenSSH options in the format used in the configuration file").Short('o').AllowDuplicate().StringsVar(&cf.Options)
	ssh.Flag("no-remote-exec", "Don't execute remote command, useful for port forwarding").Short('N').BoolVar(&cf.NoRemoteExec)

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
	login.Flag("bind-addr", "Address in the form of host:port to bind to for login command webhook").Envar(bindAddrEnvVar).StringVar(&cf.BindAddr)
	login.Flag("out", "Identity output").Short('o').AllowDuplicate().StringVar(&cf.IdentityFileOut)
	login.Flag("format", fmt.Sprintf("Identity format: %s, %s (for OpenSSH compatibility) or %s (for kubeconfig)",
		identityfile.DefaultFormat,
		identityfile.FormatOpenSSH,
		identityfile.FormatKubernetes,
	)).Default(string(identityfile.DefaultFormat)).StringVar((*string)(&cf.IdentityFormat))
	login.Flag("request-roles", "Request one or more extra roles").StringVar(&cf.DesiredRoles)
	login.Arg("cluster", clusterHelp).StringVar(&cf.SiteName)
	login.Flag("browser", browserHelp).StringVar(&cf.Browser)
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

	// While in debug mode, send logs to stdout.
	if cf.Debug {
		utils.InitLogger(utils.LoggingForCLI, logrus.DebugLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		exitSignals := make(chan os.Signal, 1)
		signal.Notify(exitSignals, syscall.SIGTERM, syscall.SIGINT)

		sig := <-exitSignals
		log.Debugf("signal: %v", sig)
		cancel()
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
		onListClusters(&cf)
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

	// populate cluster name from environment variables
	// only if not set by argument (that does not support env variables)
	clusterName := os.Getenv(clusterEnvVar)
	if cf.SiteName == "" {
		cf.SiteName = clusterName
	}

	if cf.IdentityFileIn != "" {
		utils.FatalError(trace.BadParameter("-i flag cannot be used here"))
	}

	switch cf.IdentityFormat {
	case identityfile.FormatFile, identityfile.FormatOpenSSH, identityfile.FormatKubernetes:
	default:
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
		case cf.Proxy == "" && cf.SiteName == "" && cf.DesiredRoles == "" && cf.IdentityFileOut == "":
			printProfiles(cf.Debug, profile, profiles)
			return
		// in case if parameters match, print current status
		case host(cf.Proxy) == host(profile.ProxyURL.Host) && cf.SiteName == profile.Cluster && cf.DesiredRoles == "":
			printProfiles(cf.Debug, profile, profiles)
			return
		// proxy is unspecified or the same as the currently provided proxy,
		// but cluster is specified, treat this as selecting a new cluster
		// for the same proxy
		case (cf.Proxy == "" || host(cf.Proxy) == host(profile.ProxyURL.Host)) && cf.SiteName != "":
			// trigger reissue, preserving any active requests.
			err = tc.ReissueUserCerts(cf.Context, client.ReissueParams{
				AccessRequests: profile.ActiveRequests.AccessRequests,
				RouteToCluster: cf.SiteName,
			})
			if err != nil {
				utils.FatalError(err)
			}
			if err := tc.SaveProfile("", ""); err != nil {
				utils.FatalError(err)
			}
			if err := kubeconfig.UpdateWithClient("", tc); err != nil {
				utils.FatalError(err)
			}
			onStatus(cf)
			return
		// proxy is unspecified or the same as the currently provided proxy,
		// but desired roles are specified, treat this as a privilege escalation
		// request for the same login session.
		case (cf.Proxy == "" || host(cf.Proxy) == host(profile.ProxyURL.Host)) && cf.DesiredRoles != "" && cf.IdentityFileOut == "":
			executeAccessRequest(cf)
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

	key, err = tc.Login(cf.Context, activateKey)
	if err != nil {
		utils.FatalError(err)
	}

	if makeIdentityFile {
		if err := setupNoninteractiveClient(tc, key); err != nil {
			utils.FatalError(err)
		}
		// key.TrustedCA at this point only has the CA of the root cluster we
		// logged into. We need to fetch all the CAs for leaf clusters too, to
		// make them available in the identity file.
		authorities, err := tc.GetTrustedCA(cf.Context, key.ClusterName)
		if err != nil {
			utils.FatalError(err)
		}
		key.TrustedCA = auth.AuthoritiesToTrustedCerts(authorities)

		filesWritten, err := identityfile.Write(cf.IdentityFileOut, key, cf.IdentityFormat, tc.KubeClusterAddr())
		if err != nil {
			utils.FatalError(err)
		}
		fmt.Printf("\nThe certificate has been written to %s\n", strings.Join(filesWritten, ","))
		return
	}

	// If the proxy is advertising that it supports Kubernetes, update kubeconfig.
	if tc.KubeProxyAddr != "" {
		if err := kubeconfig.UpdateWithClient("", tc); err != nil {
			utils.FatalError(err)
		}
	}

	// Regular login without -i flag.
	if err := tc.SaveProfile(key.ProxyHost, ""); err != nil {
		utils.FatalError(err)
	}

	// Print status to show information of the logged in user. Update the
	// command line flag (used to print status) for the proxy to make sure any
	// advertised settings are picked up.
	webProxyHost, _ := tc.WebProxyHostPort()
	cf.Proxy = webProxyHost
	if cf.DesiredRoles != "" {
		fmt.Println("") // visually separate onRequestExecute output
		executeAccessRequest(cf)
	} else {
		onStatus(cf)
	}
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
	profiles := append([]*client.ProfileStatus{}, available...)
	if active != nil {
		profiles = append(profiles, active)
	}

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
		err = kubeconfig.Remove("", clusterName)
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
			err = kubeconfig.Remove("", profile.Cluster)
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

	// Get list of all nodes in backend and sort by "Node Name".
	var nodes []services.Server
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		nodes, err = tc.ListNodes(cf.Context)
		return err
	})
	if err != nil {
		utils.FatalError(err)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].GetHostname() < nodes[j].GetHostname()
	})

	showNodes(nodes, cf.Verbose)
}

func executeAccessRequest(cf *CLIConf) {
	if cf.DesiredRoles == "" {
		utils.FatalError(trace.BadParameter("one or more roles must be specified"))
	}
	roles := strings.Split(cf.DesiredRoles, ",")
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	if cf.Username == "" {
		cf.Username = tc.Username
	}
	req, err := services.NewAccessRequest(cf.Username, roles...)
	if err != nil {
		utils.FatalError(err)
	}
	fmt.Fprintf(os.Stderr, "Seeking request approval... (id: %s)\n", req.GetName())
	if err := getRequestApproval(cf, tc, req); err != nil {
		utils.FatalError(err)
	}
	fmt.Fprintf(os.Stderr, "Approval received, getting updated certificates...\n\n")
	if err := reissueWithRequests(cf, tc, req.GetName()); err != nil {
		utils.FatalError(err)
	}
	onStatus(cf)
}

func showNodes(nodes []services.Server, verbose bool) {
	switch verbose {
	// In verbose mode, print everything on a single line and include the Node
	// ID (UUID). Useful for machines that need to parse the output of "tsh ls".
	case true:
		t := asciitable.MakeTable([]string{"Node Name", "Node ID", "Address", "Labels"})
		for _, n := range nodes {
			addr := n.GetAddr()
			if n.GetUseTunnel() {
				addr = "⟵ Tunnel"
			}

			t.AddRow([]string{
				n.GetHostname(), n.GetName(), addr, n.LabelsString(),
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
					if n.GetUseTunnel() {
						addr = "⟵ Tunnel"
					}
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

// onListClusters executes 'tsh clusters' command
func onListClusters(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}

	var sites []services.Site
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		proxyClient, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return err
		}
		defer proxyClient.Close()

		sites, err = proxyClient.GetSites()
		return err
	})
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
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.SSH(cf.Context, cf.RemoteCommand, cf.LocalExec)
	})
	if err != nil {
		if strings.Contains(utils.UserMessageFromError(err), teleport.NodeIsAmbiguous) {
			allNodes, err := tc.ListAllNodes(cf.Context)
			if err != nil {
				utils.FatalError(err)
			}
			var nodes []services.Server
			for _, node := range allNodes {
				if node.GetHostname() == tc.Host {
					nodes = append(nodes, node)
				}
			}
			fmt.Fprintf(os.Stderr, "error: ambiguous host could match multiple nodes\n\n")
			showNodes(nodes, true)
			fmt.Fprintf(os.Stderr, "Hint: try addressing the node by unique id (ex: tsh ssh user@node-id)\n")
			fmt.Fprintf(os.Stderr, "Hint: use 'tsh ls -v' to list all nodes with their unique ids\n")
			fmt.Fprintf(os.Stderr, "\n")
			os.Exit(1)
		}
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
	if _, err := io.Copy(os.Stdout, t.AsBuffer()); err != nil {
		utils.FatalError(err)
	}
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
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.Join(context.TODO(), cf.Namespace, *sid, nil)
	})
	if err != nil {
		utils.FatalError(err)
	}
}

// onSCP executes 'tsh scp' command
func onSCP(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.SCP(context.TODO(), cf.CopySpec, int(cf.NodePort), cf.RecursiveCopy, cf.Quiet)
	})
	if err != nil {
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
func makeClient(cf *CLIConf, useProfileLogin bool) (*client.TeleportClient, error) {
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

	// ProxyJump is an alias of Proxy flag
	if cf.ProxyJump != "" {
		hosts, err := utils.ParseProxyJump(cf.ProxyJump)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		c.JumpHosts = hosts
	}

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
		key, hostAuthFunc, err = common.LoadIdentity(cf.IdentityFileIn)
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

		if len(key.TLSCert) > 0 {
			c.TLS, err = key.ClientTLSConfig()
			if err != nil {
				return nil, trace.Wrap(err)
			}
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
	// if proxy is set, and proxy is not equal to profile's
	// loaded addresses, override the values
	if cf.Proxy != "" && c.WebProxyAddr == "" {
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
	if !options.StrictHostKeyChecking {
		c.HostKeyCallback = client.InsecureSkipHostKeyChecking
	}
	c.BindAddr = cf.BindAddr

	// Don't execute remote command, used when port forwarding.
	c.NoRemoteExec = cf.NoRemoteExec

	// Allow the default browser used to open tsh login links to be overridden
	// (not currently implemented) or set to 'none' to suppress browser opening entirely.
	c.Browser = cf.Browser

	// Do not write SSH certs into the local ssh-agent if user requested it.
	//
	// This is specifically for gpg-agent, which doesn't support SSH
	// certificates (https://dev.gnupg.org/T1756)
	c.UseLocalSSHAgent = cf.UseLocalSSHAgent

	c.EnableEscapeSequences = cf.EnableEscapeSequences

	tc, err := client.NewClient(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// If identity file was provided, we skip loading the local profile info
	// (above). This profile info provides the proxy-advertised listening
	// addresses.
	// To compensate, when using an identity file, explicitly fetch these
	// addresses from the proxy (this is what Ping does).
	if cf.IdentityFileIn != "" {
		log.Debug("Pinging the proxy to fetch listening addresses for non-web ports.")
		if _, err := tc.Ping(cf.Context); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return tc, nil
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
	key, _, err := common.LoadIdentity(cf.IdentityFileIn)
	if err != nil {
		utils.FatalError(err)
	}

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
func printStatus(debug bool, p *client.ProfileStatus, isActive bool) {
	var count int
	var prefix string
	if isActive {
		prefix = "> "
	} else {
		prefix = "  "
	}
	duration := time.Until(p.ValidUntil)
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
	if debug {
		for k, v := range p.Traits {
			if count == 0 {
				fmt.Printf("  Traits:       %v: %v\n", k, v)
			} else {
				fmt.Printf("                %v: %v\n", k, v)
			}
			count = count + 1
		}
	}
	fmt.Printf("  Logins:       %v\n", strings.Join(p.Logins, ", "))
	fmt.Printf("  Valid until:  %v [%v]\n", p.ValidUntil, humanDuration)
	fmt.Printf("  Extensions:   %v\n", strings.Join(p.Extensions, ", "))

	fmt.Printf("\n")
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
	printProfiles(cf.Debug, profile, profiles)
}

func printProfiles(debug bool, profile *client.ProfileStatus, profiles []*client.ProfileStatus) {
	// Print the active profile.
	if profile != nil {
		printStatus(debug, profile, true)
	}

	// Print all other profiles.
	for _, p := range profiles {
		printStatus(debug, p, false)
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

// getRequestApproval registers an access request with the auth server and waits for it to be approved.
func getRequestApproval(cf *CLIConf, tc *client.TeleportClient, req services.AccessRequest) error {
	// set up request watcher before submitting the request to the admin server
	// in order to avoid potential race.
	filter := services.AccessRequestFilter{
		User: tc.Username,
	}
	watcher, err := tc.NewWatcher(cf.Context, services.Watch{
		Name: "await-request-approval",
		Kinds: []services.WatchKind{
			services.WatchKind{
				Kind:   services.KindAccessRequest,
				Filter: filter.IntoMap(),
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	if err := tc.CreateAccessRequest(cf.Context, req); err != nil {
		utils.FatalError(err)
	}
Loop:
	for {
		select {
		case event := <-watcher.Events():
			switch event.Type {
			case backend.OpInit:
				log.Infof("Access-request watcher initialized...")
				continue Loop
			case backend.OpPut:
				r, ok := event.Resource.(*services.AccessRequestV3)
				if !ok {
					return trace.BadParameter("unexpected resource type %T", event.Resource)
				}
				if r.GetName() != req.GetName() || r.GetState().IsPending() {
					log.Infof("Skipping put event id=%s,state=%s.", r.GetName(), r.GetState())
					continue Loop
				}
				if !r.GetState().IsApproved() {
					return trace.Errorf("request %s has been set to %s", r.GetName(), r.GetState().String())
				}
				return nil
			case backend.OpDelete:
				if event.Resource.GetName() != req.GetName() {
					log.Infof("Skipping delete event id=%s", event.Resource.GetName())
					continue Loop
				}
				return trace.Errorf("request %s has expired or been deleted...", event.Resource.GetName())
			default:
				log.Warnf("Skipping unknown event type %s", event.Type)
			}
		case <-watcher.Done():
			utils.FatalError(watcher.Error())
		}
	}
}

// reissueWithRequests handles a certificate reissue, applying new requests by ID,
// and saving the updated profile.
func reissueWithRequests(cf *CLIConf, tc *client.TeleportClient, reqIDs ...string) error {
	profile, _, err := client.Status("", cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	params := client.ReissueParams{
		AccessRequests: reqIDs,
		RouteToCluster: cf.SiteName,
	}
	// if the certificate already had active requests, add them to our inputs parameters.
	if len(profile.ActiveRequests.AccessRequests) > 0 {
		params.AccessRequests = append(params.AccessRequests, profile.ActiveRequests.AccessRequests...)
	}
	if params.RouteToCluster == "" {
		params.RouteToCluster = profile.Cluster
	}
	if err := tc.ReissueUserCerts(cf.Context, params); err != nil {
		return trace.Wrap(err)
	}
	if err := tc.SaveProfile("", ""); err != nil {
		return trace.Wrap(err)
	}
	if err := kubeconfig.UpdateWithClient("", tc); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
