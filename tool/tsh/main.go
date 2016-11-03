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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/buger/goterm"
	"github.com/pborman/uuid"
)

func main() {
	run(os.Args[1:], false)
}

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
	// ExternalAuth is used to authenticate using external OIDC method
	ExternalAuth string
	// SiteName specifies remote site go login to
	SiteName string
	// Interactive, when set to true, launches remote command with the terminal attached
	Interactive bool
}

// run executes TSH client. same as main() but easier to test
func run(args []string, underTest bool) {
	var (
		cf CLIConf
	)
	cf.IsUnderTest = underTest
	utils.InitLoggerCLI()

	// configure CLI argument parser:
	app := utils.InitCLIParser("tsh", "TSH: Teleport SSH client").Interspersed(false)
	app.Flag("login", "Remote host login").Short('l').Envar("TELEPORT_LOGIN").StringVar(&cf.NodeLogin)
	app.Flag("user", fmt.Sprintf("SSH proxy user [%s]", client.Username())).Envar("TELEPORT_USER").StringVar(&cf.Username)
	app.Flag("auth", "[EXPERIMENTAL] Use external authentication, e.g. 'google'").Envar("TELEPORT_AUTH").Hidden().StringVar(&cf.ExternalAuth)
	app.Flag("cluster", "Specify the cluster to connect").Envar("TELEPORT_SITE").StringVar(&cf.SiteName)
	app.Flag("proxy", "SSH proxy host or IP address").Envar("TELEPORT_PROXY").StringVar(&cf.Proxy)
	app.Flag("ttl", "Minutes to live for a SSH session").Int32Var(&cf.MinsToLive)
	app.Flag("insecure", "Do not verify server's certificate and host name. Use only in test environments").Default("false").BoolVar(&cf.InsecureSkipVerify)
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
	// ls
	ls := app.Command("ls", "List remote SSH nodes")
	ls.Arg("labels", "List of labels to filter node list").StringVar(&cf.UserHost)
	// clusters
	clusters := app.Command("clusters", "List available Teleport clusters")
	// agent (SSH agent listening on unix socket)
	agent := app.Command("agent", "Start SSH agent on unix socket")
	agent.Flag("socket", "SSH agent listening socket address, e.g. unix:///tmp/teleport.agent.sock").SetValue(&cf.AgentSocketAddr)

	// login logs in with remote proxy and obtains a "session certificate" which gets
	// stored in ~/.tsh directory
	login := app.Command("login", "Log in to the cluster and store the session certificate to avoid login prompts")

	// logout deletes obtained session certificates in ~/.tsh
	logout := app.Command("logout", "Delete a cluster certificate")

	// parse CLI commands+flags:
	command, err := app.Parse(args)
	if err != nil {
		utils.FatalError(err)
	}

	// apply -d flag:
	if *debugMode {
		utils.InitLoggerDebug()
	}

	switch command {
	case ver.FullCommand():
		onVersion()
	case ssh.FullCommand():
		onSSH(&cf)
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
		onLogin(&cf)
	case logout.FullCommand():
		onLogout(&cf)
	}
}

// onPlay replays a session with a given ID
func onPlay(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	if err := tc.Play(cf.SessionID); err != nil {
		utils.FatalError(err)
	}
}

// onLogin logs in with remote proxy and gets signed certificates
func onLogin(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	if err := tc.Login(); err != nil {
		utils.FatalError(err)
	}
	if tc.SiteName != "" {
		fmt.Printf("\nYou are now logged into %s as %s\n", tc.SiteName, tc.Username)
	} else {
		fmt.Printf("\nYou are now logged in\n")
	}
}

// onLogout deletes a "session certificate" from ~/.tsh for a given proxy
func onLogout(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	if err = tc.Logout(); err != nil {
		if trace.IsNotFound(err) {
			fmt.Println("You are not logged in")
			return
		}
		utils.FatalError(err)
	}
	fmt.Printf("%s has logged out\n", tc.Username)
}

// onListNodes executes 'tsh ls' command
func onListNodes(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	servers, err := tc.ListNodes()
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
			fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", n.Hostname, n.ID, n.Addr, n.LabelsString())
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
	fmt.Printf(sitesView())
}

// onSSH executes 'tsh ssh' command
func onSSH(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}

	tc.Stdin = os.Stdin
	if err = tc.SSH(cf.RemoteCommand, cf.LocalExec); err != nil {
		// exit with the same exit status as the failed command:
		if tc.ExitStatus != 0 {
			os.Exit(tc.ExitStatus)
		} else {
			utils.FatalError(err)
		}
	} else {
		// successful session? update the profile then:
		tc.SaveProfile("")
	}
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
	if err = tc.Join(*sid, nil); err != nil {
		utils.FatalError(err)
	} else {
		// successful session? update the profile then:
		tc.SaveProfile("")
	}
}

// onSCP executes 'tsh scp' command
func onSCP(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}
	if err := tc.SCP(cf.CopySpec, int(cf.NodePort), cf.RecursiveCopy); err != nil {
		// exit with the same exit status as the failed command:
		if tc.ExitStatus != 0 {
			os.Exit(tc.ExitStatus)
		} else {
			utils.FatalError(err)
		}
	} else {
		// successful session? update the profile then:
		tc.SaveProfile("")
	}
}

// onAgentStart start ssh agent on a socket
func onAgentStart(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	socketAddr := utils.NetAddr(cf.AgentSocketAddr)
	if socketAddr.IsEmpty() {
		socketAddr = utils.NetAddr{AddrNetwork: "unix", Addr: filepath.Join(os.TempDir(), fmt.Sprintf("%v.socket", uuid.New()))}
	}
	// This makes teleport agent behave exactly like ssh-agent command,
	// the output and behavior matches the openssh behavior,
	// so users can do 'eval $(tsh agent --proxy=<addr>&)
	pid := os.Getpid()
	fmt.Printf(`
SSH_AUTH_SOCK=%v; export SSH_AUTH_SOCK;
SSH_AGENT_PID=%v; export SSH_AGENT_PID;
echo Agent pid %v;
`, socketAddr.Addr, pid, pid)
	agentServer := teleagent.NewServer()
	agentKeys, err := tc.GetKeys()
	if err != nil {
		utils.FatalError(err)
	}
	// add existing keys to the running agent for ux purposes
	for _, key := range agentKeys {
		err := agentServer.Add(key)
		if err != nil {
			utils.FatalError(err)
		}
	}
	if err := agentServer.ListenAndServe(socketAddr); err != nil {
		utils.FatalError(err)
	}
}

// makeClient takes the command-line configuration and constructs & returns
// a fully configured TeleportClient object
func makeClient(cf *CLIConf, useProfileLogin bool) (tc *client.TeleportClient, err error) {
	// apply defults
	if cf.NodePort == 0 {
		cf.NodePort = defaults.SSHServerListenPort
	}
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

	// 2: override with `./tsh` profiles
	if err = c.LoadProfile(""); err != nil {
		fmt.Printf("WARNING: Failed loading tsh profile.\n%v\n", err)
	}
	// 3: override with the CLI flags
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
	c.ConnectorID = cf.ExternalAuth
	c.Interactive = cf.Interactive
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
