/*
Copyright 2015-2017 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/Sirupsen/logrus"
	"github.com/buger/goterm"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	kyaml "k8s.io/client-go/1.4/pkg/util/yaml"
)

type CLIConfig struct {
	Debug        bool
	ConfigFile   string
	ConfigString string
}

type UserCommand struct {
	config        *service.Config
	login         string
	allowedLogins string
	roles         string
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
	// namespace is node namespace
	namespace string
}

type AuthCommand struct {
	config                     *service.Config
	authType                   string
	genPubPath                 string
	genPrivPath                string
	genUser                    string
	genTTL                     time.Duration
	exportAuthorityFingerprint string
	exportPrivateKeys          bool
	outDir                     string
	compatVersion              string
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

type GetCommand struct {
	config      *service.Config
	ref         services.Ref
	format      string
	namespace   string
	withSecrets bool
}

type CreateCommand struct {
	config   *service.Config
	filename string
}

type DeleteCommand struct {
	config *service.Config
	ref    services.Ref
}

func Run() {
	utils.InitLogger(utils.LoggingForCLI, logrus.WarnLevel)
	app := utils.InitCLIParser("tctl", GlobalHelpString)

	// generate default tctl configuration:
	cfg := service.MakeDefaultConfig()
	cmdUsers := UserCommand{config: cfg}
	cmdNodes := NodeCommand{config: cfg}
	cmdAuth := AuthCommand{config: cfg}
	cmdReverseTunnel := ReverseTunnelCommand{config: cfg}
	cmdTokens := TokenCommand{config: cfg}
	cmdGet := GetCommand{config: cfg}
	cmdCreate := CreateCommand{config: cfg}
	cmdDelete := DeleteCommand{config: cfg}

	// define global flags:
	var ccf CLIConfig
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)
	app.Flag("config", fmt.Sprintf("Path to a configuration file [%v]", defaults.ConfigFilePath)).
		Short('c').
		ExistingFileVar(&ccf.ConfigFile)
	app.Flag("config-string",
		"Base64 encoded configuration string").Hidden().Envar(defaults.ConfigEnvar).StringVar(&ccf.ConfigString)

	// commands:
	ver := app.Command("version", "Print the version.")
	app.HelpFlag.Short('h')

	// user add command:
	users := app.Command("users", "Manage users logins")

	userAdd := users.Command("add", "Generate an invitation token and print the signup URL")
	userAdd.Arg("login", "Teleport user login").Required().StringVar(&cmdUsers.login)
	userAdd.Arg("local-logins", "Local UNIX users this account can log in as [login]").
		Default("").StringVar(&cmdUsers.allowedLogins)
	userAdd.Alias(AddUserHelp)

	userUpdate := users.Command("update", "Update properties for existing user").Hidden()
	userUpdate.Arg("login", "Teleport user login").Required().StringVar(&cmdUsers.login)
	userUpdate.Flag("set-roles", "Roles to assign to this user").
		Default("").StringVar(&cmdUsers.roles)

	delete := app.Command("del", "Delete resources").Hidden()
	delete.Arg("resource", "Resource to delete").SetValue(&cmdDelete.ref)

	// get one or many resources in the system
	get := app.Command("get", "Get one or many objects in the system").Hidden()
	get.Arg("resource", "Resource type and name").SetValue(&cmdGet.ref)
	get.Flag("format", "Format output type, one of 'yaml', 'json' or 'text'").Default(formatText).StringVar(&cmdGet.format)
	get.Flag("namespace", "Namespace of the resources").Default(defaults.Namespace).StringVar(&cmdGet.namespace)
	get.Flag("with-secrets", "Include secrets in resources like certificate authorities or OIDC connectors").Default("false").BoolVar(&cmdGet.withSecrets)

	// upsert one or many resources
	create := app.Command("create", "Create or update a resource").Hidden()
	create.Flag("filename", "resource definition file").Short('f').StringVar(&cmdCreate.filename)

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
	nodeAdd.Flag("ttl", "Time to live for a generated token").Default(defaults.ProvisioningTokenTTL.String()).DurationVar(&cmdNodes.ttl)
	nodeAdd.Flag("count", "add count tokens and output JSON with the list").Hidden().Default("1").IntVar(&cmdNodes.count)
	nodeAdd.Flag("format", "output format, 'text' or 'json'").Hidden().Default("text").StringVar(&cmdNodes.format)
	nodeAdd.Alias(AddNodeHelp)
	nodeList := nodes.Command("ls", "List all active SSH nodes within the cluster")
	nodeList.Flag("namespace", "Namespace of the nodes").Default(defaults.Namespace).StringVar(&cmdNodes.namespace)
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
	authExport.Flag("compat", "export cerfiticates compatible with specific version of Teleport").StringVar(&cmdAuth.compatVersion)

	authGenerate := auth.Command("gen", "Generate a new SSH keypair")
	authGenerate.Flag("pub-key", "path to the public key").Required().StringVar(&cmdAuth.genPubPath)
	authGenerate.Flag("priv-key", "path to the private key").Required().StringVar(&cmdAuth.genPrivPath)

	authSign := auth.Command("sign", "Create a signed user session cerfiticate")
	authSign.Flag("user", "Teleport user name").Required().StringVar(&cmdAuth.genUser)
	authSign.Flag("out", "Output directory [defaults to current]").Short('o').StringVar(&cmdAuth.outDir)
	authSign.Flag("ttl", "TTL (time to live) for the generated certificate").Default(fmt.Sprintf("%v", defaults.CertDuration)).DurationVar(&cmdAuth.genTTL)

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

	// "version" command?
	if command == ver.FullCommand() {
		onVersion()
		return
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
	}
	// connect to the teleport auth service:
	client, err := connectToAuthService(cfg)
	if err != nil {
		utils.FatalError(err)
	}

	// execute the selected command:
	switch command {
	case get.FullCommand():
		err = cmdGet.Get(client)
	case create.FullCommand():
		err = cmdCreate.Create(client)
	case delete.FullCommand():
		err = cmdDelete.Delete(client)
	case userAdd.FullCommand():
		err = cmdUsers.Add(client)
	case userList.FullCommand():
		err = cmdUsers.List(client)
	case userUpdate.FullCommand():
		err = cmdUsers.Update(client)
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
	case authSign.FullCommand():
		err = cmdAuth.GenerateAndSignKeys(client)
		if err != nil {
			utils.FatalError(err)
		}
		return
	}

	if err != nil {
		logrus.Error(trace.DebugReport(err))
		utils.FatalError(err)
	}
}

func Ref(s kingpin.Settings) *services.Ref {
	r := new(services.Ref)
	s.SetValue(r)
	return r
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
	user := services.UserV1{
		Name:          u.login,
		AllowedLogins: strings.Split(u.allowedLogins, ","),
	}
	token, err := client.CreateSignupToken(user)
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
		hostname = proxies[0].GetHostname()
	}

	// try to auto-suggest the activation link
	_, proxyPort, err := net.SplitHostPort(u.config.Proxy.WebAddr.Addr)
	if err != nil {
		proxyPort = strconv.Itoa(defaults.HTTPListenPort)
	}
	url := web.CreateSignupLink(net.JoinHostPort(hostname, proxyPort), token)
	fmt.Printf("Signup token has been created and is valid for %v seconds. Share this URL with the user:\n%v\n\nNOTE: make sure '%s' is accessible!\n", defaults.MaxSignupTokenTTL.Seconds(), url, hostname)
	return nil
}

// List prints all existing user accounts
func (u *UserCommand) List(client *auth.TunClient) error {
	users, err := client.GetUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &userCollection{users: users}
	coll.writeText(os.Stdout)
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

// Update updates existing user
func (u *UserCommand) Update(client *auth.TunClient) error {
	user, err := client.GetUser(u.login)
	if err != nil {
		return trace.Wrap(err)
	}
	roles := strings.Split(u.roles, ",")
	for _, role := range roles {
		if _, err := client.GetRole(role); err != nil {
			return trace.Wrap(err)
		}
	}
	user.SetRoles(roles)
	if err := client.UpsertUser(user); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%v has been updated with roles %v\n", user.GetName(), strings.Join(user.GetRoles(), ","))
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
				token, strings.ToLower(roles.String()), token, authServers[0].GetAddr())
		}
		fmt.Printf("  - This invitation token will expire in %d minutes\n", int(u.ttl.Minutes()))
		fmt.Printf("  - %v must be reachable from the new node, see --advertise-ip server flag\n", authServers[0].GetAddr())
		fmt.Printf(`  - For tokens of type "trustedcluster", tctl needs to be used to create a TrustedCluster resource. See the Admin Guide for more details.`)
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
	nodes, err := client.GetNodes(u.namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &serverCollection{servers: nodes}
	coll.writeText(os.Stdout)
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
	localAuthName, err := client.GetDomainName()
	if err != nil {
		return trace.Wrap(err)
	}
	var (
		localCAs   []services.CertAuthority
		trustedCAs []services.CertAuthority
	)
	for _, t := range authTypes {
		cas, err := client.GetCertAuthorities(t, false)
		if err != nil {
			return trace.Wrap(err)
		}
		for i := range cas {
			if cas[i].GetClusterName() == localAuthName {
				localCAs = append(localCAs, cas[i])
			} else {
				trustedCAs = append(trustedCAs, cas[i])
			}
		}
	}
	localCAsView := func() string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"CA Type", "Fingerprint"})
		for _, a := range localCAs {
			for _, keyBytes := range a.GetCheckingKeys() {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
				if err != nil {
					fingerprint = fmt.Sprintf("<bad key: %v", err)
				}
				fmt.Fprintf(t, "%v\t%v\n", a.GetType(), fingerprint)
			}
		}
		return fmt.Sprintf("CA keys for the local cluster %v:\n\n", localAuthName) +
			t.String()
	}
	trustedCAsView := func() string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Cluster Name", "CA Type", "Fingerprint", "Roles"})
		for _, a := range trustedCAs {
			for _, keyBytes := range a.GetCheckingKeys() {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
				if err != nil {
					fingerprint = fmt.Sprintf("<bad key: %v", err)
				}
				var logins string
				if a.GetType() == services.HostCA {
					logins = "N/A"
				} else {
					logins = strings.Join(a.GetRoles(), ",")
				}
				fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", a.GetClusterName(), a.GetType(), fingerprint, logins)
			}
		}
		return "\nCA Keys for Trusted Clusters:\n\n" + t.String()
	}
	fmt.Printf(localCAsView())
	if len(trustedCAs) > 0 {
		fmt.Printf(trustedCAsView())
	}
	return nil
}

// ExportAuthorities outputs the list of authorities in OpenSSH compatible formats
// If --type flag is given, only prints keys for CAs of this type, otherwise
// prints all keys
func (a *AuthCommand) ExportAuthorities(client *auth.TunClient) error {
	var typesToExport []services.CertAuthType

	// if no --type flag is given, export all types
	if a.authType == "" {
		typesToExport = []services.CertAuthType{services.HostCA, services.UserCA}
	} else {
		authType := services.CertAuthType(a.authType)
		if err := authType.Check(); err != nil {
			return trace.Wrap(err)
		}
		typesToExport = []services.CertAuthType{authType}
	}
	localAuthName, err := client.GetDomainName()
	if err != nil {
		return trace.Wrap(err)
	}

	// fetch authorities via auth API (and only take local CAs, ignoring
	// trusted ones)
	var authorities []services.CertAuthority
	for _, at := range typesToExport {
		cas, err := client.GetCertAuthorities(at, a.exportPrivateKeys)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, ca := range cas {
			if ca.GetClusterName() == localAuthName {
				authorities = append(authorities, ca)
			}
		}
	}

	// print:
	for _, ca := range authorities {
		if a.exportPrivateKeys {
			for _, key := range ca.GetSigningKeys() {
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
			for _, keyBytes := range ca.GetCheckingKeys() {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}

				// export certificates in the old 1.0 format where host and user
				// certificate authorities were exported in the known_hosts format.
				if a.compatVersion == "1.0" {
					castr, err := hostCAFormat(ca, keyBytes, client)
					if err != nil {
						return trace.Wrap(err)
					}

					fmt.Println(castr)
					continue
				}

				// export certificate authority in user or host ca format
				var castr string
				switch ca.GetType() {
				case services.UserCA:
					castr, err = userCAFormat(ca, keyBytes)
				case services.HostCA:
					castr, err = hostCAFormat(ca, keyBytes, client)
				default:
					return trace.BadParameter("unknown user type: %q", ca.GetType())
				}
				if err != nil {
					return trace.Wrap(err)
				}

				// print the export friendly string
				fmt.Println(castr)
			}
		}
	}
	return nil
}

// userCAFormat returns the certificate authority public key exported as a single
// line that can be placed in ~/.ssh/authorized_keys file. The format adheres to the
// man sshd (8) authorized_keys format, a space-separated list of: options, keytype,
// base64-encoded key, comment.
// For example:
//
//    cert-authority AAA... type=user&clustername=cluster-a
//
// URL encoding is used to pass the CA type and cluster name into the comment field.
func userCAFormat(ca services.CertAuthority, keyBytes []byte) (string, error) {
	comment := url.Values{
		"type":        []string{string(services.UserCA)},
		"clustername": []string{ca.GetClusterName()},
	}

	return fmt.Sprintf("cert-authority %s %s", strings.TrimSpace(string(keyBytes)), comment.Encode()), nil
}

// hostCAFormat returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/authorized_hosts. The format adheres to the man sshd (8)
// authorized_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//    @cert-authority *.cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func hostCAFormat(ca services.CertAuthority, keyBytes []byte, client *auth.TunClient) (string, error) {
	comment := url.Values{
		"type": []string{string(ca.GetType())},
	}

	roles, err := services.FetchRoles(ca.GetRoles(), client)
	if err != nil {
		return "", trace.Wrap(err)
	}
	allowedLogins, _ := roles.CheckLogins(defaults.MinCertDuration + time.Second)
	if len(allowedLogins) > 0 {
		comment["logins"] = allowedLogins
	}

	return fmt.Sprintf("@cert-authority *.%s %s %s",
		ca.GetClusterName(), strings.TrimSpace(string(keyBytes)), comment.Encode()), nil
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
func (a *AuthCommand) GenerateAndSignKeys(client *auth.TunClient) error {
	ca := native.New()
	defer ca.Close()
	privateKey, publicKey, err := ca.GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}
	cert, err := client.GenerateUserCert(publicKey, a.genUser, a.genTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	certPath := a.genUser + ".cert"
	keyPath := a.genUser + ".key"
	pubPath := a.genUser + ".pub"

	// --out flag
	if a.outDir != "" {
		if !utils.IsDir(a.outDir) {
			if err = os.MkdirAll(a.outDir, 0770); err != nil {
				return trace.Wrap(err)
			}
		}
		certPath = filepath.Join(a.outDir, certPath)
		keyPath = filepath.Join(a.outDir, keyPath)
		pubPath = filepath.Join(a.outDir, pubPath)
	}

	err = ioutil.WriteFile(certPath, cert, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(keyPath, privateKey, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(pubPath, publicKey, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Public key : %v\nPrivate key: %v\nCertificate: %v\n",
		pubPath, keyPath, certPath)
	return nil
}

// ListActive retreives the list of nodes who recently sent heartbeats to
// to a cluster and prints it to stdout
func (r *ReverseTunnelCommand) ListActive(client *auth.TunClient) error {
	tunnels, err := client.GetReverseTunnels()
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &reverseTunnelCollection{tunnels: tunnels}
	coll.writeText(os.Stdout)
	return nil
}

// Upsert updates or inserts new reverse tunnel
func (r *ReverseTunnelCommand) Upsert(client *auth.TunClient) error {
	tunnel := services.NewReverseTunnel(r.domainNames, r.dialAddrs.Addresses())
	tunnel.SetTTL(clockwork.NewRealClock(), r.ttl)
	err := client.UpsertReverseTunnel(tunnel)
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

// validateConfig validates and updates tctl configuration
func validateConfig(cfg *service.Config) {
	var err error
	// read a host UUID for this node
	cfg.HostUUID, err = utils.ReadHostUUID(cfg.DataDir)
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
	// if configuration is passed as an environment variable,
	// try to decode it and override the config file
	if ccf.ConfigString != "" {
		fileConf, err = config.ReadFromString(ccf.ConfigString)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if err = config.ApplyFileConfig(fileConf, cfg); err != nil {
		return trace.Wrap(err)
	}
	// --debug flag
	if ccf.Debug {
		utils.InitLogger(utils.LoggingForCLI, logrus.DebugLevel)
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

// Get prints one or many resources of a certain type
func (g *GetCommand) Get(client *auth.TunClient) error {
	collection, err := g.getCollection(client)
	if err != nil {
		return trace.Wrap(err)
	}
	switch g.format {
	case formatText:
		return collection.writeText(os.Stdout)
	case formatJSON:
		return collection.writeJSON(os.Stdout)
	case formatYAML:
		return collection.writeYAML(os.Stdout)
	}
	return trace.BadParameter("unsupported format")
}

// Create updates or insterts one or many resources
func (u *CreateCommand) Create(client *auth.TunClient) error {
	var reader io.ReadCloser
	var err error
	if u.filename != "" {
		reader, err = utils.OpenFile(u.filename)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		reader = ioutil.NopCloser(os.Stdin)
	}
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	count := 0
	for {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if err != nil {
			if err == io.EOF {
				if count == 0 {
					return trace.BadParameter("no resources found, emtpy input?")
				}
				return nil
			}
			return trace.Wrap(err)
		}
		count += 1
		switch raw.Kind {
		case services.KindSAMLConnector:
			conn, err := services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := conn.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertSAMLConnector(conn); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("SAML connector %v upserted\n", conn.GetName())
		case services.KindOIDCConnector:
			conn, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertOIDCConnector(conn); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("OIDC connector %v upserted\n", conn.GetName())
		case services.KindReverseTunnel:
			tun, err := services.GetReverseTunnelMarshaler().UnmarshalReverseTunnel(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertReverseTunnel(tun); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("reverse tunnel %v upserted\n", tun.GetName())
		case services.KindCertAuthority:
			ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertCertAuthority(ca); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("cert authority %v upserted\n", ca.GetName())
		case services.KindUser:
			user, err := services.GetUserMarshaler().UnmarshalUser(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertUser(user); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("user %v upserted\n", user.GetName())
		case services.KindRole:
			role, err := services.GetRoleMarshaler().UnmarshalRole(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			err = role.CheckAndSetDefaults()
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertRole(role, backend.Forever); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("role %v upserted\n", role.GetName())
		case services.KindNamespace:
			ns, err := services.UnmarshalNamespace(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertNamespace(*ns); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("namespace %v upserted\n", ns.Metadata.Name)
		case services.KindTrustedCluster:
			tc, err := services.GetTrustedClusterMarshaler().Unmarshal(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.UpsertTrustedCluster(tc); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("trusted cluster %q upserted\n", tc.GetName())
		case services.KindClusterAuthPreference:
			cap, err := services.GetAuthPreferenceMarshaler().Unmarshal(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.SetClusterAuthPreference(cap); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("cluster auth preference upserted\n")
		case services.KindUniversalSecondFactor:
			universalSecondFactor, err := services.GetUniversalSecondFactorMarshaler().Unmarshal(raw.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := client.SetUniversalSecondFactor(universalSecondFactor); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("universal second factor upserted\n")
		case "":
			return trace.BadParameter("missing resource kind")
		default:
			return trace.BadParameter("%q is not supported", raw.Kind)
		}
	}
}

// Delete deletes resource by name
func (d *DeleteCommand) Delete(client *auth.TunClient) error {
	if d.ref.Kind == "" {
		return trace.BadParameter("provide full resource name to delete e.g. roles/example")
	}
	if d.ref.Name == "" {
		return trace.BadParameter("provide full resource name to delete e.g. roles/example")
	}

	switch d.ref.Kind {
	case services.KindUser:
		if err := client.DeleteUser(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %v has been deleted\n", d.ref.Name)
	case services.KindSAMLConnector:
		if err := client.DeleteSAMLConnector(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("SAML Connector %v has been deleted\n", d.ref.Name)
	case services.KindOIDCConnector:
		if err := client.DeleteOIDCConnector(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("OIDC Connector %v has been deleted\n", d.ref.Name)
	case services.KindReverseTunnel:
		if err := client.DeleteReverseTunnel(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("reverse tunnel %v has been deleted\n", d.ref.Name)
	case services.KindRole:
		if err := client.DeleteRole(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("role %v has been deleted\n", d.ref.Name)
	case services.KindNamespace:
		if err := client.DeleteNamespace(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("namespace %v has been deleted\n", d.ref.Name)
	case services.KindTrustedCluster:
		if err := client.DeleteTrustedCluster(d.ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("trusted cluster %q has been deleted\n", d.ref.Name)
	case "":
		return trace.BadParameter("missing resource kind")
	default:
		return trace.BadParameter("%q is not supported", d.ref.Kind)
	}

	return nil
}

func (g *GetCommand) getCollection(client auth.ClientI) (collection, error) {
	if g.ref.Kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}
	switch g.ref.Kind {
	case services.KindSAMLConnector:
		connectors, err := client.GetSAMLConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &samlCollection{connectors: connectors}, nil
	case services.KindOIDCConnector:
		connectors, err := client.GetOIDCConnectors(g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oidcCollection{connectors: connectors}, nil
	case services.KindReverseTunnel:
		tunnels, err := client.GetReverseTunnels()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &reverseTunnelCollection{tunnels: tunnels}, nil
	case services.KindCertAuthority:
		userAuthorities, err := client.GetCertAuthorities(services.UserCA, g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		hostAuthorities, err := client.GetCertAuthorities(services.HostCA, g.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userAuthorities = append(userAuthorities, hostAuthorities...)
		return &authorityCollection{cas: userAuthorities}, nil
	case services.KindUser:
		users, err := client.GetUsers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &userCollection{users: users}, nil
	case services.KindNode:
		nodes, err := client.GetNodes(g.namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: nodes}, nil
	case services.KindAuthServer:
		servers, err := client.GetAuthServers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: servers}, nil
	case services.KindProxy:
		servers, err := client.GetAuthServers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverCollection{servers: servers}, nil
	case services.KindRole:
		if g.ref.Name == "" {
			roles, err := client.GetRoles()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &roleCollection{roles: roles}, nil
		}
		role, err := client.GetRole(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &roleCollection{roles: []services.Role{role}}, nil
	case services.KindNamespace:
		if g.ref.Name == "" {
			namespaces, err := client.GetNamespaces()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &namespaceCollection{namespaces: namespaces}, nil
		}
		ns, err := client.GetNamespace(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &namespaceCollection{namespaces: []services.Namespace{*ns}}, nil
	case services.KindTrustedCluster:
		if g.ref.Name == "" {
			trustedClusters, err := client.GetTrustedClusters()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &trustedClusterCollection{trustedClusters: trustedClusters}, nil
		}
		trustedCluster, err := client.GetTrustedCluster(g.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &trustedClusterCollection{trustedClusters: []services.TrustedCluster{trustedCluster}}, nil
	case services.KindClusterAuthPreference:
		cap, err := client.GetClusterAuthPreference()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &authPreferenceCollection{AuthPreference: cap}, nil
	case services.KindUniversalSecondFactor:
		universalSecondFactor, err := client.GetUniversalSecondFactor()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &universalSecondFactorCollection{UniversalSecondFactor: universalSecondFactor}, nil
	}

	return nil, trace.BadParameter("'%v' is not supported", g.ref.Kind)
}

const (
	formatYAML = "yaml"
	formatText = "text"
	formatJSON = "json"
)
