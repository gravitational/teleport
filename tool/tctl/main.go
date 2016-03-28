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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/buger/goterm"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
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
	// count is optional hidden field that will cause
	// tctl issue count tokens and output them in JSON format
	count int
	// format is the output format, e.g. text or json
	format string
}

type AuthCommand struct {
	config                     *service.Config
	authType                   string
	genPubPath                 string
	genPrivPath                string
	genSigningKeyPath          string
	genRole                    teleport.Role
	genAuthorityDomain         string
	exportAuthorityFingerprint string
	exportPrivateKeys          bool
}

type AuthServerCommand struct {
	config *service.Config
}

type ReverseTunnelCommand struct {
	config      *service.Config
	domainNames string
	dialAddrs   utils.NetAddrList
	ttl         time.Duration
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
	cmdReverseTunnel := ReverseTunnelCommand{config: cfg}

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
	nodeAdd.Flag("count", "add count tokens and output JSON with the list").Hidden().Default("1").IntVar(&cmdNodes.count)
	nodeAdd.Flag("format", "output format, 'text' or 'json'").Hidden().Default("text").StringVar(&cmdNodes.format)
	nodeAdd.Alias(AddNodeHelp)
	nodeList := nodes.Command("ls", "Lists all active SSH nodes within the cluster")
	nodeList.Alias(ListNodesHelp)

	// operations with authorities
	auth := app.Command("authorities", "Operations with user and host certificate authorities").Hidden()
	auth.Flag("type", "authority type, 'user' or 'host'").Default(string(services.UserCA)).StringVar(&cmdAuth.authType)
	authList := auth.Command("ls", "List trusted user certificate authorities").Hidden()
	authExport := auth.Command("export", "Export concatenated keys to standard output").Hidden()
	authExport.Flag("private-keys", "if set, will print private keys").BoolVar(&cmdAuth.exportPrivateKeys)
	authExport.Flag("fingerprint", "filter authority by fingerprint").StringVar(&cmdAuth.exportAuthorityFingerprint)

	authGenerate := auth.Command("gen", "Generate new OpenSSH keypair").Hidden()
	authGenerate.Flag("pub-key", "path to the public key to write").Required().StringVar(&cmdAuth.genPubPath)
	authGenerate.Flag("priv-key", "path to the private key to write").Required().StringVar(&cmdAuth.genPrivPath)

	authGenAndSign := auth.Command("gencert", "Generate OpenSSH keys and certificate for a joining teleport proxy, node or auth server").Hidden()
	authGenAndSign.Flag("priv-key", "path to the private key to write").Required().StringVar(&cmdAuth.genPrivPath)
	authGenAndSign.Flag("cert", "path to the public signed cert to write").Required().StringVar(&cmdAuth.genPubPath)
	authGenAndSign.Flag("sign-key", "path to the private OpenSSH signing key").Required().StringVar(&cmdAuth.genSigningKeyPath)
	authGenAndSign.Flag("role", "server role, e.g. 'proxy', 'auth' or 'node'").Required().SetValue(&cmdAuth.genRole)
	authGenAndSign.Flag("domain", "cluster certificate authority domain name").Required().StringVar(&cmdAuth.genAuthorityDomain)

	// operations with auth servers
	authServers := app.Command("authservers", "Operations with user and host certificate authorities").Hidden()
	authServerAdd := authServers.Command("add", "Add a new auth server node to the cluster").Hidden()

	// operations with reverse tunnels
	reverseTunnels := app.Command("rts", "Operations with reverse tunnels").Hidden()
	reverseTunnelsList := reverseTunnels.Command("ls", "List reverse tunnels").Hidden()
	reverseTunnelsDelete := reverseTunnels.Command("del", "Deletes reverse tunnels").Hidden()
	reverseTunnelsDelete.Arg("domain", "Comma-separated list of reverse tunnels to delete").
		Required().StringVar(&cmdReverseTunnel.domainNames)
	reverseTunnelsUpsert := reverseTunnels.Command("upsert", "Update or add a new reverse tunnel").Hidden()
	reverseTunnelsUpsert.Arg("domain", "Domain name of the reverse tunnel").
		Required().StringVar(&cmdReverseTunnel.domainNames)
	reverseTunnelsUpsert.Arg("addrs", "Comma-separated list of dial addresses for reverse tunnels to dial").
		Required().SetValue(&cmdReverseTunnel.dialAddrs)
	reverseTunnelsUpsert.Flag("ttl", "Optional TTL (time to live) for reverse tunnel").DurationVar(&cmdReverseTunnel.ttl)

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

	// some commands do not need a connection to client
	switch command {
	case authGenerate.FullCommand():
		err = cmdAuth.GenerateKeys()
		if err != nil {
			utils.FatalError(err)
		}
		return
	case authGenAndSign.FullCommand():
		err = cmdAuth.GenerateAndSignKeys()
		if err != nil {
			utils.FatalError(err)
		}
		return
	}

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
	case reverseTunnelsList.FullCommand():
		err = cmdReverseTunnel.ListActive(client)
	case reverseTunnelsDelete.FullCommand():
		err = cmdReverseTunnel.Delete(client)
	case reverseTunnelsUpsert.FullCommand():
		err = cmdReverseTunnel.Upsert(client)
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
	fmt.Printf("Signup token has been created and is valid for %v seconds. Share this URL with the user:\n%v\n\nNOTE: make sure the hostname is accessible!\n", defaults.MaxSignupTokenTTL.Seconds(), url)
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
	if u.count < 1 {
		return trace.Wrap(teleport.BadParameter("count", fmt.Sprintf("count should be > 0, got %v", u.count)))
	}
	var tokens []string
	for i := 0; i < u.count; i++ {
		token, err := client.GenerateToken(teleport.RoleNode, defaults.MaxProvisioningTokenTTL)
		if err != nil {
			return trace.Wrap(err)
		}
		tokens = append(tokens, token)
	}

	if u.format == "text" {
		for _, token := range tokens {
			fmt.Printf(
				"The invite token: %v\nRun this on the new node to join the cluster:\n> teleport start --roles=node --token=%v --auth-server=<Address>\n\nNotes:\n",
				token, token)
		}
		fmt.Printf("  1. This invitation token will expire in %v seconds.\n", defaults.MaxProvisioningTokenTTL.Seconds())
		fmt.Printf("  2. <Address> is the IP this auth server is reachable at from the node.\n")
	} else {
		out, err := json.Marshal(tokens)
		if err != nil {
			return trace.Wrap(err, "failed to marshal tokens")
		}
		fmt.Printf(string(out))
	}

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
		printHeader(t, []string{"Node Name", "Node ID", "Address", "Labels"})
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
	authorities, err := client.GetCertAuthorities(authType, false)
	if err != nil {
		return trace.Wrap(err)
	}
	view := func() string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Type", "Authority Domain", "Fingerprint", "Restricted to logins"})
		if len(authorities) == 0 {
			return t.String()
		}
		for _, a := range authorities {
			for _, keyBytes := range a.CheckingKeys {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
				if err != nil {
					fingerprint = fmt.Sprintf("<bad key: %v", err)
				}
				fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", a.Type, a.DomainName, fingerprint, strings.Join(a.AllowedLogins, ","))
			}
		}
		return t.String()
	}
	fmt.Printf(view())
	return nil
}

