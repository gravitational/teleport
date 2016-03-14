/*
Copyright 2015 Gravitational, Inc.

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
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/buger/goterm"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

type CLIConfig struct {
	Debug bool
}

type UserCommand struct {
	config        *service.Config
	login         string
	allowedLogins string
}

type NodeCommand struct {
	config *service.Config
}

type AuthCommand struct {
	config   *service.Config
	authType string
}

type AuthServerCommand struct {
	config *service.Config
}

func main() {
	utils.InitLoggerCLI()
	app := utils.InitCLIParser("tctl", GlobalHelpString)

	// generate default tctl configuration:
	cfg := service.MakeDefaultConfig()
	cmdUsers := UserCommand{config: cfg}
	cmdNodes := NodeCommand{config: cfg}
	cmdAuth := AuthCommand{config: cfg}
	cmdAuthServers := AuthServerCommand{config: cfg}

	// define global flags:
	var ccf CLIConfig
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)

	// commands:
	ver := app.Command("version", "Print the version.")
	app.HelpFlag.Short('h')

	// user add command:
	users := app.Command("users", "Manage users logins")
	userAdd := users.Command("add", "Generates an invitation token and prints the signup URL for setting up 2nd factor auth.")
	userAdd.Arg("login", "Teleport user login").Required().StringVar(&cmdUsers.login)
	userAdd.Arg("local-logins", "Local UNIX users this account can log in as [login]").
		Default("").StringVar(&cmdUsers.allowedLogins)
	userAdd.Alias(AddUserHelp)

	// list users command
	userList := users.Command("ls", "Lists all user accounts")

	// delete user command
	userDelete := users.Command("del", "Deletes user accounts")
	userDelete.Arg("logins", "Comma-separated list of user logins to delete").
		Required().StringVar(&cmdUsers.login)

	// add node command
	nodes := app.Command("nodes", "Issue invites for other nodes to join the cluster")
	nodeAdd := nodes.Command("add", "Adds a new SSH node to join the cluster")
	nodeAdd.Alias(AddNodeHelp)
	nodeList := nodes.Command("ls", "Lists all active SSH nodes within the cluster")
	nodeList.Alias(ListNodesHelp)

	// operations with authorities
	auth := app.Command("authorities", "Operations with user and host certificate authorities").Hidden()
	auth.Flag("type", "authority type, 'user' or 'host'").Default(string(services.UserCA)).StringVar(&cmdAuth.authType)
	authList := auth.Command("ls", "List trusted user certificate authorities").Hidden()
	authExport := auth.Command("export", "Export concatenated keys to standard output").Hidden()

	// operations with auth servers
	authServers := app.Command("authservers", "Operations with user and host certificate authorities").Hidden()
	authServerAdd := authServers.Command("add", "Add a new auth server node to the cluster").Hidden()

	// parse CLI commands+flags:
	command, err := app.Parse(os.Args[1:])
	if err != nil {
		utils.FatalError(err)
	}

	// --debug flag
	if ccf.Debug {
		utils.InitLoggerDebug()
	}

	validateConfig(cfg)

	// connect to the teleport auth service:
	client, err := connectToAuthService(cfg)
	if err != nil {
		utils.FatalError(err)
	}

	// execute the selected command:
	switch command {
	case ver.FullCommand():
		onVersion()
	case userAdd.FullCommand():
		err = cmdUsers.Add(cfg.Hostname, client)
	case userList.FullCommand():
		err = cmdUsers.List(client)
	case userDelete.FullCommand():
		err = cmdUsers.Delete(client)
	case nodeAdd.FullCommand():
		err = cmdNodes.Invite(client)
	case nodeList.FullCommand():
		err = cmdNodes.ListActive(client)
	case authList.FullCommand():
		err = cmdAuth.ListAuthorities(client)
	case authExport.FullCommand():
		err = cmdAuth.ExportAuthorities(client)
	case authServerAdd.FullCommand():
		err = cmdAuthServers.Invite(client)
	}

	if err != nil {
		utils.FatalError(err)
	}
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

// Add creates a new sign-up token and prints a token URL to stdout.
// A user is not created until he visits the sign-up URL and completes the process
func (u *UserCommand) Add(hostname string, client *auth.TunClient) error {
	// if no local logins were specified, default to 'login'
	if u.allowedLogins == "" {
		u.allowedLogins = u.login
	}
	token, err := client.CreateSignupToken(u.login, strings.Split(u.allowedLogins, ","))
	if err != nil {
		return err
	}
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	url := web.CreateSignupLink(net.JoinHostPort(hostname, strconv.Itoa(defaults.HTTPListenPort)), token)
	fmt.Printf("Signup token has been created. Share this URL with the user:\n%v\n\nNOTE: make sure the hostname is accessible!\n", url)
	return nil
}

// List prints all existing user accounts
func (u *UserCommand) List(client *auth.TunClient) error {
	users, err := client.GetUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	usersView := func(users []services.User) string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"User", "Allowed to login as"})
		if len(users) == 0 {
			return t.String()
		}
		for _, u := range users {
			fmt.Fprintf(t, "%v\t%v\n", u.Name, strings.Join(u.AllowedLogins, ","))
		}
		return t.String()
	}
	fmt.Printf(usersView(users))
	return nil
}

// Delete deletes teleport user(s). User IDs are passed as a comma-separated
// list in UserCommand.login
func (u *UserCommand) Delete(client *auth.TunClient) error {
	for _, l := range strings.Split(u.login, ",") {
		if err := client.DeleteUser(l); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("User '%v' has been deleted\n", l)
	}
	return nil
}

// Invite generates a token which can be used to add another SSH node
// to a cluster
func (u *NodeCommand) Invite(client *auth.TunClient) error {
	invitationTTL := time.Minute * 15
	token, err := client.GenerateToken(teleport.RoleNode, invitationTTL)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		"The invite token: %v\nRun this on the new node to join the cluster:\n> teleport start --roles=node --token=%v --auth-server=<Address>\n\nNotes:\n",
		token, token)
	fmt.Printf("  1. This invitation token will expire in %v seconds.\n", invitationTTL.Seconds())
	fmt.Printf("  2. <Address> is the IP this auth server is reachable at from the node.\n")
	return nil
}

// ListActive retreives the list of nodes who recently sent heartbeats to
// to a cluster and prints it to stdout
func (u *NodeCommand) ListActive(client *auth.TunClient) error {
	nodes, err := client.GetNodes()
	if err != nil {
		return trace.Wrap(err)
	}
	nodesView := func(nodes []services.Server) string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Node Hostname", "Node ID", "Address", "Labels"})
		if len(nodes) == 0 {
			return t.String()
		}
		for _, n := range nodes {
			fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", n.Hostname, n.ID, n.Addr, n.LabelsString())
		}
		return t.String()
	}
	fmt.Printf(nodesView(nodes))
	return nil
}

// ListAuthorities shows list of user authorities we trust
func (a *AuthCommand) ListAuthorities(client *auth.TunClient) error {
	authType := services.CertAuthType(a.authType)
	if err := authType.Check(); err != nil {
		return trace.Wrap(err)
	}
	authorities, err := client.GetCertAuthorities(authType)
	if err != nil {
		return trace.Wrap(err)
	}
	view := func() string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Type", "Cluster name", "Fingerprint", "Restricted to logins"})
		if len(authorities) == 0 {
			return t.String()
		}
		for _, a := range authorities {
			for _, keyBytes := range a.CheckingKeys {
				fingerprint := ""
				key, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
				if err != nil {
					fingerprint = fmt.Sprintf("<bad key: %v", err)
				} else {
					fingerprint = sshutils.Fingerprint(key)
				}
				fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", a.Type, a.DomainName, fingerprint, strings.Join(a.AllowedLogins, ","))
			}
		}
		return t.String()
	}
	fmt.Printf(view())
	return nil
}

// ExportAuthorities outputs the list of authorities
func (a *AuthCommand) ExportAuthorities(client *auth.TunClient) error {
	authType := services.CertAuthType(a.authType)
	if err := authType.Check(); err != nil {
		return trace.Wrap(err)
	}
	authorities, err := client.GetCertAuthorities(authType)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, a := range authorities {
		for _, key := range a.CheckingKeys {
			if authType == services.UserCA {
				// for user authorities, export in the sshd's TrustedUserCAKeys format
				os.Stdout.Write(key)
			} else {
				// for host authorities export them in the authorized_keys - compatible format
				fmt.Fprintf(os.Stdout, "@cert-authority *.%v %v", a.DomainName, string(key))
			}
		}
	}
	return nil
}

// Invite generates a token which can be used to add another SSH auth server
// to the cluster
func (u *AuthServerCommand) Invite(client *auth.TunClient) error {
	authDomainName, err := client.GetLocalDomain()
	if err != nil {
		return trace.Wrap(err)
	}
	invitationTTL := time.Minute * 15
	token, err := client.GenerateToken(teleport.RoleAuth, invitationTTL)
	if err != nil {
		return trace.Wrap(err)
	}
	cfg := config.MakeAuthPeerFileConfig(authDomainName, token)
	out := cfg.DebugDumpToYAML()

	fmt.Printf(
		"# Run this config the new auth server to join the cluster:\n# > teleport start --config config.yaml\n# Fill in auth peers in this config:\n")
	fmt.Println(out)
	return nil
}

// connectToAuthService creates a valid client connection to the auth service
func connectToAuthService(cfg *service.Config) (client *auth.TunClient, err error) {
	// connect to the local auth server by default:
	cfg.Auth.Enabled = true
	cfg.AuthServers = []utils.NetAddr{
		*defaults.AuthConnectAddr(),
	}

	// read the host SSH keys and use them to open an SSH connection to the auth service
	i, err := auth.ReadIdentity(cfg.DataDir, auth.IdentityID{Role: teleport.RoleAdmin, HostUUID: cfg.HostUUID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err = auth.NewTunClient(
		cfg.AuthServers[0],
		cfg.HostUUID,
		[]ssh.AuthMethod{ssh.PublicKeys(i.KeySigner)})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check connectivity by calling something on a clinet:
	_, err = client.GetDialer()()
	if err != nil {
		utils.Consolef(os.Stderr,
			"Cannot connect to the auth server: %v.\nIs the auth server running on %v?", err, cfg.AuthServers[0].Addr)
		os.Exit(1)
	}
	return client, nil
}

// validateConfig updtes&validates tctl configuration
func validateConfig(cfg *service.Config) {
	var err error
	// read or generate a host UUID for this node
	cfg.HostUUID, err = utils.ReadOrMakeHostUUID(cfg.DataDir)
	if err != nil {
		utils.FatalError(err)
	}
}
