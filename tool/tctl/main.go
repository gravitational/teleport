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

	"github.com/Sirupsen/logrus"
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
	Debug      bool
	ConfigFile string
}

type UserCommand struct {
	config        *service.Config
	login         string
	allowedLogins string
	identities    []string
}

type NodeCommand struct {
	config *service.Config
	// count is optional hidden field that will cause
	// tctl issue count tokens and output them in JSON format
	count int
	// format is the output format, e.g. text or json
	format string
	// list of roles for the new node to assume
	roles string
	// TTL: duration of time during which a generated node token will
	// be valid.
	ttl time.Duration
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

type TokenCommand struct {
	config *service.Config
	// token argument to 'tokens del' command
	token string
}

func main() {
	utils.InitLoggerCLI()
	app := utils.InitCLIParser("tctl", GlobalHelpString)

	// generate default tctl configuration:
	cfg := service.MakeDefaultConfig()
	cmdUsers := UserCommand{config: cfg}
	cmdNodes := NodeCommand{config: cfg}
	cmdAuth := AuthCommand{config: cfg}
	cmdReverseTunnel := ReverseTunnelCommand{config: cfg}
	cmdTokens := TokenCommand{config: cfg}

	// define global flags:
	var ccf CLIConfig
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)
	app.Flag("config", fmt.Sprintf("Path to a configuration file [%v]", defaults.ConfigFilePath)).
		Short('c').
		ExistingFileVar(&ccf.ConfigFile)

	// commands:
	ver := app.Command("version", "Print the version.")
	app.HelpFlag.Short('h')

	// user add command:
	users := app.Command("users", "Manage users logins")
	userAdd := users.Command("add", "Generate an invitation token and print the signup URL")
	userAdd.Arg("login", "Teleport user login").Required().StringVar(&cmdUsers.login)
	userAdd.Arg("local-logins", "Local UNIX users this account can log in as [login]").
		Default("").StringVar(&cmdUsers.allowedLogins)
	userAdd.Flag("identity", "[EXPERIMENTAL] Add OpenID Connect identity, e.g. --identity=google:bob@gmail.com").Hidden().StringsVar(&cmdUsers.identities)
	userAdd.Alias(AddUserHelp)

	// list users command
	userList := users.Command("ls", "List all user accounts")

	// delete user command
	userDelete := users.Command("del", "Deletes user accounts")
	userDelete.Arg("logins", "Comma-separated list of user logins to delete").
		Required().StringVar(&cmdUsers.login)

	// add node command
	nodes := app.Command("nodes", "Issue invites for other nodes to join the cluster")
	nodeAdd := nodes.Command("add", "Generate an invitation token. Use it to add a new node to the Teleport cluster")
	nodeAdd.Flag("roles", "Comma-separated list of roles for the new node to assume [node]").Default("node").StringVar(&cmdNodes.roles)
	nodeAdd.Flag("ttl", "Time to live for a generated token").DurationVar(&cmdNodes.ttl)
	nodeAdd.Flag("count", "add count tokens and output JSON with the list").Hidden().Default("1").IntVar(&cmdNodes.count)
	nodeAdd.Flag("format", "output format, 'text' or 'json'").Hidden().Default("text").StringVar(&cmdNodes.format)
	nodeAdd.Alias(AddNodeHelp)
	nodeList := nodes.Command("ls", "List all active SSH nodes within the cluster")
	nodeList.Alias(ListNodesHelp)

	// operations on invitation tokens
	tokens := app.Command("tokens", "List or revoke invitation tokens")
	tokenList := tokens.Command("ls", "List node and user invitation tokens")
	tokenDel := tokens.Command("del", "Delete/revoke an invitation token")
	tokenDel.Arg("token", "Token to delete").StringVar(&cmdTokens.token)

	// operations with authorities
	auth := app.Command("auth", "Operations with user and host certificate authorities").Hidden()
	auth.Flag("type", "authority type, 'user' or 'host'").StringVar(&cmdAuth.authType)
	authList := auth.Command("ls", "List trusted certificate authorities (CAs)")
	authExport := auth.Command("export", "Export CA keys to standard output")
	authExport.Flag("keys", "if set, will print private keys").BoolVar(&cmdAuth.exportPrivateKeys)
	authExport.Flag("fingerprint", "filter authority by fingerprint").StringVar(&cmdAuth.exportAuthorityFingerprint)

	authGenerate := auth.Command("gen", "Generate a new SSH keypair")
	authGenerate.Flag("pub-key", "path to the public key").Required().StringVar(&cmdAuth.genPubPath)
	authGenerate.Flag("priv-key", "path to the private key").Required().StringVar(&cmdAuth.genPrivPath)

	authGenAndSign := auth.Command("gencert", "Generate OpenSSH keys and certificate for a joining teleport proxy, node or auth server").Hidden()
	authGenAndSign.Flag("priv-key", "path to the private key to write").Required().StringVar(&cmdAuth.genPrivPath)
	authGenAndSign.Flag("cert", "path to the public signed cert to write").Required().StringVar(&cmdAuth.genPubPath)
	authGenAndSign.Flag("sign-key", "path to the private OpenSSH signing key").Required().StringVar(&cmdAuth.genSigningKeyPath)
	authGenAndSign.Flag("role", "server role, e.g. 'proxy', 'auth' or 'node'").Required().SetValue(&cmdAuth.genRole)
	authGenAndSign.Flag("domain", "cluster certificate authority domain name").Required().StringVar(&cmdAuth.genAuthorityDomain)

	// operations with reverse tunnels
	reverseTunnels := app.Command("tunnels", "Operations on reverse tunnels clusters").Hidden()
	reverseTunnelsList := reverseTunnels.Command("ls", "List tunnels").Hidden()
	reverseTunnelsDelete := reverseTunnels.Command("del", "Delete a tunnel").Hidden()
	reverseTunnelsDelete.Arg("name", "Tunnels to delete").
		Required().StringVar(&cmdReverseTunnel.domainNames)
	reverseTunnelsUpsert := reverseTunnels.Command("add", "Create a new reverse tunnel").Hidden()
	reverseTunnelsUpsert.Arg("name", "Name of the tunnel").
		Required().StringVar(&cmdReverseTunnel.domainNames)
	reverseTunnelsUpsert.Arg("addrs", "Comma-separated list of tunnels").
		Required().SetValue(&cmdReverseTunnel.dialAddrs)
	reverseTunnelsUpsert.Flag("ttl", "Optional TTL (time to live) for the tunnel").DurationVar(&cmdReverseTunnel.ttl)

	// parse CLI commands+flags:
	command, err := app.Parse(os.Args[1:])
	if err != nil {
		utils.FatalError(err)
	}

	applyConfig(&ccf, cfg)
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
		err = cmdUsers.Add(client)
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
	case reverseTunnelsList.FullCommand():
		err = cmdReverseTunnel.ListActive(client)
	case reverseTunnelsDelete.FullCommand():
		err = cmdReverseTunnel.Delete(client)
	case reverseTunnelsUpsert.FullCommand():
		err = cmdReverseTunnel.Upsert(client)
	case tokenList.FullCommand():
		err = cmdTokens.List(client)
	case tokenDel.FullCommand():
		err = cmdTokens.Del(client)
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
func (u *UserCommand) Add(client *auth.TunClient) error {

	// if no local logins were specified, default to 'login'
	if u.allowedLogins == "" {
		u.allowedLogins = u.login
	}
	user := services.TeleportUser{
		Name:          u.login,
		AllowedLogins: strings.Split(u.allowedLogins, ","),
	}
	if len(u.identities) != 0 {
		for _, identityVar := range u.identities {
			vals := strings.SplitN(identityVar, ":", 2)
			if len(vals) != 2 {
				return trace.Errorf("bad flag --identity=%v, expected <connector-id>:<email> format", identityVar)
			}
			user.OIDCIdentities = append(user.OIDCIdentities, services.OIDCIdentity{ConnectorID: vals[0], Email: vals[1]})
		}
	}
	token, err := client.CreateSignupToken(&user)
	if err != nil {
		return err
	}
	proxies, err := client.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}
	hostname := "teleport-proxy"
	if len(proxies) == 0 {
		fmt.Printf("\x1b[1mWARNING\x1b[0m: this Teleport cluster does not have any proxy servers online.\nYou need to start some to be able to login.\n\n")
	} else {
		hostname = proxies[0].Hostname
	}
	url := web.CreateSignupLink(net.JoinHostPort(hostname, strconv.Itoa(defaults.HTTPListenPort)), token)
	fmt.Printf("Signup token has been created and is valid for %v seconds. Share this URL with the user:\n%v\n\nNOTE: make sure '%s' is accessible!\n", defaults.MaxSignupTokenTTL.Seconds(), url, hostname)
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
			fmt.Fprintf(t, "%v\t%v\n", u.GetName(), strings.Join(u.GetAllowedLogins(), ","))
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
		return trace.BadParameter("count should be > 0, got %v", u.count)
	}
	// parse --roles flag
	roles, err := teleport.ParseRoles(u.roles)
	if err != nil {
		return trace.Wrap(err)
	}
	var tokens []string
	for i := 0; i < u.count; i++ {
		token, err := client.GenerateToken(roles, u.ttl)
		if err != nil {
			return trace.Wrap(err)
		}
		tokens = append(tokens, token)
	}

	authServers, err := client.GetAuthServers()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(authServers) == 0 {
		return trace.Errorf("This cluster does not have any auth servers running")
	}

	// output format swtich:
	if u.format == "text" {
		for _, token := range tokens {
			fmt.Printf(
				"The invite token: %v\nRun this on the new node to join the cluster:\n> teleport start --roles=%s --token=%v --auth-server=%v\n\nPlease note:\n",
				token, strings.ToLower(roles.String()), token, authServers[0].Addr)
		}
		fmt.Printf("  - This invitation token will expire in %d minutes\n", int(u.ttl.Minutes()))
		fmt.Printf("  - %v must be reachable from the new node, see --advertise-ip server flag\n", authServers[0].Addr)
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
	// by default show authorities of both types:
	authTypes := []services.CertAuthType{
		services.UserCA,
		services.HostCA,
	}
	// but if there was a --type switch, only select those:
	if a.authType != "" {
		authTypes = []services.CertAuthType{services.CertAuthType(a.authType)}
		if err := authTypes[0].Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	authorities := make([]*services.CertAuthority, 0)
	for _, t := range authTypes {
		ats, err := client.GetCertAuthorities(t, false)
		if err != nil {
			return trace.Wrap(err)
		}
		authorities = append(authorities, ats...)
	}
	view := func() string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Type", "Cluster Name", "Fingerprint", "Allowed Logins"})
		if len(authorities) == 0 {
			return t.String()
		}
		for _, a := range authorities {
			for _, keyBytes := range a.CheckingKeys {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
				if err != nil {
					fingerprint = fmt.Sprintf("<bad key: %v", err)
				}
				logins := strings.Join(a.AllowedLogins, ",")
				if logins == "" {
					logins = "<any>"
				}
				fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", a.Type, a.DomainName, fingerprint, logins)
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

// GenerateKeys generates a new keypair
func (a *AuthCommand) GenerateKeys() error {
	keygen := native.New()
	defer keygen.Close()
	privBytes, pubBytes, err := keygen.GenerateKeyPair("")
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
	defer ca.Close()
	privBytes, pubBytes, err := ca.GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}
	nodeID := uuid.New()
	certBytes, err := ca.GenerateHostCert(privSigningKeyBytes, pubBytes, nodeID, a.genAuthorityDomain, teleport.Roles{a.genRole}, 0)
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
			if trace.IsNotFound(err) {
				return trace.Errorf("'%v' is not found", domainName)
			}
			return trace.Wrap(err)
		}
		fmt.Printf("Cluster '%v' has been disconnected\n", domainName)
	}
	return nil
}

// connectToAuthService creates a valid client connection to the auth service
func connectToAuthService(cfg *service.Config) (client *auth.TunClient, err error) {
	// connect to the local auth server by default:
	cfg.Auth.Enabled = true
	if len(cfg.AuthServers) == 0 {
		cfg.AuthServers = []utils.NetAddr{
			*defaults.AuthConnectAddr(),
		}
	}
	// read the host SSH keys and use them to open an SSH connection to the auth service
	i, err := auth.ReadIdentity(cfg.DataDir, auth.IdentityID{Role: teleport.RoleAdmin, HostUUID: cfg.HostUUID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err = auth.NewTunClient(
		"tctl",
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

// applyConfig takes configuration values from the config file and applies
// them to 'service.Config' object
func applyConfig(ccf *CLIConfig, cfg *service.Config) error {
	// load /etc/teleport.yaml and apply it's values:
	fileConf, err := config.ReadConfigFile(ccf.ConfigFile)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = config.ApplyFileConfig(fileConf, cfg); err != nil {
		return trace.Wrap(err)
	}
	// --debug flag
	if ccf.Debug {
		utils.InitLoggerDebug()
		logrus.Debugf("DEBUG loggign enabled")
	}
	return nil
}

// onTokenList is called to execute "tokens ls" command
func (c *TokenCommand) List(client *auth.TunClient) error {
	tokens, err := client.GetTokens()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(tokens) == 0 {
		fmt.Println("No active tokens found.")
		return nil
	}
	tokensView := func() string {
		table := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(table, []string{"Token", "Role", "Expiry Time (UTC)"})
		for _, t := range tokens {
			expiry := "never"
			if t.Expires.Unix() > 0 {
				expiry = t.Expires.Format(time.RFC822)
			}
			fmt.Fprintf(table, "%v\t%v\t%s\n", t.Token, t.Roles.String(), expiry)
		}
		return table.String()
	}
	fmt.Printf(tokensView())
	return nil
}

// onTokenList is called to execute "tokens del" command
func (c *TokenCommand) Del(client *auth.TunClient) error {
	if c.token == "" {
		return trace.Errorf("Need an argument: token")
	}
	if err := client.DeleteToken(c.token); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Token %s has been deleted\n", c.token)
	return nil
}