// ExportAuthorities outputs the list of authorities in OpenSSH compatible formats
func (a *AuthCommand) ExportAuthorities(client *auth.TunClient) error {
	authType := services.CertAuthType(a.authType)
	if err := authType.Check(); err != nil {
		return trace.Wrap(err)
	}
	authorities, err := client.GetCertAuthorities(authType, a.exportPrivateKeys)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, ca := range authorities {
		if a.exportPrivateKeys {
			for _, key := range ca.SigningKeys {
				fingerprint, err := sshutils.PrivateKeyFingerprint(key)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}
				os.Stdout.Write(key)
				fmt.Fprintf(os.Stdout, "\n")
			}
		} else {
			for _, keyBytes := range ca.CheckingKeys {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}
				if authType == services.UserCA {
					// for user authorities, export in the sshd's TrustedUserCAKeys format
					os.Stdout.Write(keyBytes)
				} else {
					// for host authorities export them in the authorized_keys - compatible format
					fmt.Fprintf(os.Stdout, "@cert-authority *.%v %v", ca.DomainName, string(keyBytes))
				}
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
	token, err := client.GenerateToken(teleport.RoleAuth, defaults.InviteTokenTTL)
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

// GenerateKeys generates a new keypair
func (a *AuthCommand) GenerateKeys() error {
	privBytes, pubBytes, err := native.New().GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}
	err = ioutil.WriteFile(a.genPubPath, pubBytes, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(a.genPrivPath, privBytes, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("wrote public key to: %v and private key to: %v\n", a.genPubPath, a.genPrivPath)
	return nil
}

// GenerateAndSignKeys generates a new keypair and signs it for role
func (a *AuthCommand) GenerateAndSignKeys() error {
	privSigningKeyBytes, err := ioutil.ReadFile(a.genSigningKeyPath)
	if err != nil {
		return trace.Wrap(err)
	}
	ca := native.New()
	privBytes, pubBytes, err := ca.GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}
	nodeID := uuid.New()
	certBytes, err := ca.GenerateHostCert(privSigningKeyBytes, pubBytes, nodeID, a.genAuthorityDomain, a.genRole, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ioutil.WriteFile(a.genPubPath, certBytes, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(a.genPrivPath, privBytes, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("wrote signed certificate to: %v and private key to: %v\n", a.genPubPath, a.genPrivPath)
	return nil
}

// ListActive retreives the list of nodes who recently sent heartbeats to
// to a cluster and prints it to stdout
func (r *ReverseTunnelCommand) ListActive(client *auth.TunClient) error {
	tunnels, err := client.GetReverseTunnels()
	if err != nil {
		return trace.Wrap(err)
	}
	tunnelsView := func() string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Domain", "Dial Addresses"})
		if len(tunnels) == 0 {
			return t.String()
		}
		for _, tunnel := range tunnels {
			fmt.Fprintf(t, "%v\t%v\n", tunnel.DomainName, strings.Join(tunnel.DialAddrs, ","))
		}
		return t.String()
	}
	fmt.Printf(tunnelsView())
	return nil
}

// Upsert updates or inserts new reverse tunnel
func (r *ReverseTunnelCommand) Upsert(client *auth.TunClient) error {
	err := client.UpsertReverseTunnel(services.ReverseTunnel{
		DomainName: r.domainNames,
		DialAddrs:  r.dialAddrs.Addresses()},
		r.ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Reverse tunnel updated\n")
	return nil
}

// Delete deletes teleport user(s). User IDs are passed as a comma-separated
// list in UserCommand.login
func (r *ReverseTunnelCommand) Delete(client *auth.TunClient) error {
	for _, domainName := range strings.Split(r.domainNames, ",") {
		if err := client.DeleteReverseTunnel(domainName); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Reverse tunnel '%v' has been deleted\n", domainName)
	}
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
		cfg.AuthServers,
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
