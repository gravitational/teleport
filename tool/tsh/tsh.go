/*
Copyright 2016-2021 Gravitational, Inc.

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
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/benchmark"
	"github.com/gravitational/teleport/lib/client"
	dbprofile "github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/prompt"

	"github.com/ghodss/yaml"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTSH,
})

const (
	// mfaModeAuto automatically chooses the best MFA device(s), without any
	// restrictions.
	// Allows both Webauthn and OTP.
	mfaModeAuto = "auto"
	// mfaModeCrossPlatform utilizes only cross-platform devices, such as
	// pluggable hardware keys.
	// Implies Webauthn.
	mfaModeCrossPlatform = "cross-platform"
	// mfaModePlatform utilizes only platform devices, such as Touch ID.
	// Implies Webauthn.
	mfaModePlatform = "platform"
	// mfaModeOTP utilizes only OTP devices.
	mfaModeOTP = "otp"

	hostnameOrIDPredicateTemplate = `resource.spec.hostname == "%[1]s" || name == "%[1]s"`
)

// CLIConf stores command line arguments and flags:
type CLIConf struct {
	// UserHost contains "[login]@hostname" argument to SSH command
	UserHost string
	// Commands to execute on a remote host
	RemoteCommand []string
	// DesiredRoles indicates one or more roles which should be requested.
	DesiredRoles string
	// RequestReason indicates the reason for an access request.
	RequestReason string
	// SuggestedReviewers is a list of suggested request reviewers.
	SuggestedReviewers string
	// NoWait can be used with an access request to exit without waiting for a request resolution.
	NoWait bool
	// RequestedResourceIDs is a list of resources to request access to.
	RequestedResourceIDs []string
	// RequestID is an access request ID
	RequestID string
	// ReviewReason indicates the reason for an access review.
	ReviewReason string
	// ReviewableRequests indicates that only requests which can be reviewed should
	// be listed.
	ReviewableRequests bool
	// SuggestedRequests indicates that only requests which suggest the current user
	// as a reviewer should be listed.
	SuggestedRequests bool
	// MyRequests indicates that only requests created by the current user
	// should be listed.
	MyRequests bool
	// Approve/Deny indicates the desired review kind.
	Approve, Deny bool
	// ResourceKind is the resource kind to search for
	ResourceKind string
	// Username is the Teleport user's username (to login into proxies)
	Username string
	// ExplicitUsername is true if Username was initially set by the end-user
	// (for example, using command-line flags).
	ExplicitUsername bool
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
	// SessionID identifies the session tsh is operating on.
	// For `tsh join`, it is the ID of the session to join.
	// For `tsh play`, it is either the ID of the session to play,
	// or the path to a local session file which has already been
	// downloaded.
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
	// KubernetesCluster specifies the kubernetes cluster to login to.
	KubernetesCluster string
	// DaemonAddr is the daemon listening address.
	DaemonAddr string
	// DatabaseService specifies the database proxy server to log into.
	DatabaseService string
	// DatabaseUser specifies database user to embed in the certificate.
	DatabaseUser string
	// DatabaseName specifies database name to embed in the certificate.
	DatabaseName string
	// AppName specifies proxied application name.
	AppName string
	// Interactive, when set to true, launches remote command with the terminal attached
	Interactive bool
	// Quiet mode, -q command (disables progress printing)
	Quiet bool
	// Namespace is used to select cluster namespace
	Namespace string
	// NoCache is used to turn off client cache for nodes discovery
	NoCache bool
	// BenchDuration is a duration for the benchmark
	BenchDuration time.Duration
	// BenchRate is a requests per second rate to maintain
	BenchRate int
	// BenchInteractive indicates that we should create interactive session
	BenchInteractive bool
	// BenchExport exports the latency profile
	BenchExport bool
	// BenchExportPath saves the latency profile in provided path
	BenchExportPath string
	// BenchTicks ticks per half distance
	BenchTicks int32
	// BenchValueScale value at which to scale the values recorded
	BenchValueScale float64
	// Context is a context to control execution
	Context context.Context
	// IdentityFileIn is an argument to -i flag (path to the private key+cert file)
	IdentityFileIn string
	// Compatibility flags, --compat, specifies OpenSSH compatibility flags.
	Compatibility string
	// CertificateFormat defines the format of the user SSH certificate.
	CertificateFormat string
	// IdentityFileOut is an argument to -out flag
	IdentityFileOut string
	// IdentityFormat (used for --format flag for 'tsh login') defines which
	// format to use with --out to store a freshly retrieved certificate
	IdentityFormat identityfile.Format
	// IdentityOverwrite when true will overwrite any existing identity file at
	// IdentityFileOut. When false, user will be prompted before overwriting
	// any files.
	IdentityOverwrite bool

	// BindAddr is an address in the form of host:port to bind to
	// during `tsh login` command
	BindAddr string

	// AuthConnector is the name of the connector to use.
	AuthConnector string

	// MFAMode is the preferred mode for MFA/Passwordless assertions.
	MFAMode string

	// SkipVersionCheck skips version checking for client and server
	SkipVersionCheck bool

	// Options is a list of OpenSSH options in the format used in the
	// configuration file.
	Options []string

	// Verbose is used to print extra output.
	Verbose bool

	// Format is used to change the format of output
	Format string

	// SearchKeywords is a list of search keywords to match against resource field values.
	SearchKeywords string

	// PredicateExpression defines boolean conditions that will be matched against the resource.
	PredicateExpression string

	// NoRemoteExec will not execute a remote command after connecting to a host,
	// will block instead. Useful when port forwarding. Equivalent of -N for OpenSSH.
	NoRemoteExec bool

	// X11ForwardingUntrusted will set up untrusted X11 forwarding for the session ('ssh -X')
	X11ForwardingUntrusted bool

	// X11Forwarding will set up trusted X11 forwarding for the session ('ssh -Y')
	X11ForwardingTrusted bool

	// X11ForwardingTimeout can optionally set to set a timeout for untrusted X11 forwarding.
	X11ForwardingTimeout time.Duration

	// Debug sends debug logs to stdout.
	Debug bool

	// Browser can be used to pass the name of a browser to override the system default
	// (not currently implemented), or set to 'none' to suppress browser opening entirely.
	Browser string

	// UseLocalSSHAgent set to false will prevent this client from attempting to
	// connect to the local ssh-agent (or similar) socket at $SSH_AUTH_SOCK.
	//
	// Deprecated in favor of `AddKeysToAgent`.
	UseLocalSSHAgent bool

	// AddKeysToAgent specifies the behavior of how certs are handled.
	AddKeysToAgent string

	// EnableEscapeSequences will scan stdin for SSH escape sequences during
	// command/shell execution. This also requires stdin to be an interactive
	// terminal.
	EnableEscapeSequences bool

	// PreserveAttrs preserves access/modification times from the original file.
	PreserveAttrs bool

	// executablePath is the absolute path to the current executable.
	executablePath string

	// unsetEnvironment unsets Teleport related environment variables.
	unsetEnvironment bool

	// overrideStdout allows to switch standard output source for resource command. Used in tests.
	overrideStdout io.Writer
	// overrideStderr allows to switch standard error source for resource command. Used in tests.
	overrideStderr io.Writer

	// mockSSOLogin used in tests to override sso login handler in teleport client.
	mockSSOLogin client.SSOLoginFunc

	// HomePath is where tsh stores profiles
	HomePath string

	// GlobalTshConfigPath is a path to global TSH config. Can be overridden with TELEPORT_GLOBAL_TSH_CONFIG.
	GlobalTshConfigPath string

	// LocalProxyPort is a port used by local proxy listener.
	LocalProxyPort string
	// LocalProxyCertFile is the client certificate used by local proxy.
	LocalProxyCertFile string
	// LocalProxyKeyFile is the client key used by local proxy.
	LocalProxyKeyFile string
	// LocalProxyTunnel specifies whether local proxy will open auth'd tunnel.
	LocalProxyTunnel bool

	// AWSRole is Amazon Role ARN or role name that will be used for AWS CLI access.
	AWSRole string
	// AWSCommandArgs contains arguments that will be forwarded to AWS CLI binary.
	AWSCommandArgs []string
	// AWSEndpointURLMode is an AWS proxy mode that serves an AWS endpoint URL
	// proxy instead of an HTTPS proxy.
	AWSEndpointURLMode bool

	// Reason is the reason for starting an ssh or kube session.
	Reason string

	// Invited is a list of invited users to an ssh or kube session.
	Invited []string

	// JoinMode is the participant mode someone is joining a session as.
	JoinMode string

	// displayParticipantRequirements is set if verbose participant requirement information should be printed for moderated sessions.
	displayParticipantRequirements bool

	// TshConfig is the loaded tsh configuration file ~/.tsh/config/config.yaml.
	TshConfig TshConfig

	// ListAll specifies if an ls command should return results from all clusters and proxies.
	ListAll bool
	// SampleTraces indicates whether traces should be sampled.
	SampleTraces bool

	// TracingProvider is the provider to use to create tracers, from which spans can be created.
	TracingProvider oteltrace.TracerProvider

	// disableAccessRequest disables automatic resource access requests.
	disableAccessRequest bool
}

// Stdout returns the stdout writer.
func (c *CLIConf) Stdout() io.Writer {
	if c.overrideStdout != nil {
		return c.overrideStdout
	}
	return os.Stdout
}

// Stderr returns the stderr writer.
func (c *CLIConf) Stderr() io.Writer {
	if c.overrideStderr != nil {
		return c.overrideStderr
	}
	return os.Stderr
}

type exitCodeError struct {
	code int
}

func (e *exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

func main() {
	cmdLineOrig := os.Args[1:]
	var cmdLine []string

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

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

	err := Run(ctx, cmdLine)
	prompt.NotifyExit() // Allow prompt to restore terminal state on exit.
	if err != nil {
		var exitError *exitCodeError
		if errors.As(err, &exitError) {
			os.Exit(exitError.code)
		}
		utils.FatalError(err)
	}
}

const (
	authEnvVar        = "TELEPORT_AUTH"
	clusterEnvVar     = "TELEPORT_CLUSTER"
	kubeClusterEnvVar = "TELEPORT_KUBE_CLUSTER"
	loginEnvVar       = "TELEPORT_LOGIN"
	bindAddrEnvVar    = "TELEPORT_LOGIN_BIND_ADDR"
	proxyEnvVar       = "TELEPORT_PROXY"
	// TELEPORT_SITE uses the older deprecated "site" terminology to refer to a
	// cluster. All new code should use TELEPORT_CLUSTER instead.
	siteEnvVar             = "TELEPORT_SITE"
	userEnvVar             = "TELEPORT_USER"
	addKeysToAgentEnvVar   = "TELEPORT_ADD_KEYS_TO_AGENT"
	useLocalSSHAgentEnvVar = "TELEPORT_USE_LOCAL_SSH_AGENT"
	globalTshConfigEnvVar  = "TELEPORT_GLOBAL_TSH_CONFIG"
	mfaModeEnvVar          = "TELEPORT_MFA_MODE"

	clusterHelp = "Specify the Teleport cluster to connect"
	browserHelp = "Set to 'none' to suppress browser opening on login"
	searchHelp  = `List of comma separated search keywords or phrases enclosed in quotations (e.g. --search=foo,bar,"some phrase")`
	queryHelp   = `Query by predicate language enclosed in single quotes. Supports ==, !=, &&, and || (e.g. --query='labels["key1"] == "value1" && labels["key2"] != "value2"')`
	labelHelp   = "List of comma separated labels to filter by labels (e.g. key1=value1,key2=value2)"
	// proxyDefaultResolutionTimeout is how long to wait for an unknown proxy
	// port to be resolved.
	//
	// Originally based on the RFC-8305 "Maximum Connection Attempt Delay"
	// recommended default value of 2s. In the RFC this value is for the
	// establishment of a TCP connection, rather than the full HTTP round-
	// trip that we measure against, so some tweaking may be needed.
	proxyDefaultResolutionTimeout = 2 * time.Second
)

// env vars that tsh status will check to provide hints about active env vars to a user.
var tshStatusEnvVars = [...]string{proxyEnvVar, clusterEnvVar, siteEnvVar, kubeClusterEnvVar, teleport.EnvKubeConfig}

// cliOption is used in tests to inject/override configuration within Run
type cliOption func(*CLIConf) error

// defaultFormats is the default set of formats to use for commands that have the --format flag.
var defaultFormats = []string{teleport.Text, teleport.JSON, teleport.YAML}

// Run executes TSH client. same as main() but easier to test
func Run(ctx context.Context, args []string, opts ...cliOption) error {
	cf := CLIConf{
		Context:         ctx,
		TracingProvider: tracing.NoopProvider(),
	}
	utils.InitLogger(utils.LoggingForCLI, logrus.WarnLevel)

	moduleCfg := modules.GetModules()
	var cpuProfile, memProfile string

	// configure CLI argument parser:
	app := utils.InitCLIParser("tsh", "Teleport Command Line Client").Interspersed(false)
	app.Flag("login", "Remote host login").Short('l').Envar(loginEnvVar).StringVar(&cf.NodeLogin)
	localUser, _ := client.Username()
	app.Flag("proxy", "SSH proxy address").Envar(proxyEnvVar).StringVar(&cf.Proxy)
	app.Flag("nocache", "do not cache cluster discovery locally").Hidden().BoolVar(&cf.NoCache)
	app.Flag("user", fmt.Sprintf("SSH proxy user [%s]", localUser)).Envar(userEnvVar).StringVar(&cf.Username)
	app.Flag("mem-profile", "Write memory profile to file").Hidden().StringVar(&memProfile)
	app.Flag("cpu-profile", "Write CPU profile to file").Hidden().StringVar(&cpuProfile)
	app.Flag("option", "").Short('o').Hidden().AllowDuplicate().PreAction(func(ctx *kingpin.ParseContext) error {
		return trace.BadParameter("invalid flag, perhaps you want to use this flag as tsh ssh -o?")
	}).String()

	app.Flag("ttl", "Minutes to live for a SSH session").Int32Var(&cf.MinsToLive)
	app.Flag("identity", "Identity file").Short('i').StringVar(&cf.IdentityFileIn)
	app.Flag("compat", "OpenSSH compatibility flag").Hidden().StringVar(&cf.Compatibility)
	app.Flag("cert-format", "SSH certificate format").StringVar(&cf.CertificateFormat)
	app.Flag("trace", "Capture and export distributed traces").Hidden().BoolVar(&cf.SampleTraces)

	if !moduleCfg.IsBoringBinary() {
		// The user is *never* allowed to do this in FIPS mode.
		app.Flag("insecure", "Do not verify server's certificate and host name. Use only in test environments").
			Default("false").
			BoolVar(&cf.InsecureSkipVerify)
	}

	app.Flag("auth", "Specify the name of authentication connector to use.").Envar(authEnvVar).StringVar(&cf.AuthConnector)
	app.Flag("namespace", "Namespace of the cluster").Default(apidefaults.Namespace).Hidden().StringVar(&cf.Namespace)
	app.Flag("skip-version-check", "Skip version checking between server and client.").BoolVar(&cf.SkipVersionCheck)
	app.Flag("debug", "Verbose logging to stdout").Short('d').BoolVar(&cf.Debug)
	app.Flag("add-keys-to-agent", fmt.Sprintf("Controls how keys are handled. Valid values are %v.", client.AllAddKeysOptions)).Short('k').Envar(addKeysToAgentEnvVar).Default(client.AddKeysToAgentAuto).StringVar(&cf.AddKeysToAgent)
	app.Flag("use-local-ssh-agent", "Deprecated in favor of the add-keys-to-agent flag.").
		Hidden().
		Envar(useLocalSSHAgentEnvVar).
		Default("true").
		BoolVar(&cf.UseLocalSSHAgent)
	app.Flag("enable-escape-sequences", "Enable support for SSH escape sequences. Type '~?' during an SSH session to list supported sequences. Default is enabled.").
		Default("true").
		BoolVar(&cf.EnableEscapeSequences)
	app.Flag("bind-addr", "Override host:port used when opening a browser for cluster logins").Envar(bindAddrEnvVar).StringVar(&cf.BindAddr)
	modes := []string{mfaModeAuto, mfaModeCrossPlatform, mfaModePlatform, mfaModeOTP}
	app.Flag("mfa-mode", fmt.Sprintf("Preferred mode for MFA and Passwordless assertions (%v)", strings.Join(modes, ", "))).
		Default(mfaModeAuto).
		Envar(mfaModeEnvVar).
		EnumVar(&cf.MFAMode, modes...)
	app.HelpFlag.Short('h')

	ver := app.Command("version", "Print the version of your tsh binary")
	ver.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
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
	ssh.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	ssh.Flag("option", "OpenSSH options in the format used in the configuration file").Short('o').AllowDuplicate().StringsVar(&cf.Options)
	ssh.Flag("no-remote-exec", "Don't execute remote command, useful for port forwarding").Short('N').BoolVar(&cf.NoRemoteExec)
	ssh.Flag("x11-untrusted", "Requests untrusted (secure) X11 forwarding for this session").Short('X').BoolVar(&cf.X11ForwardingUntrusted)
	ssh.Flag("x11-trusted", "Requests trusted (insecure) X11 forwarding for this session. This can make your local machine vulnerable to attacks, use with caution").Short('Y').BoolVar(&cf.X11ForwardingTrusted)
	ssh.Flag("x11-untrusted-timeout", "Sets a timeout for untrusted X11 forwarding, after which the client will reject any forwarding requests from the server").Default("10m").DurationVar((&cf.X11ForwardingTimeout))
	ssh.Flag("participant-req", "Displays a verbose list of required participants in a moderated session.").BoolVar(&cf.displayParticipantRequirements)
	ssh.Flag("request-reason", "Reason for requesting access").StringVar(&cf.RequestReason)
	ssh.Flag("disable-access-request", "Disable automatic resource access requests").BoolVar(&cf.disableAccessRequest)

	// Daemon service for teleterm client
	daemon := app.Command("daemon", "Daemon is the tsh daemon service").Hidden()
	daemonStart := daemon.Command("start", "Starts tsh daemon service").Hidden()
	daemonStart.Flag("addr", "Addr is the daemon listening address.").StringVar(&cf.DaemonAddr)

	// AWS.
	aws := app.Command("aws", "Access AWS API.")
	aws.Arg("command", "AWS command and subcommands arguments that are going to be forwarded to AWS CLI.").StringsVar(&cf.AWSCommandArgs)
	aws.Flag("app", "Optional Name of the AWS application to use if logged into multiple.").StringVar(&cf.AppName)
	aws.Flag("endpoint-url", "Run local proxy to serve as an AWS endpoint URL. If not specified, local proxy serves as an HTTPS proxy.").
		Short('e').Hidden().BoolVar(&cf.AWSEndpointURLMode)

	// Applications.
	apps := app.Command("apps", "View and control proxied applications.").Alias("app")
	lsApps := apps.Command("ls", "List available applications.")
	lsApps.Flag("verbose", "Show extra application fields.").Short('v').BoolVar(&cf.Verbose)
	lsApps.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	lsApps.Flag("search", searchHelp).StringVar(&cf.SearchKeywords)
	lsApps.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	lsApps.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
	lsApps.Arg("labels", labelHelp).StringVar(&cf.UserHost)
	lsApps.Flag("all", "List apps from all clusters and proxies.").Short('R').BoolVar(&cf.ListAll)
	appLogin := apps.Command("login", "Retrieve short-lived certificate for an app.")
	appLogin.Arg("app", "App name to retrieve credentials for. Can be obtained from `tsh apps ls` output.").Required().StringVar(&cf.AppName)
	appLogin.Flag("aws-role", "(For AWS CLI access only) Amazon IAM role ARN or role name.").StringVar(&cf.AWSRole)
	appLogout := apps.Command("logout", "Remove app certificate.")
	appLogout.Arg("app", "App to remove credentials for.").StringVar(&cf.AppName)
	appConfig := apps.Command("config", "Print app connection information.")
	appConfig.Arg("app", "App to print information for. Required when logged into multiple apps.").StringVar(&cf.AppName)
	appConfig.Flag("format", fmt.Sprintf("Optional print format, one of: %q to print app address, %q to print CA cert path, %q to print cert path, %q print key path, %q to print example curl command, %q or %q to print everything as JSON or YAML.",
		appFormatURI, appFormatCA, appFormatCert, appFormatKey, appFormatCURL, appFormatJSON, appFormatYAML),
	).Short('f').StringVar(&cf.Format)

	// Local TLS proxy.
	proxy := app.Command("proxy", "Run local TLS proxy allowing connecting to Teleport in single-port mode")
	proxySSH := proxy.Command("ssh", "Start local TLS proxy for ssh connections when using Teleport in single-port mode")
	proxySSH.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cf.UserHost)
	proxySSH.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	proxyDB := proxy.Command("db", "Start local TLS proxy for database connections when using Teleport in single-port mode")
	proxyDB.Arg("db", "The name of the database to start local proxy for").Required().StringVar(&cf.DatabaseService)
	proxyDB.Flag("port", "Specifies the source port used by proxy db listener").Short('p').StringVar(&cf.LocalProxyPort)
	proxyDB.Flag("cert-file", "Certificate file for proxy client TLS configuration").StringVar(&cf.LocalProxyCertFile)
	proxyDB.Flag("key-file", "Key file for proxy client TLS configuration").StringVar(&cf.LocalProxyKeyFile)
	proxyDB.Flag("tunnel", "Open authenticated tunnel using database's client certificate so clients don't need to authenticate").BoolVar(&cf.LocalProxyTunnel)
	proxyApp := proxy.Command("app", "Start local TLS proxy for app connection when using Teleport in single-port mode")
	proxyApp.Arg("app", "The name of the application to start local proxy for").Required().StringVar(&cf.AppName)
	proxyApp.Flag("port", "Specifies the source port used by by the proxy app listener").Short('p').StringVar(&cf.LocalProxyPort)
	proxyAWS := proxy.Command("aws", "Start local proxy for AWS access.")
	proxyAWS.Flag("app", "Optional Name of the AWS application to use if logged into multiple.").StringVar(&cf.AppName)
	proxyAWS.Flag("port", "Specifies the source port used by the proxy listener.").Short('p').StringVar(&cf.LocalProxyPort)
	proxyAWS.Flag("endpoint-url", "Run local proxy to serve as an AWS endpoint URL. If not specified, local proxy serves as an HTTPS proxy.").Short('e').BoolVar(&cf.AWSEndpointURLMode)
	proxyAWS.Flag("format", envVarFormatFlagDescription()).Short('f').Default(envVarDefaultFormat()).EnumVar(&cf.Format, envVarFormats...)

	// Databases.
	db := app.Command("db", "View and control proxied databases.")
	db.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	dbList := db.Command("ls", "List all available databases.")
	dbList.Flag("verbose", "Show extra database fields.").Short('v').BoolVar(&cf.Verbose)
	dbList.Flag("search", searchHelp).StringVar(&cf.SearchKeywords)
	dbList.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	dbList.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
	dbList.Flag("all", "List databases from all clusters and proxies.").Short('R').BoolVar(&cf.ListAll)
	dbList.Arg("labels", labelHelp).StringVar(&cf.UserHost)
	dbLogin := db.Command("login", "Retrieve credentials for a database.")
	dbLogin.Arg("db", "Database to retrieve credentials for. Can be obtained from 'tsh db ls' output.").Required().StringVar(&cf.DatabaseService)
	dbLogin.Flag("db-user", "Optional database user to configure as default.").StringVar(&cf.DatabaseUser)
	dbLogin.Flag("db-name", "Optional database name to configure as default.").StringVar(&cf.DatabaseName)
	dbLogout := db.Command("logout", "Remove database credentials.")
	dbLogout.Arg("db", "Database to remove credentials for.").StringVar(&cf.DatabaseService)
	dbEnv := db.Command("env", "Print environment variables for the configured database.")
	dbEnv.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
	dbEnv.Arg("db", "Print environment for the specified database").StringVar(&cf.DatabaseService)
	// --db flag is deprecated in favor of positional argument for consistency with other commands.
	dbEnv.Flag("db", "Print environment for the specified database.").Hidden().StringVar(&cf.DatabaseService)
	dbConfig := db.Command("config", "Print database connection information. Useful when configuring GUI clients.")
	dbConfig.Arg("db", "Print information for the specified database.").StringVar(&cf.DatabaseService)
	// --db flag is deprecated in favor of positional argument for consistency with other commands.
	dbConfig.Flag("db", "Print information for the specified database.").Hidden().StringVar(&cf.DatabaseService)
	dbConfig.Flag("format", fmt.Sprintf("Print format: %q to print in table format (default), %q to print connect command, %q or %q to print in JSON or YAML.",
		dbFormatText, dbFormatCommand, dbFormatJSON, dbFormatYAML)).Short('f').EnumVar(&cf.Format, dbFormatText, dbFormatCommand, dbFormatJSON, dbFormatYAML)
	dbConnect := db.Command("connect", "Connect to a database.")
	dbConnect.Arg("db", "Database service name to connect to.").StringVar(&cf.DatabaseService)
	dbConnect.Flag("db-user", "Optional database user to log in as.").StringVar(&cf.DatabaseUser)
	dbConnect.Flag("db-name", "Optional database name to log in to.").StringVar(&cf.DatabaseName)

	// join
	join := app.Command("join", "Join the active SSH session")
	join.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	join.Flag("mode", "Mode of joining the session, valid modes are observer and moderator").Short('m').Default("peer").StringVar(&cf.JoinMode)
	join.Flag("reason", "The purpose of the session.").StringVar(&cf.Reason)
	join.Flag("invite", "A comma separated list of people to mark as invited for the session.").StringsVar(&cf.Invited)
	join.Arg("session-id", "ID of the session to join").Required().StringVar(&cf.SessionID)
	// play
	play := app.Command("play", "Replay the recorded SSH session")
	play.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	play.Flag("format", formatFlagDescription(
		teleport.PTY, teleport.JSON, teleport.YAML,
	)).Short('f').Default(teleport.PTY).EnumVar(&cf.Format, teleport.PTY, teleport.JSON, teleport.YAML)
	play.Arg("session-id", "ID of the session to play").Required().StringVar(&cf.SessionID)

	// scp
	scp := app.Command("scp", "Secure file copy")
	scp.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	scp.Arg("from, to", "Source and destination to copy").Required().StringsVar(&cf.CopySpec)
	scp.Flag("recursive", "Recursive copy of subdirectories").Short('r').BoolVar(&cf.RecursiveCopy)
	scp.Flag("port", "Port to connect to on the remote host").Short('P').Int32Var(&cf.NodePort)
	scp.Flag("preserve", "Preserves access and modification times from the original file").Short('p').BoolVar(&cf.PreserveAttrs)
	scp.Flag("quiet", "Quiet mode").Short('q').BoolVar(&cf.Quiet)
	// ls
	ls := app.Command("ls", "List remote SSH nodes")
	ls.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	ls.Flag("verbose", "One-line output (for text format), including node UUIDs").Short('v').BoolVar(&cf.Verbose)
	ls.Flag("format", formatFlagDescription(
		teleport.Text, teleport.JSON, teleport.YAML, teleport.Names,
	)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, teleport.Text, teleport.JSON, teleport.YAML, teleport.Names)
	ls.Arg("labels", labelHelp).StringVar(&cf.UserHost)
	ls.Flag("search", searchHelp).StringVar(&cf.SearchKeywords)
	ls.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	ls.Flag("all", "List nodes from all clusters and proxies.").Short('R').BoolVar(&cf.ListAll)
	// clusters
	clusters := app.Command("clusters", "List available Teleport clusters")
	clusters.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
	clusters.Flag("quiet", "Quiet mode").Short('q').BoolVar(&cf.Quiet)

	// login logs in with remote proxy and obtains a "session certificate" which gets
	// stored in ~/.tsh directory
	login := app.Command("login", "Log in to a cluster and retrieve the session certificate")
	login.Flag("out", "Identity output").Short('o').AllowDuplicate().StringVar(&cf.IdentityFileOut)
	login.Flag("format", fmt.Sprintf("Identity format: %s, %s (for OpenSSH compatibility) or %s (for kubeconfig)",
		identityfile.DefaultFormat,
		identityfile.FormatOpenSSH,
		identityfile.FormatKubernetes,
	)).Default(string(identityfile.DefaultFormat)).Short('f').StringVar((*string)(&cf.IdentityFormat))
	login.Flag("overwrite", "Whether to overwrite the existing identity file.").BoolVar(&cf.IdentityOverwrite)
	login.Flag("request-roles", "Request one or more extra roles").StringVar(&cf.DesiredRoles)
	login.Flag("request-reason", "Reason for requesting additional roles").StringVar(&cf.RequestReason)
	login.Flag("request-reviewers", "Suggested reviewers for role request").StringVar(&cf.SuggestedReviewers)
	login.Flag("request-nowait", "Finish without waiting for request resolution").BoolVar(&cf.NoWait)
	login.Flag("request-id", "Login with the roles requested in the given request").StringVar(&cf.RequestID)
	login.Arg("cluster", clusterHelp).StringVar(&cf.SiteName)
	login.Flag("browser", browserHelp).StringVar(&cf.Browser)
	login.Flag("kube-cluster", "Name of the Kubernetes cluster to login to").StringVar(&cf.KubernetesCluster)
	login.Flag("verbose", "Show extra status information").Short('v').BoolVar(&cf.Verbose)
	login.Alias(loginUsageFooter)

	// logout deletes obtained session certificates in ~/.tsh
	logout := app.Command("logout", "Delete a cluster certificate")

	// bench
	bench := app.Command("bench", "Run shell or execute a command on a remote SSH node").Hidden()
	bench.Flag("cluster", clusterHelp).Short('c').StringVar(&cf.SiteName)
	bench.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cf.UserHost)
	bench.Arg("command", "Command to execute on a remote host").Required().StringsVar(&cf.RemoteCommand)
	bench.Flag("port", "SSH port on a remote host").Short('p').Int32Var(&cf.NodePort)
	bench.Flag("duration", "Test duration").Default("1s").DurationVar(&cf.BenchDuration)
	bench.Flag("rate", "Requests per second rate").Default("10").IntVar(&cf.BenchRate)
	bench.Flag("interactive", "Create interactive SSH session").BoolVar(&cf.BenchInteractive)
	bench.Flag("export", "Export the latency profile").BoolVar(&cf.BenchExport)
	bench.Flag("path", "Directory to save the latency profile to, default path is the current directory").Default(".").StringVar(&cf.BenchExportPath)
	bench.Flag("ticks", "Ticks per half distance").Default("100").Int32Var(&cf.BenchTicks)
	bench.Flag("scale", "Value scale in which to scale the recorded values").Default("1.0").Float64Var(&cf.BenchValueScale)

	// show key
	show := app.Command("show", "Read an identity from file and print to stdout").Hidden()
	show.Arg("identity_file", "The file containing a public key or a certificate").Required().StringVar(&cf.IdentityFileIn)

	// The status command shows which proxy the user is logged into and metadata
	// about the certificate.
	status := app.Command("status", "Display the list of proxy servers and retrieved certificates")
	status.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
	status.Flag("verbose", "Show extra status information after successful login").Short('v').BoolVar(&cf.Verbose)

	// The environment command prints out environment variables for the configured
	// proxy and cluster. Can be used to create sessions "sticky" to a terminal
	// even if the user runs "tsh login" again in another window.
	environment := app.Command("env", "Print commands to set Teleport session environment variables")
	environment.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
	environment.Flag("unset", "Print commands to clear Teleport session environment variables").BoolVar(&cf.unsetEnvironment)

	req := app.Command("request", "Manage access requests").Alias("requests")

	reqList := req.Command("ls", "List access requests").Alias("list")
	reqList.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
	reqList.Flag("reviewable", "Only show requests reviewable by current user").BoolVar(&cf.ReviewableRequests)
	reqList.Flag("suggested", "Only show requests that suggest current user as reviewer").BoolVar(&cf.SuggestedRequests)
	reqList.Flag("my-requests", "Only show requests created by current user").BoolVar(&cf.MyRequests)

	reqShow := req.Command("show", "Show request details").Alias("details")
	reqShow.Flag("format", formatFlagDescription(defaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaultFormats...)
	reqShow.Arg("request-id", "ID of the target request").Required().StringVar(&cf.RequestID)

	reqCreate := req.Command("new", "Create a new access request").Alias("create")
	reqCreate.Flag("roles", "Roles to be requested").StringVar(&cf.DesiredRoles)
	reqCreate.Flag("reason", "Reason for requesting").StringVar(&cf.RequestReason)
	reqCreate.Flag("reviewers", "Suggested reviewers").StringVar(&cf.SuggestedReviewers)
	reqCreate.Flag("nowait", "Finish without waiting for request resolution").BoolVar(&cf.NoWait)
	reqCreate.Flag("resource", "Resource ID to be requested").StringsVar(&cf.RequestedResourceIDs)

	reqReview := req.Command("review", "Review an access request")
	reqReview.Arg("request-id", "ID of target request").Required().StringVar(&cf.RequestID)
	reqReview.Flag("approve", "Review proposes approval").BoolVar(&cf.Approve)
	reqReview.Flag("deny", "Review proposes denial").BoolVar(&cf.Deny)
	reqReview.Flag("reason", "Review reason message").StringVar(&cf.ReviewReason)

	reqSearch := req.Command("search", "Search for resources to request access to")
	reqSearch.Flag("kind",
		fmt.Sprintf("Resource kind to search for (%s)",
			strings.Join(types.RequestableResourceKinds, ", ")),
	).Required().EnumVar(&cf.ResourceKind, types.RequestableResourceKinds...)
	reqSearch.Flag("search", searchHelp).StringVar(&cf.SearchKeywords)
	reqSearch.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	reqSearch.Flag("labels", labelHelp).StringVar(&cf.UserHost)

	// Kubernetes subcommands.
	kube := newKubeCommand(app)
	// MFA subcommands.
	mfa := newMFACommand(app)

	config := app.Command("config", "Print OpenSSH configuration details")

	f2 := app.Command("fido2", "FIDO2 commands").Hidden()
	f2Diag := f2.Command("diag", "Run FIDO2 diagnostics").Hidden()

	// touchid subcommands.
	tid := newTouchIDCommand(app)

	if runtime.GOOS == constants.WindowsOS {
		bench.Hidden()
	}

	// parse CLI commands+flags:
	utils.UpdateAppUsageTemplate(app, args)
	command, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}
	// Did we initially get the Username from flags/env?
	cf.ExplicitUsername = cf.Username != ""

	// apply any options after parsing of arguments to ensure
	// that defaults don't overwrite options.
	for _, opt := range opts {
		if err := opt(&cf); err != nil {
			return trace.Wrap(err)
		}
	}

	// While in debug mode, send logs to stdout.
	if cf.Debug {
		utils.InitLogger(utils.LoggingForCLI, logrus.DebugLevel)
	}

	// Connect to the span exporter and initialize the trace provider only if
	// the --trace flag was set.
	if cf.SampleTraces {
		provider, err := newTraceProvider(&cf, command, []string{login.FullCommand()})
		if err != nil {
			log.WithError(err).Debug("failed to set up span forwarding.")
		} else {
			// only update the provider if we successfully set it up
			cf.TracingProvider = provider

			// ensure that the provider is shutdown on exit to flush any spans
			// that haven't been forwarded yet.
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(cf.Context, 1*time.Second)
				defer cancel()
				err := provider.Shutdown(shutdownCtx)
				if err != nil && !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
					log.WithError(err).Debugf("failed to shutdown trace provider")
				}
			}()
		}
	}

	// start the span for the command and update the config context so that all spans created
	// in the future will be rooted at this span.
	ctx, span := cf.TracingProvider.Tracer(teleport.ComponentTSH).Start(cf.Context, command)
	cf.Context = ctx
	defer span.End()

	cf.executablePath, err = os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.ValidateAgentKeyOption(cf.AddKeysToAgent); err != nil {
		return trace.Wrap(err)
	}

	setEnvFlags(&cf, os.Getenv)

	confOptions, err := loadAllConfigs(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	cf.TshConfig = *confOptions

	if cpuProfile != "" {
		log.Debugf("writing CPU profile to %v", cpuProfile)
		f, err := os.Create(cpuProfile)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return trace.Wrap(err)
		}
		defer pprof.StopCPUProfile()
	}

	if memProfile != "" {
		log.Debugf("writing memory profile to %v", memProfile)
		defer func() {
			f, err := os.Create(memProfile)
			if err != nil {
				log.Errorf("could not open memory profile: %v", err)
				return
			}
			defer f.Close()
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Errorf("could not write memory profile: %v", err)
				return
			}
		}()
	}

	switch command {
	case ver.FullCommand():
		err = onVersion(&cf)
	case ssh.FullCommand():
		err = onSSH(&cf)
	case bench.FullCommand():
		err = onBenchmark(&cf)
	case join.FullCommand():
		err = onJoin(&cf)
	case scp.FullCommand():
		err = onSCP(&cf)
	case play.FullCommand():
		err = onPlay(&cf)
	case ls.FullCommand():
		err = onListNodes(&cf)
	case clusters.FullCommand():
		err = onListClusters(&cf)
	case login.FullCommand():
		err = onLogin(&cf)
	case logout.FullCommand():
		if err := refuseArgs(logout.FullCommand(), args); err != nil {
			return trace.Wrap(err)
		}
		err = onLogout(&cf)
	case show.FullCommand():
		err = onShow(&cf)
	case status.FullCommand():
		err = onStatus(&cf)
	case lsApps.FullCommand():
		err = onApps(&cf)
	case appLogin.FullCommand():
		err = onAppLogin(&cf)
	case appLogout.FullCommand():
		err = onAppLogout(&cf)
	case appConfig.FullCommand():
		err = onAppConfig(&cf)
	case kube.credentials.FullCommand():
		err = kube.credentials.run(&cf)
	case kube.ls.FullCommand():
		err = kube.ls.run(&cf)
	case kube.login.FullCommand():
		err = kube.login.run(&cf)
	case kube.sessions.FullCommand():
		err = kube.sessions.run(&cf)
	case kube.exec.FullCommand():
		err = kube.exec.run(&cf)
	case kube.join.FullCommand():
		err = kube.join.run(&cf)

	case proxySSH.FullCommand():
		err = onProxyCommandSSH(&cf)
	case proxyDB.FullCommand():
		err = onProxyCommandDB(&cf)
	case proxyApp.FullCommand():
		err = onProxyCommandApp(&cf)
	case proxyAWS.FullCommand():
		err = onProxyCommandAWS(&cf)

	case dbList.FullCommand():
		err = onListDatabases(&cf)
	case dbLogin.FullCommand():
		err = onDatabaseLogin(&cf)
	case dbLogout.FullCommand():
		err = onDatabaseLogout(&cf)
	case dbEnv.FullCommand():
		err = onDatabaseEnv(&cf)
	case dbConfig.FullCommand():
		err = onDatabaseConfig(&cf)
	case dbConnect.FullCommand():
		err = onDatabaseConnect(&cf)
	case environment.FullCommand():
		err = onEnvironment(&cf)
	case mfa.ls.FullCommand():
		err = mfa.ls.run(&cf)
	case mfa.add.FullCommand():
		err = mfa.add.run(&cf)
	case mfa.rm.FullCommand():
		err = mfa.rm.run(&cf)
	case reqList.FullCommand():
		err = onRequestList(&cf)
	case reqShow.FullCommand():
		err = onRequestShow(&cf)
	case reqCreate.FullCommand():
		err = onRequestCreate(&cf)
	case reqReview.FullCommand():
		err = onRequestReview(&cf)
	case reqSearch.FullCommand():
		err = onRequestSearch(&cf)
	case config.FullCommand():
		err = onConfig(&cf)
	case aws.FullCommand():
		err = onAWS(&cf)
	case daemonStart.FullCommand():
		err = onDaemonStart(&cf)
	case f2Diag.FullCommand():
		err = onFIDO2Diag(&cf)
	case tid.diag.FullCommand():
		err = tid.diag.run(&cf)
	default:
		// Handle commands that might not be available.
		switch {
		case tid.ls != nil && command == tid.ls.FullCommand():
			err = tid.ls.run(&cf)
		case tid.rm != nil && command == tid.rm.FullCommand():
			err = tid.rm.run(&cf)
		default:
			// This should only happen when there's a missing switch case above.
			err = trace.BadParameter("command %q not configured", command)
		}
	}

	if trace.IsNotImplemented(err) {
		return handleUnimplementedError(ctx, err, cf)
	}

	return trace.Wrap(err)
}

// newTraceProvider initializes the tracing provider and exports all recorded spans
// to the auth server to be forwarded to the telemetry backend. The whitelist allows
// certain commands to have exporting spans be a no-op. Since the provider requires
// connecting to the auth server, this means a user may have to log in first before
// the provider can be created. By whitelisting the login command we can avoid having
// users logging in twice at the expense of not exporting spans for the login command.
func newTraceProvider(cf *CLIConf, command string, whitelist []string) (*tracing.Provider, error) {
	// don't record any spans for commands that have been whitelisted
	for _, c := range whitelist {
		if strings.EqualFold(command, c) {
			return tracing.NoopProvider(), nil
		}
	}

	tc, err := makeClient(cf, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var traceclt *apitracing.Client
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		traceclt, err = tc.NewTracingClient(cf.Context)
		return err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	provider, err := tracing.NewTraceProvider(cf.Context,
		tracing.Config{
			Service: teleport.ComponentTSH,
			Client:  traceclt,
			// We are using 1 here to record all spans as a result of this tsh command. Teleport
			// will respect the recording flag of remote spans even if the spans it generates
			// wouldn't otherwise be recorded due to its configured sampling rate.
			SamplingRate: 1.0,
		})
	if err != nil {
		return nil, trace.NewAggregate(err, traceclt.Close())
	}

	return provider, nil
}

// onVersion prints version info.
func onVersion(cf *CLIConf) error {
	proxyVersion, err := fetchProxyVersion(cf)
	if err != nil {
		fmt.Fprintf(cf.Stderr(), "Failed to fetch proxy version: %s\n", err)
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		utils.PrintVersion()
		if proxyVersion != "" {
			fmt.Printf("Proxy version: %s\n", proxyVersion)
		}
	case teleport.JSON, teleport.YAML:
		out, err := serializeVersion(format, proxyVersion)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}

	return nil
}

// fetchProxyVersion returns the current version of the Teleport Proxy.
func fetchProxyVersion(cf *CLIConf) (string, error) {
	profile, _, err := client.Status(cf.HomePath, cf.Proxy)
	if err != nil {
		if trace.IsNotFound(err) {
			return "", nil
		}
		return "", trace.Wrap(err)
	}

	if profile == nil {
		return "", nil
	}

	tc, err := makeClient(cf, false)
	if err != nil {
		return "", trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(cf.Context, time.Second*5)
	defer cancel()
	pingRes, err := tc.Ping(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return pingRes.ServerVersion, nil
}

func serializeVersion(format string, proxyVersion string) (string, error) {
	versionInfo := struct {
		Version      string `json:"version"`
		Gitref       string `json:"gitref"`
		Runtime      string `json:"runtime"`
		ProxyVersion string `json:"proxyVersion,omitempty"`
	}{
		teleport.Version,
		teleport.Gitref,
		runtime.Version(),
		proxyVersion,
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(versionInfo, "", "  ")
	} else {
		out, err = yaml.Marshal(versionInfo)
	}
	return string(out), trace.Wrap(err)
}

// onPlay is used to interact with recorded sessions.
// It has several modes:
//
// 1. If --format is "pty" (the default), then the recorded
//    session is played back in the user's terminal.
// 2. Otherwise, `tsh play` is used to export a session from the
//    binary protobuf format into YAML or JSON.
//
// Each of these modes has two subcases:
// a) --session-id ends with ".tar" - tsh operates on a local file
//    containing a previously downloaded session
// b) --session-id is the ID of a session - tsh operates on the session
//    recording by connecting to the Teleport cluster
func onPlay(cf *CLIConf) error {
	if format := strings.ToLower(cf.Format); format == teleport.PTY {
		return playSession(cf)
	}
	return exportSession(cf)
}

func exportSession(cf *CLIConf) error {
	format := strings.ToLower(cf.Format)
	isLocalFile := path.Ext(cf.SessionID) == ".tar"
	if isLocalFile {
		return trace.Wrap(exportFile(cf.Context, cf.SessionID, format))
	}

	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}
	events, err := tc.GetSessionEvents(cf.Context, cf.Namespace, cf.SessionID)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, event := range events {
		// when playing from a file, id is not included, this
		// makes the outputs otherwise identical
		delete(event, "id")
		var e []byte
		var err error
		if format == teleport.JSON {
			e, err = utils.FastMarshal(event)
		} else {
			e, err = yaml.Marshal(event)
		}
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(string(e))
	}
	return nil
}

func playSession(cf *CLIConf) error {
	isLocalFile := path.Ext(cf.SessionID) == ".tar"
	if isLocalFile {
		sid := sessionIDFromPath(cf.SessionID)
		tarFile, err := os.Open(cf.SessionID)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		defer tarFile.Close()
		if err := client.PlayFile(cf.Context, tarFile, sid); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := tc.Play(cf.Context, cf.Namespace, cf.SessionID); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func sessionIDFromPath(path string) string {
	fileName := filepath.Base(path)
	return strings.TrimSuffix(fileName, ".tar")
}

// exportFile converts the binary protobuf events from the file
// identified by path to text (JSON/YAML) and writes the converted
// events to standard out.
func exportFile(ctx context.Context, path string, format string) error {
	f, err := os.Open(path)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	err = events.Export(ctx, f, os.Stdout, format)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// onLogin logs in with remote proxy and gets signed certificates
func onLogin(cf *CLIConf) error {
	autoRequest := true
	// special case: --request-roles=no disables auto-request behavior.
	if cf.DesiredRoles == "no" {
		autoRequest = false
		cf.DesiredRoles = ""
	}

	if cf.IdentityFileIn != "" {
		return trace.BadParameter("-i flag cannot be used here")
	}

	switch cf.IdentityFormat {
	case identityfile.FormatFile, identityfile.FormatOpenSSH, identityfile.FormatKubernetes:
	default:
		return trace.BadParameter("invalid identity format: %s", cf.IdentityFormat)
	}

	// Get the status of the active profile as well as the status
	// of any other proxies the user is logged into.
	profile, profiles, err := client.Status(cf.HomePath, cf.Proxy)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	// make the teleport client and retrieve the certificate from the proxy:
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.HomePath = cf.HomePath

	// client is already logged in and profile is not expired
	if profile != nil && !profile.IsExpired(clockwork.NewRealClock()) {
		switch {
		// in case if nothing is specified, re-fetch kube clusters and print
		// current status
		case cf.Proxy == "" && cf.SiteName == "" && cf.DesiredRoles == "" && cf.RequestID == "" && cf.IdentityFileOut == "":
			_, err := tc.PingAndShowMOTD(cf.Context)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := updateKubeConfig(cf, tc, ""); err != nil {
				return trace.Wrap(err)
			}
			env := getTshEnv()
			active, others := makeAllProfileInfo(profile, profiles, env)
			printProfiles(cf.Debug, active, others, env, cf.Verbose)

			return nil
		// in case if parameters match, re-fetch kube clusters and print
		// current status
		case host(cf.Proxy) == host(profile.ProxyURL.Host) && cf.SiteName == profile.Cluster && cf.DesiredRoles == "" && cf.RequestID == "":
			_, err := tc.PingAndShowMOTD(cf.Context)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := updateKubeConfig(cf, tc, ""); err != nil {
				return trace.Wrap(err)
			}
			env := getTshEnv()
			active, others := makeAllProfileInfo(profile, profiles, env)
			printProfiles(cf.Debug, active, others, env, cf.Verbose)

			return nil
		// proxy is unspecified or the same as the currently provided proxy,
		// but cluster is specified, treat this as selecting a new cluster
		// for the same proxy
		case (cf.Proxy == "" || host(cf.Proxy) == host(profile.ProxyURL.Host)) && cf.SiteName != "":
			_, err := tc.PingAndShowMOTD(cf.Context)
			if err != nil {
				return trace.Wrap(err)
			}
			// trigger reissue, preserving any active requests.
			err = tc.ReissueUserCerts(cf.Context, client.CertCacheKeep, client.ReissueParams{
				AccessRequests: profile.ActiveRequests.AccessRequests,
				RouteToCluster: cf.SiteName,
			})
			if err != nil {
				return trace.Wrap(err)
			}
			if err := tc.SaveProfile(cf.HomePath, true); err != nil {
				return trace.Wrap(err)
			}
			if err := updateKubeConfig(cf, tc, ""); err != nil {
				return trace.Wrap(err)
			}

			return trace.Wrap(onStatus(cf))
		// proxy is unspecified or the same as the currently provided proxy,
		// but desired roles or request ID is specified, treat this as a
		// privilege escalation request for the same login session.
		case (cf.Proxy == "" || host(cf.Proxy) == host(profile.ProxyURL.Host)) && (cf.DesiredRoles != "" || cf.RequestID != "") && cf.IdentityFileOut == "":
			_, err := tc.PingAndShowMOTD(cf.Context)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := executeAccessRequest(cf, tc); err != nil {
				return trace.Wrap(err)
			}
			if err := updateKubeConfig(cf, tc, ""); err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(onStatus(cf))
		// otherwise just passthrough to standard login
		default:
		}
	}

	if cf.Username == "" {
		cf.Username = tc.Username
	}

	// -i flag specified? save the retrieved cert into an identity file
	makeIdentityFile := (cf.IdentityFileOut != "")

	// stdin hijack is OK for login, since it tsh doesn't read input after the
	// login ceremony is complete.
	// Only allow the option during the login ceremony.
	tc.AllowStdinHijack = true

	key, err := tc.Login(cf.Context)
	if err != nil {
		if !cf.ExplicitUsername && auth.IsInvalidLocalCredentialError(err) {
			fmt.Fprintf(os.Stderr, "\nhint: set the --user flag to log in as a specific user, or leave it empty to use the system user (%v)\n\n", tc.Username)
		}
		return trace.Wrap(err)
	}
	tc.AllowStdinHijack = false

	// the login operation may update the username and should be considered the more
	// "authoritative" source.
	cf.Username = tc.Username

	// TODO(fspmarshall): Refactor access request & cert reissue logic to allow
	// access requests to be applied to identity files.

	if makeIdentityFile {
		if err := setupNoninteractiveClient(tc, key); err != nil {
			return trace.Wrap(err)
		}
		// key.TrustedCA at this point only has the CA of the root cluster we
		// logged into. We need to fetch all the CAs for leaf clusters too, to
		// make them available in the identity file.
		rootClusterName := key.TrustedCA[0].ClusterName
		authorities, err := tc.GetTrustedCA(cf.Context, rootClusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		key.TrustedCA = auth.AuthoritiesToTrustedCerts(authorities)

		filesWritten, err := identityfile.Write(identityfile.WriteConfig{
			OutputPath:           cf.IdentityFileOut,
			Key:                  key,
			Format:               cf.IdentityFormat,
			KubeProxyAddr:        tc.KubeClusterAddr(),
			OverwriteDestination: cf.IdentityOverwrite,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("\nThe certificate has been written to %s\n", strings.Join(filesWritten, ","))
		return nil
	}

	if err := tc.ActivateKey(cf.Context, key); err != nil {
		return trace.Wrap(err)
	}

	// If the proxy is advertising that it supports Kubernetes, update kubeconfig.
	if tc.KubeProxyAddr != "" {
		if err := updateKubeConfig(cf, tc, ""); err != nil {
			return trace.Wrap(err)
		}
	}

	// Regular login without -i flag.
	if err := tc.SaveProfile(cf.HomePath, true); err != nil {
		return trace.Wrap(err)
	}

	if autoRequest && cf.DesiredRoles == "" && cf.RequestID == "" {
		var requireReason, auto bool
		var prompt string
		roleNames, err := key.CertRoles()
		if err != nil {
			logoutErr := tc.Logout()
			return trace.NewAggregate(err, logoutErr)
		}
		// load all roles from root cluster and collect relevant options.
		// the normal one-off TeleportClient methods don't re-use the auth server
		// connection, so we use WithRootClusterClient to speed things up.
		err = tc.WithRootClusterClient(cf.Context, func(clt auth.ClientI) error {
			for _, roleName := range roleNames {
				role, err := clt.GetRole(cf.Context, roleName)
				if err != nil {
					return trace.Wrap(err)
				}
				requireReason = requireReason || role.GetOptions().RequestAccess.RequireReason()
				auto = auto || role.GetOptions().RequestAccess.ShouldAutoRequest()
				if prompt == "" {
					prompt = role.GetOptions().RequestPrompt
				}
			}
			return nil
		})
		if err != nil {
			logoutErr := tc.Logout()
			return trace.NewAggregate(err, logoutErr)
		}
		if requireReason && cf.RequestReason == "" {
			msg := "--request-reason must be specified"
			if prompt != "" {
				msg = msg + ", prompt=" + prompt
			}
			err := trace.BadParameter(msg)
			logoutErr := tc.Logout()
			return trace.NewAggregate(err, logoutErr)
		}
		if auto {
			cf.DesiredRoles = "*"
		}
	}

	if cf.DesiredRoles != "" || cf.RequestID != "" {
		fmt.Println("") // visually separate access request output
		if err := executeAccessRequest(cf, tc); err != nil {
			logoutErr := tc.Logout()
			return trace.NewAggregate(err, logoutErr)
		}
	}

	// Update the command line flag for the proxy to make sure any advertised
	// settings are picked up.
	webProxyHost, _ := tc.WebProxyHostPort()
	cf.Proxy = webProxyHost

	// Print status to show information of the logged in user.
	return trace.Wrap(onStatus(cf))
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

	rootCluster, err := key.RootClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	tc.TLS, err = key.TeleportClientTLSConfig(nil, []string{rootCluster})
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AuthMethods = []ssh.AuthMethod{identityAuth}
	tc.Interactive = false
	tc.SkipLocalAuth = true

	// When user logs in for the first time without a CA in ~/.tsh/known_hosts,
	// and specifies the -out flag, we need to avoid writing anything to
	// ~/.tsh/ but still validate the proxy cert. Because the existing
	// client.Client methods have a side-effect of persisting the CA on disk,
	// we do all of this by hand.
	//
	// Wrap tc.HostKeyCallback with a another checker. This outer checker uses
	// key.TrustedCA to validate the remote host cert first, before falling
	// back to the original HostKeyCallback.
	oldHostKeyCallback := tc.HostKeyCallback
	tc.HostKeyCallback = func(hostname string, remote net.Addr, hostKey ssh.PublicKey) error {
		checker := ssh.CertChecker{
			// ssh.CertChecker will parse hostKey, extract public key of the
			// signer (CA) and call IsHostAuthority. IsHostAuthority in turn
			// has to match hostCAKey to any known trusted CA.
			IsHostAuthority: func(hostCAKey ssh.PublicKey, address string) bool {
				for _, ca := range key.TrustedCA {
					caKeys, err := ca.SSHCertPublicKeys()
					if err != nil {
						return false
					}
					for _, caKey := range caKeys {
						if apisshutils.KeysEqual(caKey, hostCAKey) {
							return true
						}
					}
				}
				return false
			},
		}
		err := checker.CheckHostKey(hostname, remote, hostKey)
		if err != nil {
			if oldHostKeyCallback == nil {
				return trace.Wrap(err)
			}
			errOld := oldHostKeyCallback(hostname, remote, hostKey)
			if errOld != nil {
				return trace.NewAggregate(err, errOld)
			}
		}
		return nil
	}
	return nil
}

// onLogout deletes a "session certificate" from ~/.tsh for a given proxy
func onLogout(cf *CLIConf) error {
	// Extract all clusters the user is currently logged into.
	active, available, err := client.Status(cf.HomePath, "")
	if err != nil {
		if trace.IsNotFound(err) {
			fmt.Printf("All users logged out.\n")
			return nil
		} else if trace.IsAccessDenied(err) {
			fmt.Printf("%v: Logged in user does not have the correct permissions\n", err)
			return nil
		}
		return trace.Wrap(err)
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
			return trace.Wrap(err)
		}

		// Load profile for the requested proxy/user.
		profile, err := client.StatusFor(cf.HomePath, proxyHost, cf.Username)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		// Log out user from the databases.
		if profile != nil {
			for _, db := range profile.Databases {
				log.Debugf("Logging %v out of database %v.", profile.Name, db)
				err = dbprofile.Delete(tc, db)
				if err != nil {
					return trace.Wrap(err)
				}
			}
		}

		// Remove keys for this user from disk and running agent.
		err = tc.Logout()
		if err != nil {
			if trace.IsNotFound(err) {
				fmt.Printf("User %v already logged out from %v.\n", cf.Username, proxyHost)
				return trace.Wrap(&exitCodeError{code: 1})
			}
			return trace.Wrap(err)
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
			return trace.Wrap(err)
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
			return trace.Wrap(err)
		}

		// Remove Teleport related entries from kubeconfig for all clusters.
		for _, profile := range profiles {
			log.Debugf("Removing Teleport related entries for '%v' from kubeconfig.", profile.Cluster)
			err = kubeconfig.Remove("", profile.Cluster)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		// Remove all database access related profiles as well such as Postgres
		// connection service file.
		for _, profile := range profiles {
			for _, db := range profile.Databases {
				log.Debugf("Logging %v out of database %v.", profile.Name, db)
				err = dbprofile.Delete(tc, db)
				if err != nil {
					return trace.Wrap(err)
				}
			}
		}

		// Remove all keys from disk and the running agent.
		err = tc.LogoutAll()
		if err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf("Logged out all users from all proxies.\n")
	default:
		fmt.Printf("Specify --proxy and --user to remove keys for specific user ")
		fmt.Printf("from a proxy or neither to log out all users from all proxies.\n")
	}
	return nil
}

// onListNodes executes 'tsh ls' command.
func onListNodes(cf *CLIConf) error {
	if cf.ListAll {
		return trace.Wrap(listNodesAllClusters(cf))
	}

	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get list of all nodes in backend and sort by "Node Name".
	var nodes []types.Server
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		nodes, err = tc.ListNodesWithFilters(cf.Context)
		return err
	})
	if err != nil {
		return trace.Wrap(err)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].GetHostname() < nodes[j].GetHostname()
	})

	if err := printNodes(nodes, cf.Format, cf.Verbose); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type nodeListing struct {
	Proxy   string       `json:"proxy"`
	Cluster string       `json:"cluster"`
	Node    types.Server `json:"node"`
}

type nodeListings []nodeListing

func (l nodeListings) Len() int {
	return len(l)
}

func (l nodeListings) Less(i, j int) bool {
	if l[i].Proxy != l[j].Proxy {
		return l[i].Proxy < l[j].Proxy
	}
	if l[i].Cluster != l[j].Cluster {
		return l[i].Cluster < l[j].Cluster
	}
	return l[i].Node.GetHostname() < l[j].Node.GetHostname()
}

func (l nodeListings) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func listNodesAllClusters(cf *CLIConf) error {
	var listings nodeListings

	err := forEachProfile(cf, func(tc *client.TeleportClient, profile *client.ProfileStatus) error {
		result, err := tc.ListNodesWithFiltersAllClusters(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		for clusterName, nodes := range result {
			for _, node := range nodes {
				listings = append(listings, nodeListing{
					Proxy:   profile.ProxyURL.Host,
					Cluster: clusterName,
					Node:    node,
				})
			}
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	sort.Sort(listings)

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		printNodesWithClusters(listings, cf.Verbose)
	case teleport.JSON, teleport.YAML:
		out, err := serializeNodesWithClusters(listings, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}

	// Sometimes a user won't see any nodes because they're missing principals.
	if len(listings) == 0 {
		fmt.Fprintln(os.Stderr, missingPrincipalsFooter)
	}

	return nil
}

func printNodesWithClusters(nodes []nodeListing, verbose bool) {
	var rows [][]string
	for _, n := range nodes {
		rows = append(rows, getNodeRow(n.Proxy, n.Cluster, n.Node, verbose))
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable([]string{"Proxy", "Cluster", "Node Name", "Node ID", "Address", "Labels"}, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn([]string{"Proxy", "Cluster", "Node Name", "Address", "Labels"}, rows, "Labels")
	}
	fmt.Println(t.AsBuffer().String())
}

func serializeNodesWithClusters(nodes []nodeListing, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(nodes, "", "  ")
	} else {
		out, err = yaml.Marshal(nodes)
	}
	return string(out), trace.Wrap(err)
}

func getAccessRequest(ctx context.Context, tc *client.TeleportClient, requestID, username string) (types.AccessRequest, error) {
	var req types.AccessRequest
	err := tc.WithRootClusterClient(ctx, func(clt auth.ClientI) error {
		reqs, err := clt.GetAccessRequests(ctx, types.AccessRequestFilter{
			ID:   requestID,
			User: username,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if len(reqs) != 1 {
			return trace.BadParameter(`invalid access request "%v"`, requestID)
		}
		req = reqs[0]
		return nil
	})
	return req, trace.Wrap(err)
}

func createAccessRequest(cf *CLIConf) (types.AccessRequest, error) {
	roles := utils.SplitIdentifiers(cf.DesiredRoles)
	reviewers := utils.SplitIdentifiers(cf.SuggestedReviewers)
	var requestedResourceIDs []types.ResourceID
	for _, resourceIDString := range cf.RequestedResourceIDs {
		resourceID, err := types.ResourceIDFromString(resourceIDString)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedResourceIDs = append(requestedResourceIDs, resourceID)
	}
	req, err := services.NewAccessRequestWithResources(cf.Username, roles, requestedResourceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.SetRequestReason(cf.RequestReason)
	req.SetSuggestedReviewers(reviewers)
	return req, nil
}

func executeAccessRequest(cf *CLIConf, tc *client.TeleportClient) error {
	if cf.DesiredRoles == "" && cf.RequestID == "" && len(cf.RequestedResourceIDs) == 0 {
		return trace.BadParameter("at least one role or resource or a request ID must be specified")
	}
	if cf.Username == "" {
		cf.Username = tc.Username
	}

	var req types.AccessRequest
	var err error
	if cf.RequestID != "" {
		// This access request already exists, fetch it.
		req, err = getAccessRequest(cf.Context, tc, cf.RequestID, cf.Username)
		if err != nil {
			return trace.Wrap(err)
		}
		// If the request isn't pending, handle resolution
		if !req.GetState().IsPending() {
			err := onRequestResolution(cf, tc, req)
			return trace.Wrap(err)
		}
		fmt.Fprint(os.Stdout, "Request pending...\n")
	} else {
		// This is a new access request, create it. This just creates the local
		// object, it is not yet sent to the backend.
		req, err = createAccessRequest(cf)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Watch for resolution events on the given request. Start watcher and wait
	// for it to be ready before creating the request to avoid a potential race.
	requestWatcher := newAccessRequestWatcher(req)
	defer requestWatcher.Close()
	if !cf.NoWait {
		// Don't initialize the watcher unless we'll actually use it.
		if err := requestWatcher.initialize(cf.Context, tc); err != nil {
			return trace.Wrap(err)
		}
	}

	// Upsert request if it doesn't already exist.
	if cf.RequestID == "" {
		cf.RequestID = req.GetName()
		fmt.Fprint(os.Stdout, "Creating request...\n")
		// always create access request against the root cluster
		if err := tc.WithRootClusterClient(cf.Context, func(clt auth.ClientI) error {
			err := clt.CreateAccessRequest(cf.Context, req)
			return trace.Wrap(err)
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	onRequestShow(cf)
	fmt.Println("")

	// Don't wait for request to get resolved, just print out request info.
	if cf.NoWait {
		return nil
	}

	// Wait for the request to be resolved.
	fmt.Fprintf(os.Stdout, "Waiting for request approval...\n")
	resolvedReq, err := requestWatcher.awaitResolution()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := requestWatcher.Close(); err != nil {
		// This was deferred above to catch all other error cases, here we
		// actually handle any errors from requestWatcher.Close().
		return trace.Wrap(err)
	}

	// Handle resolution and update client certs if approved.
	return trace.Wrap(onRequestResolution(cf, tc, resolvedReq))
}

func printNodes(nodes []types.Server, format string, verbose bool) error {
	format = strings.ToLower(format)
	switch format {
	case teleport.Text, "":
		printNodesAsText(nodes, verbose)
	case teleport.JSON, teleport.YAML:
		out, err := serializeNodes(nodes, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	case teleport.Names:
		for _, n := range nodes {
			fmt.Println(n.GetHostname())
		}
	default:
		return trace.BadParameter("unsupported format %q", format)
	}

	// Sometimes a user won't see any nodes because they're missing principals.
	if len(nodes) == 0 {
		fmt.Fprintln(os.Stderr, missingPrincipalsFooter)
	}

	return nil
}

func serializeNodes(nodes []types.Server, format string) (string, error) {
	if nodes == nil {
		nodes = []types.Server{}
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(nodes, "", "  ")
	} else {
		out, err = yaml.Marshal(nodes)
	}
	return string(out), trace.Wrap(err)
}

func getNodeRow(proxy, cluster string, node types.Server, verbose bool) []string {
	// Reusable function to get addr or tunnel for each node
	getAddr := func(n types.Server) string {
		if n.GetUseTunnel() {
			return "⟵ Tunnel"
		}
		return n.GetAddr()
	}

	row := make([]string, 0)
	if proxy != "" && cluster != "" {
		row = append(row, proxy, cluster)
	}

	if verbose {
		row = append(row, node.GetHostname(), node.GetName(), getAddr(node), node.LabelsString())
	} else {
		row = append(row, node.GetHostname(), getAddr(node), sortedLabels(node.GetAllLabels()))
	}
	return row
}

func printNodesAsText(nodes []types.Server, verbose bool) {
	var rows [][]string
	for _, n := range nodes {
		rows = append(rows, getNodeRow("", "", n, verbose))
	}
	var t asciitable.Table
	switch verbose {
	// In verbose mode, print everything on a single line and include the Node
	// ID (UUID). Useful for machines that need to parse the output of "tsh ls".
	case true:
		t = asciitable.MakeTable([]string{"Node Name", "Node ID", "Address", "Labels"}, rows...)
	// In normal mode chunk the labels and print two per line and allow multiple
	// lines per node.
	case false:
		t = asciitable.MakeTableWithTruncatedColumn([]string{"Node Name", "Address", "Labels"}, rows, "Labels")
	}
	fmt.Println(t.AsBuffer().String())
}

func sortedLabels(labels map[string]string) string {
	var teleportNamespaced []string
	var namespaced []string
	var result []string
	for key, val := range labels {
		if strings.HasPrefix(key, types.TeleportNamespace+"/") {
			teleportNamespaced = append(teleportNamespaced, key)
			continue
		}
		if strings.Contains(key, "/") {
			namespaced = append(namespaced, fmt.Sprintf("%s=%s", key, val))
			continue
		}
		result = append(result, fmt.Sprintf("%s=%s", key, val))
	}
	sort.Strings(result)
	sort.Strings(namespaced)
	sort.Strings(teleportNamespaced)
	namespaced = append(namespaced, teleportNamespaced...)
	return strings.Join(append(result, namespaced...), ",")
}

func showApps(apps []types.Application, active []tlsca.RouteToApp, format string, verbose bool) error {
	format = strings.ToLower(format)
	switch format {
	case teleport.Text, "":
		showAppsAsText(apps, active, verbose)
	case teleport.JSON, teleport.YAML:
		out, err := serializeApps(apps, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}
	return nil
}

func serializeApps(apps []types.Application, format string) (string, error) {
	if apps == nil {
		apps = []types.Application{}
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(apps, "", "  ")
	} else {
		out, err = yaml.Marshal(apps)
	}
	return string(out), trace.Wrap(err)
}

func getAppRow(proxy, cluster string, app types.Application, active []tlsca.RouteToApp, verbose bool) []string {
	var row []string
	if proxy != "" && cluster != "" {
		row = append(row, proxy, cluster)
	}

	name := app.GetName()
	for _, a := range active {
		if name == a.Name {
			name = fmt.Sprintf("> %v", name)
			break
		}
	}
	if verbose {
		row = append(row, name, app.GetDescription(), app.GetPublicAddr(), app.GetURI(), sortedLabels(app.GetAllLabels()))
	} else {
		row = append(row, name, app.GetDescription(), app.GetPublicAddr(), sortedLabels(app.GetAllLabels()))
	}
	return row
}

func showAppsAsText(apps []types.Application, active []tlsca.RouteToApp, verbose bool) {
	var rows [][]string
	for _, app := range apps {
		rows = append(rows, getAppRow("", "", app, active, verbose))
	}
	// In verbose mode, print everything on a single line and include host UUID.
	// In normal mode, chunk the labels, print two per line and allow multiple
	// lines per node.
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable([]string{"Application", "Description", "Public Address", "URI", "Labels"}, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(
			[]string{"Application", "Description", "Public Address", "Labels"}, rows, "Labels")
	}
	fmt.Println(t.AsBuffer().String())
}

func showDatabases(w io.Writer, clusterFlag string, databases []types.Database, active []tlsca.RouteToDatabase, roleSet services.RoleSet, format string, verbose bool) error {
	format = strings.ToLower(format)
	switch format {
	case teleport.Text, "":
		showDatabasesAsText(w, clusterFlag, databases, active, roleSet, verbose)
	case teleport.JSON, teleport.YAML:
		out, err := serializeDatabases(databases, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(w, out)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}
	return nil
}

func serializeDatabases(databases []types.Database, format string) (string, error) {
	if databases == nil {
		databases = []types.Database{}
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(databases, "", "  ")
	} else {
		out, err = yaml.Marshal(databases)
	}
	return string(out), trace.Wrap(err)
}

func serializeDatabasesAllClusters(dbListings []databaseListing, format string) (string, error) {
	if dbListings == nil {
		dbListings = []databaseListing{}
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(dbListings, "", "  ")
	} else {
		out, err = yaml.Marshal(dbListings)
	}
	return string(out), trace.Wrap(err)
}

func getUsersForDb(database types.Database, roleSet services.RoleSet) string {
	// may happen if fetching the role set failed for any reason.
	if roleSet == nil {
		return "(unknown)"
	}

	dbUsers := roleSet.EnumerateDatabaseUsers(database)
	allowed := dbUsers.Allowed()

	if dbUsers.WildcardAllowed() {
		// start the list with *
		allowed = append([]string{types.Wildcard}, allowed...)
	}

	if len(allowed) == 0 {
		return "(none)"
	}

	denied := dbUsers.Denied()
	if len(denied) == 0 || !dbUsers.WildcardAllowed() {
		return fmt.Sprintf("%v", allowed)
	}
	return fmt.Sprintf("%v, except: %v", allowed, denied)
}

func getDatabaseRow(proxy, cluster, clusterFlag string, database types.Database, active []tlsca.RouteToDatabase, roleSet services.RoleSet, verbose bool) []string {
	name := database.GetName()
	var connect string
	for _, a := range active {
		if a.ServiceName == name {
			name = formatActiveDB(a)
			connect = formatConnectCommand(clusterFlag, a)
		}
	}

	row := make([]string, 0)
	if proxy != "" && cluster != "" {
		row = append(row, proxy, cluster)
	}

	if verbose {
		row = append(row,
			name,
			database.GetDescription(),
			database.GetProtocol(),
			database.GetType(),
			database.GetURI(),
			getUsersForDb(database, roleSet),
			database.LabelsString(),
			connect,
			database.Expiry().Format(constants.HumanDateFormatSeconds),
		)
	} else {
		row = append(row,
			name,
			database.GetDescription(),
			getUsersForDb(database, roleSet),
			formatDatabaseLabels(database),
			connect,
		)
	}

	return row
}

func showDatabasesAsText(w io.Writer, clusterFlag string, databases []types.Database, active []tlsca.RouteToDatabase, roleSet services.RoleSet, verbose bool) {
	var rows [][]string
	for _, database := range databases {
		rows = append(rows, getDatabaseRow("", "",
			clusterFlag,
			database,
			active,
			roleSet,
			verbose))
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable([]string{"Name", "Description", "Protocol", "Type", "URI", "Allowed Users", "Labels", "Connect", "Expires"}, rows...)
	} else {

		t = asciitable.MakeTableWithTruncatedColumn([]string{"Name", "Description", "Allowed Users", "Labels", "Connect"}, rows, "Labels")
	}
	fmt.Fprintln(w, t.AsBuffer().String())
}

func printDatabasesWithClusters(clusterFlag string, dbListings []databaseListing, active []tlsca.RouteToDatabase, verbose bool) {
	var rows [][]string
	for _, listing := range dbListings {
		rows = append(rows, getDatabaseRow(
			listing.Proxy,
			listing.Cluster,
			clusterFlag,
			listing.Database,
			active,
			listing.roleSet,
			verbose))
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable([]string{"Proxy", "Cluster", "Name", "Description", "Protocol", "Type", "URI", "Allowed Users", "Labels", "Connect", "Expires"}, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(
			[]string{"Proxy", "Cluster", "Name", "Description", "Allowed Users", "Labels", "Connect"},
			rows,
			"Labels",
		)
	}
	fmt.Println(t.AsBuffer().String())
}

func formatDatabaseLabels(database types.Database) string {
	labels := database.GetAllLabels()
	// Hide the origin label unless printing verbose table.
	delete(labels, types.OriginLabel)
	return sortedLabels(labels)
}

// formatConnectCommand formats an appropriate database connection command
// for a user based on the provided database parameters.
func formatConnectCommand(clusterFlag string, active tlsca.RouteToDatabase) string {
	cmdTokens := []string{"tsh", "db", "connect"}

	if clusterFlag != "" {
		cmdTokens = append(cmdTokens, fmt.Sprintf("--cluster=%s", clusterFlag))
	}
	if active.Username == "" {
		cmdTokens = append(cmdTokens, "--db-user=<user>")
	}
	if active.Database == "" {
		cmdTokens = append(cmdTokens, "--db-name=<name>")
	}

	cmdTokens = append(cmdTokens, active.ServiceName)
	return strings.Join(cmdTokens, " ")
}

func formatActiveDB(active tlsca.RouteToDatabase) string {
	switch {
	case active.Username != "" && active.Database != "":
		return fmt.Sprintf("> %v (user: %v, db: %v)", active.ServiceName, active.Username, active.Database)
	case active.Username != "":
		return fmt.Sprintf("> %v (user: %v)", active.ServiceName, active.Username)
	case active.Database != "":
		return fmt.Sprintf("> %v (db: %v)", active.ServiceName, active.Database)
	}
	return fmt.Sprintf("> %v", active.ServiceName)
}

// onListClusters executes 'tsh clusters' command
func onListClusters(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	var rootClusterName string
	var leafClusters []types.RemoteCluster
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		proxyClient, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return err
		}
		defer proxyClient.Close()

		var rootErr, leafErr error
		rootClusterName, rootErr = proxyClient.RootClusterName(cf.Context)
		leafClusters, leafErr = proxyClient.GetLeafClusters(cf.Context)
		return trace.NewAggregate(rootErr, leafErr)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	profile, _, err := client.Status(cf.HomePath, cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	isSelected := func(clusterName string) bool {
		return profile != nil && clusterName == profile.Cluster
	}
	showSelected := func(clusterName string) string {
		if isSelected(clusterName) {
			return "*"
		}
		return ""
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		var t asciitable.Table
		if cf.Quiet {
			t = asciitable.MakeHeadlessTable(4)
		} else {
			t = asciitable.MakeTable([]string{"Cluster Name", "Status", "Cluster Type", "Labels", "Selected"})
		}

		t.AddRow([]string{
			rootClusterName, teleport.RemoteClusterStatusOnline, "root", "", showSelected(rootClusterName),
		})
		for _, cluster := range leafClusters {
			labels := sortedLabels(cluster.GetMetadata().Labels)
			t.AddRow([]string{
				cluster.GetName(), cluster.GetConnectionStatus(), "leaf", labels, showSelected(cluster.GetName()),
			})
		}
		fmt.Println(t.AsBuffer().String())
	case teleport.JSON, teleport.YAML:
		rootClusterInfo := clusterInfo{
			ClusterName: rootClusterName,
			Status:      teleport.RemoteClusterStatusOnline,
			ClusterType: "root",
			Selected:    isSelected(rootClusterName)}
		leafClusterInfo := make([]clusterInfo, 0, len(leafClusters))
		for _, leaf := range leafClusters {
			leafClusterInfo = append(leafClusterInfo, clusterInfo{
				ClusterName: leaf.GetName(),
				Status:      leaf.GetConnectionStatus(),
				ClusterType: "leaf",
				Labels:      leaf.GetMetadata().Labels,
				Selected:    isSelected(leaf.GetName())})
		}
		out, err := serializeClusters(rootClusterInfo, leafClusterInfo, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
	return nil
}

type clusterInfo struct {
	ClusterName string            `json:"cluster_name"`
	Status      string            `json:"status"`
	ClusterType string            `json:"cluster_type"`
	Labels      map[string]string `json:"labels"`
	Selected    bool              `json:"selected"`
}

func serializeClusters(rootCluster clusterInfo, leafClusters []clusterInfo, format string) (string, error) {
	clusters := []clusterInfo{rootCluster}
	clusters = append(clusters, leafClusters...)
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(clusters, "", "  ")
	} else {
		out, err = yaml.Marshal(clusters)
	}
	return string(out), trace.Wrap(err)
}

// accessRequestForSSH attempts to create a resource access request for the case
// where "tsh ssh" was attempted and access was denied
func accessRequestForSSH(ctx context.Context, tc *client.TeleportClient) (types.AccessRequest, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	// Match on hostname or host ID, user could have given either
	expr := fmt.Sprintf(hostnameOrIDPredicateTemplate, tc.Host)
	filter := proto.ListResourcesRequest{
		UseSearchAsRoles:    true,
		PredicateExpression: expr,
	}
	nodes, err := proxyClient.FindNodesByFilters(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(nodes) > 1 {
		// Ambiguous hostname matches should have been handled by onSSH and
		// would not make it here, this is a sanity check. Ambiguous host ID
		// matches should be impossible.
		return nil, trace.NotFound("hostname %q is ambiguous and matches multiple nodes, unable to request access", tc.Host)
	}
	if len(nodes) == 0 {
		// Did not find any nodes by hostname or ID.
		return nil, trace.NotFound("node %q not found, unable to request access", tc.Host)
	}

	// At this point we have exactly 1 node.
	node := nodes[0]
	requestResourceIDs := []types.ResourceID{{
		ClusterName: tc.SiteName,
		Kind:        types.KindNode,
		Name:        node.GetName(),
	}}

	// Roles to request will be automatically determined on the backend.
	req, err := services.NewAccessRequestWithResources(tc.Username, nil /* roles */, requestResourceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.SetLoginHint(tc.HostLogin)

	// Set the DryRun flag and send the request to auth for full validation. If
	// the user has no search_as_roles or is not allowed to SSH to the host with
	// the requested login, we will get an error here.
	req.SetDryRun(true)
	req.SetRequestReason("Dry run, this request will not be created. If you see this, there is a bug.")
	if err := tc.WithRootClusterClient(ctx, func(clt auth.ClientI) error {
		return trace.Wrap(clt.CreateAccessRequest(ctx, req))
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	req.SetDryRun(false)
	req.SetRequestReason("")

	return req, nil
}

func retryWithAccessRequest(cf *CLIConf, tc *client.TeleportClient, fn func() error) error {
	origErr := fn()
	if cf.disableAccessRequest || !trace.IsAccessDenied(origErr) || tc.Host == "" {
		// Return if --disable-access-request was specified.
		// Return the original error if it's not AccessDenied.
		// Quit now if we don't have a hostname.
		return trace.Wrap(origErr)
	}

	// Try to construct an access request for this node.
	req, err := accessRequestForSSH(cf.Context, tc)
	if err != nil {
		// We can't request access to the node or it doesn't exist, return the
		// original error but put this one in the debug log.
		log.WithError(err).Debug("unable to request access to node")
		return trace.Wrap(origErr)
	}
	cf.RequestID = req.GetName()

	// Print and log the original AccessDenied error.
	fmt.Fprintln(os.Stderr, utils.UserMessageFromError(origErr))
	fmt.Fprintf(os.Stdout, "You do not currently have access to %s@%s, attempting to request access.\n\n", tc.HostLogin, tc.Host)

	requestReason := cf.RequestReason
	if requestReason == "" {
		// Prompt for a request reason.
		requestReason, err = prompt.Input(cf.Context, os.Stdout, prompt.Stdin(), "Enter request reason")
		if err != nil {
			return trace.Wrap(err)
		}
	}
	req.SetRequestReason(requestReason)

	// Watch for resolution events on the given request. Start watcher and wait
	// for it to be ready before creating the request to avoid a potential race.
	requestWatcher := newAccessRequestWatcher(req)
	defer requestWatcher.Close()
	if err := requestWatcher.initialize(cf.Context, tc); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprint(os.Stdout, "Creating request...\n")
	// Always create access request against the root cluster.
	if err := tc.WithRootClusterClient(cf.Context, func(clt auth.ClientI) error {
		return trace.Wrap(clt.CreateAccessRequest(cf.Context, req))
	}); err != nil {
		return trace.Wrap(err)
	}

	if cf.Username == "" {
		cf.Username = tc.Username
	}
	// re-fetch the request to display it with roles populated.
	onRequestShow(cf)
	fmt.Println("")

	// Wait for the request to be resolved.
	fmt.Fprintf(os.Stdout, "Waiting for request approval...\n")
	resolvedReq, err := requestWatcher.awaitResolution()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := requestWatcher.Close(); err != nil {
		// This was deferred above to catch all other error cases, here we
		// actually handle any errors from requestWatcher.Close().
		return trace.Wrap(err)
	}

	// Handle resolution and update client certs if approved.
	if err := onRequestResolution(cf, tc, resolvedReq); err != nil {
		return trace.Wrap(err)
	}

	// Retry now that request has been approved and certs updated.
	// Clear the original exit status.
	tc.ExitStatus = 0
	return trace.Wrap(fn())
}

// onSSH executes 'tsh ssh' command
func onSSH(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.Stdin = os.Stdin
	err = retryWithAccessRequest(cf, tc, func() error {
		err = client.RetryWithRelogin(cf.Context, tc, func() error {
			return tc.SSH(cf.Context, cf.RemoteCommand, cf.LocalExec)
		})
		if err != nil {
			if strings.Contains(utils.UserMessageFromError(err), teleport.NodeIsAmbiguous) {
				// Match on hostname or host ID, user could have given either
				expr := fmt.Sprintf(hostnameOrIDPredicateTemplate, tc.Host)
				tc.PredicateExpression = expr
				nodes, err := tc.ListNodesWithFilters(cf.Context)
				if err != nil {
					return trace.Wrap(err)
				}
				fmt.Fprintf(os.Stderr, "error: ambiguous host could match multiple nodes\n\n")
				printNodesAsText(nodes, true)
				fmt.Fprintf(os.Stderr, "Hint: try addressing the node by unique id (ex: tsh ssh user@node-id)\n")
				fmt.Fprintf(os.Stderr, "Hint: use 'tsh ls -v' to list all nodes with their unique ids\n")
				fmt.Fprintf(os.Stderr, "\n")
				return trace.Wrap(&exitCodeError{code: 1})
			}
			return trace.Wrap(err)
		}
		return nil
	})
	// Exit with the same exit status as the failed command.
	if tc.ExitStatus != 0 {
		var exitErr *exitCodeError
		if errors.As(err, &exitErr) {
			// Already have an exitCodeError, return that.
			return trace.Wrap(err)
		}
		if err != nil {
			// Print the error here so we don't lose it when returning the exitCodeError.
			fmt.Fprintln(os.Stderr, utils.UserMessageFromError(err))
		}
		err = &exitCodeError{code: tc.ExitStatus}
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

// onBenchmark executes benchmark
func onBenchmark(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	cnf := benchmark.Config{
		Command:       cf.RemoteCommand,
		MinimumWindow: cf.BenchDuration,
		Rate:          cf.BenchRate,
	}
	result, err := cnf.Benchmark(cf.Context, tc)
	if err != nil {
		fmt.Fprintln(os.Stderr, utils.UserMessageFromError(err))
		return trace.Wrap(&exitCodeError{code: 255})
	}
	fmt.Printf("\n")
	fmt.Printf("* Requests originated: %v\n", result.RequestsOriginated)
	fmt.Printf("* Requests failed: %v\n", result.RequestsFailed)
	if result.LastError != nil {
		fmt.Printf("* Last error: %v\n", result.LastError)
	}
	fmt.Printf("\nHistogram\n\n")
	t := asciitable.MakeTable([]string{"Percentile", "Response Duration"})
	for _, quantile := range []float64{25, 50, 75, 90, 95, 99, 100} {
		t.AddRow([]string{
			fmt.Sprintf("%v", quantile),
			fmt.Sprintf("%v ms", result.Histogram.ValueAtQuantile(quantile)),
		})
	}
	if _, err := io.Copy(os.Stdout, t.AsBuffer()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("\n")
	if cf.BenchExport {
		path, err := benchmark.ExportLatencyProfile(cf.BenchExportPath, result.Histogram, cf.BenchTicks, cf.BenchValueScale)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed exporting latency profile: %s\n", utils.UserMessageFromError(err))
		} else {
			fmt.Printf("latency profile saved: %v\n", path)
		}
	}
	return nil
}

// onJoin executes 'ssh join' command
func onJoin(cf *CLIConf) error {
	if err := validateParticipantMode(types.SessionParticipantMode(cf.JoinMode)); err != nil {
		return trace.Wrap(err)
	}

	cf.NodeLogin = teleport.SSHSessionJoinPrincipal
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}
	sid, err := session.ParseID(cf.SessionID)
	if err != nil {
		return trace.BadParameter("'%v' is not a valid session ID (must be GUID)", cf.SessionID)
	}
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.Join(context.TODO(), types.SessionParticipantMode(cf.JoinMode), cf.Namespace, *sid, nil)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// onSCP executes 'tsh scp' command
func onSCP(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}
	flags := scp.Flags{
		Recursive:     cf.RecursiveCopy,
		PreserveAttrs: cf.PreserveAttrs,
	}
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.SCP(cf.Context, cf.CopySpec, int(cf.NodePort), flags, cf.Quiet)
	})
	if err == nil {
		return nil
	}
	// exit with the same exit status as the failed command:
	if tc.ExitStatus != 0 {
		fmt.Fprintln(os.Stderr, utils.UserMessageFromError(err))
		return trace.Wrap(&exitCodeError{code: tc.ExitStatus})
	}
	return trace.Wrap(err)
}

// makeClient takes the command-line configuration and constructs & returns
// a fully configured TeleportClient object
func makeClient(cf *CLIConf, useProfileLogin bool) (*client.TeleportClient, error) {
	tc, err := makeClientForProxy(cf, cf.Proxy, useProfileLogin)
	return tc, trace.Wrap(err)
}

// makeClient takes the command-line configuration and a proxy address and constructs & returns
// a fully configured TeleportClient object
func makeClientForProxy(cf *CLIConf, proxy string, useProfileLogin bool) (*client.TeleportClient, error) {
	// Parse OpenSSH style options.
	options, err := parseOptions(cf.Options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// apply defaults
	if cf.MinsToLive == 0 {
		cf.MinsToLive = int32(apidefaults.CertDuration / time.Minute)
	}

	// split login & host
	hostLogin := cf.NodeLogin
	var labels map[string]string
	if cf.UserHost != "" {
		parts := strings.Split(cf.UserHost, "@")
		partsLength := len(parts)
		if partsLength > 1 {
			hostLogin = strings.Join(parts[:partsLength-1], "@")
			cf.UserHost = parts[partsLength-1]
		}
		// see if remote host is specified as a set of labels
		if strings.Contains(cf.UserHost, "=") {
			labels, err = client.ParseLabelSpec(cf.UserHost)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	} else if cf.CopySpec != nil {
		for _, location := range cf.CopySpec {
			// Extract username and host from "username@host:file/path"
			parts := strings.Split(location, ":")
			parts = strings.Split(parts[0], "@")
			partsLength := len(parts)
			if partsLength > 1 {
				hostLogin = strings.Join(parts[:partsLength-1], "@")
				cf.UserHost = parts[partsLength-1]
				break
			}
		}
	}
	fPorts, err := client.ParsePortForwardSpec(cf.LocalForwardPorts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dPorts, err := client.ParseDynamicPortForwardSpec(cf.DynamicForwardedPorts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 1: start with the defaults
	c := client.MakeDefaultConfig()
	c.Host = cf.UserHost
	if cf.TracingProvider == nil {
		cf.TracingProvider = tracing.NoopProvider()
	}
	c.Tracer = cf.TracingProvider.Tracer(teleport.ComponentTSH)

	// ProxyJump is an alias of Proxy flag
	if cf.ProxyJump != "" {
		proxyJump := cf.ProxyJump
		if strings.Contains(cf.ProxyJump, "{{proxy}}") {
			proxy, host, matched := cf.TshConfig.ProxyTemplates.Apply(c.Host)
			if !matched {
				return nil, trace.BadParameter("proxy jump contains {{proxy}} variable but did not match any of the templates in tsh config")
			}
			proxyJump = strings.ReplaceAll(proxyJump, "{{proxy}}", proxy)
			c.Host = host
			log.Debugf("Will connect to proxy %q and host %q according to proxy templates.", proxyJump, host)
		}
		hosts, err := utils.ParseProxyJump(proxyJump)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		c.JumpHosts = hosts
	}

	// Look if a user identity was given via -i flag
	if cf.IdentityFileIn != "" {
		// Ignore local authentication methods when identity file is provided
		c.SkipLocalAuth = true
		// Force the use of the certificate principals so Unix
		// username does not get used when logging in
		c.UseKeyPrincipals = hostLogin == ""

		var (
			key          *client.Key
			identityAuth ssh.AuthMethod
			expiryDate   time.Time
			hostAuthFunc ssh.HostKeyCallback
		)
		// read the ID file and create an "auth method" from it:
		key, err = client.KeyFromIdentityFile(cf.IdentityFileIn)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		rootCluster, err := key.RootClusterName()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters := []string{rootCluster}
		if cf.SiteName != "" {
			clusters = append(clusters, cf.SiteName)
		}
		hostAuthFunc, err = key.HostKeyCallbackForClusters(cf.InsecureSkipVerify, apiutils.Deduplicate(clusters))
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

		// Also configure missing KeyIndex fields.
		key.ProxyHost, err = utils.Host(proxy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		key.ClusterName = rootCluster
		key.Username = certUsername

		// With the key index fields properly set, preload this key into a local store.
		c.PreloadKey = key

		identityAuth, err = authFromIdentity(key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		c.AuthMethods = []ssh.AuthMethod{identityAuth}

		// Also create an in-memory agent to hold the key. If cluster is in
		// proxy recording mode, agent forwarding will be required for
		// sessions.
		c.Agent = agent.NewKeyring()
		agentKeys, err := key.AsAgentKeys()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, k := range agentKeys {
			if err := c.Agent.Add(k); err != nil {
				return nil, trace.Wrap(err)
			}
		}

		if len(key.TLSCert) > 0 {
			c.TLS, err = key.TeleportClientTLSConfig(nil, clusters)
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
		// load profile. if no --proxy is given the currently active profile is used, otherwise
		// fetch profile for exact proxy we are trying to connect to.
		err = c.LoadProfile(cf.HomePath, proxy)
		if err != nil {
			fmt.Printf("WARNING: Failed to load tsh profile for %q: %v\n", proxy, err)
		}
	}
	// 3: override with the CLI flags
	if cf.Namespace != "" {
		c.Namespace = cf.Namespace
	}
	if cf.Username != "" {
		c.Username = cf.Username
	}
	c.ExplicitUsername = cf.ExplicitUsername
	// if proxy is set, and proxy is not equal to profile's
	// loaded addresses, override the values
	if err := setClientWebProxyAddr(cf, c); err != nil {
		return nil, trace.Wrap(err)
	}

	if c.ExtraProxyHeaders == nil {
		c.ExtraProxyHeaders = map[string]string{}
	}
	for _, proxyHeaders := range cf.TshConfig.ExtraHeaders {
		proxyGlob := utils.GlobToRegexp(proxyHeaders.Proxy)
		proxyRegexp, err := regexp.Compile(proxyGlob)
		if err != nil {
			return nil, trace.Wrap(err, "invalid proxy glob %q in tsh configuration file", proxyGlob)
		}
		if proxyRegexp.MatchString(c.WebProxyAddr) {
			for k, v := range proxyHeaders.Headers {
				c.ExtraProxyHeaders[k] = v
			}
		}
	}

	if len(fPorts) > 0 {
		c.LocalForwardPorts = fPorts
	}
	if len(dPorts) > 0 {
		c.DynamicForwardedPorts = dPorts
	}
	profileSiteName := c.SiteName
	if cf.SiteName != "" {
		c.SiteName = cf.SiteName
	}
	if cf.KubernetesCluster != "" {
		c.KubernetesCluster = cf.KubernetesCluster
	}
	if cf.DatabaseService != "" {
		c.DatabaseService = cf.DatabaseService
	}
	// if host logins stored in profiles must be ignored...
	if !useProfileLogin {
		c.HostLogin = ""
	}
	if hostLogin != "" {
		c.HostLogin = hostLogin
	}
	c.HostPort = int(cf.NodePort)
	c.Labels = labels
	c.KeyTTL = time.Minute * time.Duration(cf.MinsToLive)
	c.InsecureSkipVerify = cf.InsecureSkipVerify
	c.PredicateExpression = cf.PredicateExpression

	if cf.SearchKeywords != "" {
		c.SearchKeywords = client.ParseSearchKeywords(cf.SearchKeywords, ',')
	}

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
	mfaOpts, err := parseMFAMode(cf.MFAMode)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.AuthenticatorAttachment = mfaOpts.AuthenticatorAttachment
	c.PreferOTP = mfaOpts.PreferOTP

	// If agent forwarding was specified on the command line enable it.
	c.ForwardAgent = options.ForwardAgent
	if cf.ForwardAgent {
		c.ForwardAgent = client.ForwardAgentYes
	}

	if err := setX11Config(c, cf, options, os.Getenv); err != nil {
		log.WithError(err).Info("X11 forwarding is not properly configured, continuing without it.")
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

	c.AddKeysToAgent = cf.AddKeysToAgent
	if !cf.UseLocalSSHAgent {
		c.AddKeysToAgent = client.AddKeysToAgentNo
	}

	c.EnableEscapeSequences = cf.EnableEscapeSequences

	// pass along mock sso login if provided (only used in tests)
	c.MockSSOLogin = cf.mockSSOLogin

	// Set tsh home directory
	c.HomePath = cf.HomePath

	if c.KeysDir == "" {
		c.KeysDir = c.HomePath
	}

	tc, err := client.NewClient(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Load SSH key for the cluster indicated in the profile.
	// Handle gracefully if the profile is empty or if the key cannot be found.
	if profileSiteName != "" {
		if err := tc.LoadKeyForCluster(profileSiteName); err != nil {
			log.Debug(err)
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
		}
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

	tc.Config.Stderr = cf.Stderr()
	tc.Config.Stdout = cf.Stdout()

	tc.Config.Reason = cf.Reason
	tc.Config.Invited = cf.Invited
	tc.Config.DisplayParticipantRequirements = cf.displayParticipantRequirements
	return tc, nil
}

type mfaModeOpts struct {
	AuthenticatorAttachment wancli.AuthenticatorAttachment
	PreferOTP               bool
}

func parseMFAMode(mode string) (*mfaModeOpts, error) {
	opts := &mfaModeOpts{}
	switch mode {
	case "", mfaModeAuto:
	case mfaModeCrossPlatform:
		opts.AuthenticatorAttachment = wancli.AttachmentCrossPlatform
	case mfaModePlatform:
		opts.AuthenticatorAttachment = wancli.AttachmentPlatform
	case mfaModeOTP:
		opts.PreferOTP = true
	default:
		return nil, fmt.Errorf("invalid MFA mode: %q", mode)
	}
	return opts, nil
}

// setX11Config sets X11 config using CLI and SSH option flags.
func setX11Config(c *client.Config, cf *CLIConf, o Options, fn envGetter) error {
	// X11 forwarding can be enabled with -X, -Y, or -oForwardX11=yes
	c.EnableX11Forwarding = cf.X11ForwardingUntrusted || cf.X11ForwardingTrusted || o.ForwardX11

	if c.EnableX11Forwarding && fn(x11.DisplayEnv) == "" {
		c.EnableX11Forwarding = false
		return trace.BadParameter("$DISPLAY must be set for X11 forwarding")
	}

	c.X11ForwardingTrusted = cf.X11ForwardingTrusted
	if o.ForwardX11Trusted != nil && *o.ForwardX11Trusted {
		c.X11ForwardingTrusted = true
	}

	// Set X11 forwarding timeout, prioritizing the SSH option if set.
	c.X11ForwardingTimeout = o.ForwardX11Timeout
	if c.X11ForwardingTimeout == 0 {
		c.X11ForwardingTimeout = cf.X11ForwardingTimeout
	}

	return nil
}

// defaultWebProxyPorts is the order of default proxy ports to try, in order that
// they will be tried.
var defaultWebProxyPorts = []int{
	defaults.HTTPListenPort, teleport.StandardHTTPSPort,
}

// setClientWebProxyAddr configures the client WebProxyAddr and SSHProxyAddr
// configuration values. Values that are not fully specified via configuration
// or command-line options will be deduced if necessary.
//
// If successful, setClientWebProxyAddr will modify the client Config in-place.
func setClientWebProxyAddr(cf *CLIConf, c *client.Config) error {
	// If the user has specified a proxy on the command line, and one has not
	// already been specified from configuration...

	if cf.Proxy != "" && c.WebProxyAddr == "" {
		parsedAddrs, err := client.ParseProxyHost(cf.Proxy)
		if err != nil {
			return trace.Wrap(err)
		}

		proxyAddress := parsedAddrs.WebProxyAddr
		if parsedAddrs.UsingDefaultWebProxyPort {
			log.Debug("Web proxy port was not set. Attempting to detect port number to use.")
			timeout, cancel := context.WithTimeout(context.Background(), proxyDefaultResolutionTimeout)
			defer cancel()

			proxyAddress, err = pickDefaultAddr(
				timeout, cf.InsecureSkipVerify, parsedAddrs.Host, defaultWebProxyPorts)

			// On error, fall back to the legacy behavior
			if err != nil {
				log.WithError(err).Debug("Proxy port resolution failed, falling back to legacy default.")
				return c.ParseProxyHost(cf.Proxy)
			}
		}

		c.WebProxyAddr = proxyAddress
		c.SSHProxyAddr = parsedAddrs.SSHProxyAddr
	}

	return nil
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
func refuseArgs(command string, args []string) error {
	for _, arg := range args {
		if arg == command || strings.HasPrefix(arg, "-") {
			continue
		} else {
			return trace.BadParameter("unexpected argument: %s", arg)
		}
	}
	return nil
}

// authFromIdentity returns a standard ssh.Authmethod for a given identity file
func authFromIdentity(k *client.Key) (ssh.AuthMethod, error) {
	signer, err := sshutils.NewSigner(k.Priv, k.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.PublicKeys(signer), nil
}

// onShow reads an identity file (a public SSH key or a cert) and dumps it to stdout
func onShow(cf *CLIConf) error {
	key, err := client.KeyFromIdentityFile(cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}

	// unmarshal certificate bytes into a ssh.PublicKey
	cert, _, _, _, err := ssh.ParseAuthorizedKey(key.Cert)
	if err != nil {
		return trace.Wrap(err)
	}

	// unmarshal private key bytes into a *rsa.PrivateKey
	priv, err := ssh.ParseRawPrivateKey(key.Priv)
	if err != nil {
		return trace.Wrap(err)
	}

	pub, err := ssh.ParsePublicKey(key.Pub)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Cert: %#v\nPriv: %#v\nPub: %#v\n",
		cert, priv, pub)

	fmt.Printf("Fingerprint: %s\n", ssh.FingerprintSHA256(pub))
	return nil
}

// printStatus prints the status of the profile.
func printStatus(debug bool, p *profileInfo, env map[string]string, isActive bool) {
	var prefix string
	duration := time.Until(p.ValidUntil)
	humanDuration := "EXPIRED"
	if duration.Nanoseconds() > 0 {
		humanDuration = fmt.Sprintf("valid for %v", duration.Round(time.Minute))
	}

	proxyURL := p.getProxyURLLine(isActive, env)
	cluster := p.getClusterLine(isActive, env)
	kubeCluster := p.getKubeClusterLine(isActive, env, cluster)
	if isActive {
		prefix = "> "
	} else {
		prefix = "  "
	}

	fmt.Printf("%vProfile URL:        %v\n", prefix, proxyURL)
	fmt.Printf("  Logged in as:       %v\n", p.Username)
	if len(p.ActiveRequests) != 0 {
		fmt.Printf("  Active requests:    %v\n", strings.Join(p.ActiveRequests, ", "))
	}

	if cluster != "" {
		fmt.Printf("  Cluster:            %v\n", cluster)
	}
	fmt.Printf("  Roles:              %v\n", strings.Join(p.Roles, ", "))
	if debug {
		var count int
		for k, v := range p.Traits {
			if count == 0 {
				fmt.Printf("  Traits:             %v: %v\n", k, v)
			} else {
				fmt.Printf("                      %v: %v\n", k, v)
			}
			count = count + 1
		}
	}
	fmt.Printf("  Logins:             %v\n", strings.Join(p.Logins, ", "))
	if p.KubernetesEnabled {
		fmt.Printf("  Kubernetes:         enabled\n")
		if kubeCluster != "" {
			fmt.Printf("  Kubernetes cluster: %q\n", kubeCluster)
		}
		if len(p.KubernetesUsers) > 0 {
			fmt.Printf("  Kubernetes users:   %v\n", strings.Join(p.KubernetesUsers, ", "))
		}
		if len(p.KubernetesGroups) > 0 {
			fmt.Printf("  Kubernetes groups:  %v\n", strings.Join(p.KubernetesGroups, ", "))
		}
	} else {
		fmt.Printf("  Kubernetes:         disabled\n")
	}
	if len(p.Databases) != 0 {
		fmt.Printf("  Databases:          %v\n", strings.Join(p.Databases, ", "))
	}
	if len(p.AllowedResourceIDs) > 0 {
		allowedResourcesStr, err := types.ResourceIDsToString(p.AllowedResourceIDs)
		if err != nil {
			log.Warnf("failed to marshal allowed resource IDs to string: %v", err)
		} else {
			fmt.Printf("  Allowed Resources:  %s\n", allowedResourcesStr)
		}
	}
	fmt.Printf("  Valid until:        %v [%v]\n", p.ValidUntil, humanDuration)
	fmt.Printf("  Extensions:         %v\n", strings.Join(p.Extensions, ", "))

	if debug {
		first := true
		for k, v := range p.CriticalOptions {
			if first {
				fmt.Printf("  Critical options:   %v: %v\n", k, v)
			} else {
				fmt.Printf("                      %v: %v\n", k, v)
			}
			first = false
		}
	}

	fmt.Printf("\n")
}

// onStatus command shows which proxy the user is logged into and metadata
// about the certificate.
func onStatus(cf *CLIConf) error {
	// Get the status of the active profile as well as the status
	// of any other proxies the user is logged into.
	//
	// Return error if not logged in, no active profile, or expired.
	profile, profiles, err := client.Status(cf.HomePath, cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}

	env := getTshEnv()
	active, others := makeAllProfileInfo(profile, profiles, env)

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.JSON, teleport.YAML:
		out, err := serializeProfiles(active, others, env, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		printProfiles(cf.Debug, active, others, env, cf.Verbose)
	}

	if profile == nil {
		return trace.NotFound("Not logged in.")
	}

	duration := time.Until(profile.ValidUntil)
	if !profile.ValidUntil.IsZero() && duration.Nanoseconds() <= 0 {
		return trace.NotFound("Active profile expired.")
	}

	return nil
}

type profileInfo struct {
	ProxyURL           string             `json:"profile_url"`
	Username           string             `json:"username"`
	ActiveRequests     []string           `json:"active_requests,omitempty"`
	Cluster            string             `json:"cluster"`
	Roles              []string           `json:"roles,omitempty"`
	Traits             wrappers.Traits    `json:"traits,omitempty"`
	Logins             []string           `json:"logins,omitempty"`
	KubernetesEnabled  bool               `json:"kubernetes_enabled"`
	KubernetesCluster  string             `json:"kubernetes_cluster,omitempty"`
	KubernetesUsers    []string           `json:"kubernetes_users,omitempty"`
	KubernetesGroups   []string           `json:"kubernetes_groups,omitempty"`
	Databases          []string           `json:"databases,omitempty"`
	ValidUntil         time.Time          `json:"valid_until"`
	Extensions         []string           `json:"extensions,omitempty"`
	CriticalOptions    map[string]string  `json:"critical_options,omitempty"`
	AllowedResourceIDs []types.ResourceID `json:"allowed_resources,omitempty"`
}

func makeAllProfileInfo(active *client.ProfileStatus, others []*client.ProfileStatus, env map[string]string) (*profileInfo, []*profileInfo) {
	activeInfo := makeProfileInfo(active, env, true)
	var othersInfo []*profileInfo
	for _, p := range others {
		othersInfo = append(othersInfo, makeProfileInfo(p, env, false))
	}
	return activeInfo, othersInfo
}

func makeProfileInfo(p *client.ProfileStatus, env map[string]string, isActive bool) *profileInfo {
	if p == nil {
		return nil
	}
	out := &profileInfo{
		ProxyURL:           p.ProxyURL.String(),
		Username:           p.Username,
		ActiveRequests:     p.ActiveRequests.AccessRequests,
		Cluster:            p.Cluster,
		Roles:              p.Roles,
		Traits:             p.Traits,
		Logins:             p.Logins,
		KubernetesEnabled:  p.KubeEnabled,
		KubernetesCluster:  selectedKubeCluster(p.Cluster),
		KubernetesUsers:    p.KubeUsers,
		KubernetesGroups:   p.KubeGroups,
		Databases:          p.DatabaseServices(),
		ValidUntil:         p.ValidUntil,
		Extensions:         p.Extensions,
		CriticalOptions:    p.CriticalOptions,
		AllowedResourceIDs: p.AllowedResourceIDs,
	}

	// update active profile info from env
	if isActive {
		if proxy, ok := env[proxyEnvVar]; ok {
			proxyURL := url.URL{
				Scheme: "https",
				Host:   proxy,
			}
			out.ProxyURL = proxyURL.String()
		}

		if cluster, ok := env[clusterEnvVar]; ok {
			out.Cluster = cluster
		} else if siteName, ok := env[siteEnvVar]; ok {
			out.Cluster = siteName
		}

		if kubeCluster, ok := env[kubeClusterEnvVar]; ok {
			out.KubernetesCluster = kubeCluster
		}
	}
	return out
}

func (p *profileInfo) getProxyURLLine(isActive bool, env map[string]string) string {
	// indicate if active profile proxy url is shadowed by env vars.
	if isActive {
		if _, ok := env[proxyEnvVar]; ok {
			return fmt.Sprintf("%v (%v)", p.ProxyURL, proxyEnvVar)
		}
	}
	return p.ProxyURL
}

func (p *profileInfo) getClusterLine(isActive bool, env map[string]string) string {
	// indicate if active profile cluster is shadowed by env vars.
	if isActive {
		if _, ok := env[clusterEnvVar]; ok {
			return fmt.Sprintf("%v (%v)", p.Cluster, clusterEnvVar)
		} else if _, ok := env[siteEnvVar]; ok {
			return fmt.Sprintf("%v (%v)", p.Cluster, siteEnvVar)
		}
	}
	return p.Cluster
}

func (p *profileInfo) getKubeClusterLine(isActive bool, env map[string]string, cluster string) string {
	// indicate if active profile kube cluster is shadowed by env vars.
	if isActive {
		// check if kube cluster env var is set and no cluster was selected by kube config
		if _, ok := env[kubeClusterEnvVar]; ok {
			return fmt.Sprintf("%v (%v)", p.KubernetesCluster, kubeClusterEnvVar)
		}
	}
	return p.KubernetesCluster
}

func serializeProfiles(profile *profileInfo, profiles []*profileInfo, env map[string]string, format string) (string, error) {
	profileData := struct {
		Active   *profileInfo      `json:"active,omitempty"`
		Profiles []*profileInfo    `json:"profiles"`
		Env      map[string]string `json:"environment,omitempty"`
	}{profile, []*profileInfo{}, env}
	profileData.Profiles = append(profileData.Profiles, profiles...)
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(profileData, "", "  ")
	} else {
		out, err = yaml.Marshal(profileData)
	}
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(out), nil
}

func getTshEnv() map[string]string {
	env := map[string]string{}
	for _, envVar := range tshStatusEnvVars {
		if envVal, isSet := os.LookupEnv(envVar); isSet {
			env[envVar] = envVal
		}
	}
	return env
}

func printProfiles(debug bool, profile *profileInfo, profiles []*profileInfo, env map[string]string, verbose bool) {
	if profile == nil && len(profiles) == 0 {
		return
	}

	// Print the active profile.
	if profile != nil {
		printStatus(debug, profile, env, true)
	}

	// Print all other profiles.
	for _, p := range profiles {
		printStatus(debug, p, env, false)
	}

	// Print relevant active env vars, if they are set.
	if verbose {
		if len(env) > 0 {
			fmt.Println("Active Environment:")
		}
		for k, v := range env {
			fmt.Printf("\t%s=%s\n", k, v)
		}
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

// accessRequestWatcher is a helper to wait for an access request to be resolved.
type accessRequestWatcher struct {
	req     types.AccessRequest
	watcher types.Watcher
	closers []io.Closer
	sync.RWMutex
}

// newAccessRequestWatcher returns a new accessRequestWatcher. Callers should
// always defer (*accessRequestWatcher).Close().
func newAccessRequestWatcher(req types.AccessRequest) *accessRequestWatcher {
	return &accessRequestWatcher{
		req: req,
	}
}

// initialize sets up the underlying event watcher, when this returns without
// error the watcher is guaranteed to be in a ready state. Call this before
// creating the request to prevent a race.
func (w *accessRequestWatcher) initialize(ctx context.Context, tc *client.TeleportClient) error {
	w.Lock()
	defer w.Unlock()

	if w.watcher != nil {
		return trace.BadParameter("cannot re-initialize accessRequestWatcher")
	}

	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	w.closers = append(w.closers, proxyClient)

	rootClient, err := proxyClient.ConnectToRootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	w.closers = append(w.closers, rootClient)

	filter := types.AccessRequestFilter{
		User: w.req.GetUser(),
		ID:   w.req.GetName(),
	}
	w.watcher, err = rootClient.NewWatcher(ctx, types.Watch{
		Name: "await-request-approval",
		Kinds: []types.WatchKind{{
			Kind:   types.KindAccessRequest,
			Filter: filter.IntoMap(),
		}},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	w.closers = append(w.closers, w.watcher)

	// Wait for OpInit event so that returned watcher is ready.
	select {
	case event := <-w.watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("failed to watch for access requests: received an unexpected event while waiting for the initial OpInit")
		}
	case <-w.watcher.Done():
		return trace.Wrap(w.watcher.Error())
	case <-ctx.Done():
		// This should be the same as w.watcher.Done(), including for completeness.
		return trace.Wrap(ctx.Err())
	}

	return nil
}

// awaitResolution waits for the request to be resolved (state != PENDING).
func (w *accessRequestWatcher) awaitResolution() (types.AccessRequest, error) {
	w.RLock()
	defer w.RUnlock()

	if w.watcher == nil {
		return nil, trace.BadParameter("must initialize accessRequestWatcher before calling awaitResolution()")
	}

	for {
		select {
		case event := <-w.watcher.Events():
			switch event.Type {
			case types.OpPut:
				r, ok := event.Resource.(*types.AccessRequestV3)
				if !ok {
					return nil, trace.BadParameter("unexpected resource type %T", event.Resource)
				}
				if !r.GetState().IsPending() {
					return r, nil
				}
			case types.OpDelete:
				return nil, trace.Errorf("request %s has expired or been deleted...", event.Resource.GetName())
			default:
				log.Warnf("Skipping unknown event type %s", event.Type)
			}
		case <-w.watcher.Done():
			return nil, trace.Wrap(w.watcher.Error())
		}
	}
}

// Close closes the clients held by the watcher.
func (w *accessRequestWatcher) Close() error {
	var errs []error
	// Close in reverse order, like defer.
	w.RLock()
	for i := len(w.closers) - 1; i >= 0; i-- {
		errs = append(errs, w.closers[i].Close())
	}
	w.RUnlock()

	// Closed the watcher above, awaitResolution should now terminate and we can
	// grab the lock.
	w.Lock()
	w.closers = nil
	w.Unlock()

	return trace.NewAggregate(errs...)
}

func onRequestResolution(cf *CLIConf, tc *client.TeleportClient, req types.AccessRequest) error {
	if !req.GetState().IsApproved() {
		msg := fmt.Sprintf("request %s has been set to %s", req.GetName(), req.GetState().String())
		if reason := req.GetResolveReason(); reason != "" {
			msg = fmt.Sprintf("%s, reason=%q", msg, reason)
		}
		return trace.Errorf(msg)
	}

	msg := "\nApproval received, getting updated certificates...\n\n"
	if reason := req.GetResolveReason(); reason != "" {
		msg = fmt.Sprintf("\nApproval received, reason=%q\nGetting updated certificates...\n\n", reason)
	}
	fmt.Fprint(os.Stderr, msg)

	err := reissueWithRequests(cf, tc, req.GetName())
	return trace.Wrap(err)
}

// reissueWithRequests handles a certificate reissue, applying new requests by ID,
// and saving the updated profile.
func reissueWithRequests(cf *CLIConf, tc *client.TeleportClient, reqIDs ...string) error {
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}
	if profile.IsVirtual {
		return trace.BadParameter("cannot reissue certificates while using an identity file (-i)")
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
	if err := tc.ReissueUserCerts(cf.Context, client.CertCacheDrop, params); err != nil {
		return trace.Wrap(err)
	}
	if err := tc.SaveProfile(cf.HomePath, true); err != nil {
		return trace.Wrap(err)
	}
	if err := updateKubeConfig(cf, tc, ""); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func onApps(cf *CLIConf) error {
	if cf.ListAll {
		return trace.Wrap(listAppsAllClusters(cf))
	}
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get a list of all applications.
	var apps []types.Application
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		apps, err = tc.ListApps(cf.Context, nil /* custom filter */)
		return err
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Retrieve profile to be able to show which apps user is logged into.
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}

	// Sort by app name.
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].GetName() < apps[j].GetName()
	})

	return trace.Wrap(showApps(apps, profile.Apps, cf.Format, cf.Verbose))
}

type appListing struct {
	Proxy   string            `json:"proxy"`
	Cluster string            `json:"cluster"`
	App     types.Application `json:"app"`
}

type appListings []appListing

func (l appListings) Len() int {
	return len(l)
}

func (l appListings) Less(i, j int) bool {
	if l[i].Proxy != l[j].Proxy {
		return l[i].Proxy < l[j].Proxy
	}
	if l[i].Cluster != l[j].Cluster {
		return l[i].Cluster < l[j].Cluster
	}
	return l[i].App.GetName() < l[j].App.GetName()
}

func (l appListings) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func listAppsAllClusters(cf *CLIConf) error {
	var listings appListings
	err := forEachProfile(cf, func(tc *client.TeleportClient, profile *client.ProfileStatus) error {
		result, err := tc.ListAppsAllClusters(cf.Context, nil /* custom filter */)
		if err != nil {
			return trace.Wrap(err)
		}
		for clusterName, apps := range result {
			for _, app := range apps {
				listings = append(listings, appListing{
					Proxy:   profile.ProxyURL.Host,
					Cluster: clusterName,
					App:     app,
				})
			}
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	sort.Sort(listings)

	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}
	var active []tlsca.RouteToApp
	if profile != nil {
		active = profile.Apps
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		printAppsWithClusters(listings, active, cf.Verbose)
	case teleport.JSON, teleport.YAML:
		out, err := serializeAppsWithClusters(listings, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}
	return nil
}

func printAppsWithClusters(apps []appListing, active []tlsca.RouteToApp, verbose bool) {
	var rows [][]string
	for _, app := range apps {
		rows = append(rows, getAppRow(app.Proxy, app.Cluster, app.App, active, verbose))
	}
	// In verbose mode, print everything on a single line and include host UUID.
	// In normal mode, chunk the labels, print two per line and allow multiple
	// lines per node.
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable([]string{"Proxy", "Cluster", "Application", "Description", "Public Address", "URI", "Labels"}, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(
			[]string{"Proxy", "Cluster", "Application", "Description", "Public Address", "Labels"}, rows, "Labels")
	}
	fmt.Println(t.AsBuffer().String())
}

func serializeAppsWithClusters(apps []appListing, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(apps, "", "  ")
	} else {
		out, err = yaml.Marshal(apps)
	}
	return string(out), trace.Wrap(err)
}

// onEnvironment handles "tsh env" command.
func onEnvironment(cf *CLIConf) error {
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return trace.Wrap(err)
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		// Print shell built-in commands to set (or unset) environment.
		switch {
		case cf.unsetEnvironment:
			fmt.Printf("unset %v\n", proxyEnvVar)
			fmt.Printf("unset %v\n", clusterEnvVar)
			fmt.Printf("unset %v\n", kubeClusterEnvVar)
			fmt.Printf("unset %v\n", teleport.EnvKubeConfig)
		case !cf.unsetEnvironment:
			kubeName := selectedKubeCluster(profile.Cluster)
			fmt.Printf("export %v=%v\n", proxyEnvVar, profile.ProxyURL.Host)
			fmt.Printf("export %v=%v\n", clusterEnvVar, profile.Cluster)
			if kubeName != "" {
				fmt.Printf("export %v=%v\n", kubeClusterEnvVar, kubeName)
				fmt.Printf("# set %v to a standalone kubeconfig for the selected kube cluster\n", teleport.EnvKubeConfig)
				fmt.Printf("export %v=%v\n", teleport.EnvKubeConfig, profile.KubeConfigPath(kubeName))
			}
		}
	case teleport.JSON, teleport.YAML:
		out, err := serializeEnvironment(profile, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	}

	return nil
}

func serializeEnvironment(profile *client.ProfileStatus, format string) (string, error) {
	env := map[string]string{
		proxyEnvVar:   profile.ProxyURL.Host,
		clusterEnvVar: profile.Cluster,
	}
	kubeName := selectedKubeCluster(profile.Cluster)
	if kubeName != "" {
		env[kubeClusterEnvVar] = kubeName
		env[teleport.EnvKubeConfig] = profile.KubeConfigPath(kubeName)
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(env, "", "  ")
	} else {
		out, err = yaml.Marshal(env)
	}
	return string(out), trace.Wrap(err)
}

// envGetter is used to read in the environment. In production "os.Getenv"
// is used.
type envGetter func(string) string

// setEnvFlags sets flags that can be set via environment variables.
func setEnvFlags(cf *CLIConf, fn envGetter) {
	// prioritize CLI input
	if cf.SiteName == "" {
		setSiteNameFromEnv(cf, fn)
	}
	// prioritize CLI input
	if cf.KubernetesCluster == "" {
		setKubernetesClusterFromEnv(cf, fn)
	}

	// these can only be set with env vars.
	setTeleportHomeFromEnv(cf, fn)
	setGlobalTshConfigPathFromEnv(cf, fn)
}

// setSiteNameFromEnv sets teleport site name from environment if configured.
// First try reading TELEPORT_CLUSTER, then the legacy term TELEPORT_SITE.
func setSiteNameFromEnv(cf *CLIConf, fn envGetter) {
	if clusterName := fn(siteEnvVar); clusterName != "" {
		cf.SiteName = clusterName
	}
	if clusterName := fn(clusterEnvVar); clusterName != "" {
		cf.SiteName = clusterName
	}
}

// setTeleportHomeFromEnv sets home directory from environment if configured.
func setTeleportHomeFromEnv(cf *CLIConf, fn envGetter) {
	if homeDir := fn(types.HomeEnvVar); homeDir != "" {
		cf.HomePath = path.Clean(homeDir)
	}
}

// setKubernetesClusterFromEnv sets teleport kube cluster from environment if configured.
func setKubernetesClusterFromEnv(cf *CLIConf, fn envGetter) {
	if kubeName := fn(kubeClusterEnvVar); kubeName != "" {
		cf.KubernetesCluster = kubeName
	}
}

// setGlobalTshConfigPathFromEnv sets path to global tsh config file.
func setGlobalTshConfigPathFromEnv(cf *CLIConf, fn envGetter) {
	if configPath := fn(globalTshConfigEnvVar); configPath != "" {
		cf.GlobalTshConfigPath = path.Clean(configPath)
	}
}

func handleUnimplementedError(ctx context.Context, perr error, cf CLIConf) error {
	const (
		errMsgFormat         = "This server does not implement this feature yet. Likely the client version you are using is newer than the server. The server version: %v, the client version: %v. Please upgrade the server."
		unknownServerVersion = "unknown"
	)
	tc, err := makeClient(&cf, false)
	if err != nil {
		log.WithError(err).Warning("Failed to create client.")
		return trace.WrapWithMessage(perr, errMsgFormat, unknownServerVersion, teleport.Version)
	}
	pr, err := tc.Ping(ctx)
	if err != nil {
		log.WithError(err).Warning("Failed to call ping.")
		return trace.WrapWithMessage(perr, errMsgFormat, unknownServerVersion, teleport.Version)
	}
	return trace.WrapWithMessage(perr, errMsgFormat, pr.ServerVersion, teleport.Version)
}

func validateParticipantMode(mode types.SessionParticipantMode) error {
	switch mode {
	case types.SessionPeerMode, types.SessionObserverMode, types.SessionModeratorMode:
		return nil
	default:
		return trace.BadParameter("invalid participant mode %v", mode)
	}
}

// forEachProfile performs an action for each profile a user is currently logged in to.
func forEachProfile(cf *CLIConf, fn func(tc *client.TeleportClient, profile *client.ProfileStatus) error) error {
	profile, profiles, err := client.Status(cf.HomePath, "")
	if err != nil {
		return trace.Wrap(err)
	}
	if profile != nil {
		profiles = append(profiles, profile)
	}

	clock := clockwork.NewRealClock()
	errors := make([]error, 0)
	for _, p := range profiles {
		proxyAddr := p.ProxyURL.Host
		if p.IsExpired(clock) {
			fmt.Fprintf(os.Stderr, "Credentials expired for proxy %q, skipping...\n", proxyAddr)
			continue
		}
		tc, err := makeClientForProxy(cf, proxyAddr, true)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if err := fn(tc, p); err != nil {
			errors = append(errors, err)
		}
	}

	return trace.NewAggregate(errors...)
}
