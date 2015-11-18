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
package command

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/terminal"
	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
)

type Command struct {
	client *auth.TunClient
	out    io.Writer
	in     io.Reader
}

func NewCommand() *Command {
	return &Command{
		out: os.Stdout,
		in:  os.Stdin,
	}
}

func (cmd *Command) SetClient(client *auth.TunClient) {
	cmd.client = client
}

func (cmd *Command) SetOut(out io.Writer) {
	cmd.out = out
}

func (cmd *Command) Run(args []string) error {
	app := kingpin.New("tctl", "CLI for key management of teleport SSH cluster")
	configPath := app.Flag("config", "Path to the Teleport configuration file").Default(DefaultConfigPath).String()

	// SSH Key pair
	keyPair := app.Command("keypair", "Helper operations with SSH keypairs")

	keyPairNew := keyPair.Command("new", "Generate new keypair")
	keyPairNewPrivate := keyPairNew.Arg("private-key-filename", "File name where private key path will be written").Required().String()
	keyPairNewPublic := keyPairNew.Arg("public-key-filename", "File name where public key path will be written").Required().String()
	keyPairNewPass := keyPairNew.Flag("passphrase", "Passphrase to use when encrypting the private key").String()

	// Host CA
	hostCa := app.Command("host-ca", "Operations with host certificate authority")

	hostCaReset := hostCa.Command("reset", "Reset host certificate authority keys")
	hostCaResetConfirm := hostCaReset.Flag("confirm", "Automatically apply the operation without confirmation").Bool()

	hostCaPubKey := hostCa.Command("pub-key", "print host certificate authority public key")

	// User CA
	userCa := app.Command("user-ca", "Operations with user certificate authority")

	userCaReset := userCa.Command("reset", "Reset user certificate authority keys")
	userCaResetConfirm := userCaReset.Flag("confirm", "Automatically apply the operation without confirmation").Bool()

	userCaPubKey := userCa.Command("pub-key", "Print user certificate authority public key")

	// Remote CA
	remoteCa := app.Command("remote-ca", "Operations with remote certificate authority")

	remoteCaUpsert := remoteCa.Command("upsert", "Upsert remote certificate to trust")
	remoteCaUpsertID := remoteCaUpsert.Flag("id", "Certificate id").Required().String()
	remoteCaUpsertFQDN := remoteCaUpsert.Flag("fqdn", "FQDN of the remote party").Required().String()
	remoteCaUpsertType := remoteCaUpsert.Flag("type", "Cert type (host or user)").Required().String()
	remoteCaUpsertPath := remoteCaUpsert.Flag("path", "Cert path (reads from stdout if omitted)").Required().ExistingFile()
	remoteCaUpsertTTL := remoteCaUpsert.Flag("ttl", "ttl for certificate to be trusted").Duration()

	remoteCaLs := remoteCa.Command("ls", "List trusted remote certificates")
	remoteCaLsFQDN := remoteCaLs.Flag("fqdn", "FQDN of the remote party").String()
	remoteCaLsType := remoteCaLs.Flag("type", "Cert type (host or user)").Required().String()

	remoteCaRm := remoteCa.Command("rm", "Remote remote Certificate authority from list of trusted certs")
	remoteCaRmID := remoteCaRm.Flag("id", "Certificate id").Required().String()
	remoteCaRmFQDN := remoteCaRm.Flag("fqdn", "FQDN of the remote party").Required().String()
	remoteCaRmType := remoteCaRm.Flag("type", "Cert type (host or user)").Required().String()

	// Secret
	secret := app.Command("secret", "Operations with secret tokens")

	secretNew := secret.Command("new", "Generate new secret key")
	secretNewKeyFileName := secretNew.Flag("filename", "If filename is provided, the key will be saved to that file").Default("").String()

	// Token
	token := app.Command("token", "Generates provisioning tokens")

	tokenGenerate := token.Command("generate", "Generate provisioning token for server with fqdn")
	tokenGenerateFQDN := tokenGenerate.Flag("fqdn", "FQDN of the server").Required().String()
	tokenGenerateRole := tokenGenerate.Flag("role", "Role of the server: Node or Auth ").Default(auth.RoleNode).String()
	tokenGenerateTTL := tokenGenerate.Flag("ttl", "Time to live").Default("120s").Duration()
	tokenGenerateOutput := tokenGenerate.Flag("output", "Optional output file").String()
	tokenGenerateSecret := tokenGenerate.Flag("secret", "Optional secret key, will be used to generate secure token instead of talking to server").String()

	// User
	user := app.Command("user", "Operations with registered users")

	userLs := user.Command("ls", "List users registered in teleport")

	userDelete := user.Command("delete", "Delete user")
	userDeleteUser := userDelete.Flag("user", "User to delete").Required().String()

	userUpsertKey := user.Command("upsert-key", "Grant access to the user key, returns signed certificate")
	userUpsertKeyUser := userUpsertKey.Flag("user", "User holding the key").Required().String()
	userUpsertKeyKeyID := userUpsertKey.Flag("key-id", "SSH key ID").Required().String()
	userUpsertKeyKey := userUpsertKey.Flag("key", "Path to public key").Required().ExistingFile()
	userUpsertKeyTTL := userUpsertKey.Flag("ttl", "Access time to live, certificate and access entry will expire when set").Duration()

	userLsKeys := user.Command("ls-keys", "List user's keys registered in teleport")
	userLsKeysUser := userLsKeys.Flag("user", "User to list keys form").Required().String()

	userSetPass := user.Command("set-pass", "Set user password")
	userSetPassUser := userSetPass.Flag("user", "User name").Required().String()
	userSetPassPass := userSetPass.Flag("pass", "Password").Required().String()

	// Backend keys
	backendKey := app.Command("backend-keys", "Operation with backend encryption keys")

	backendKeyLs := backendKey.Command("ls", "List all the keys that this servers has")

	backendKeyGenerate := backendKey.Command("generate", "Generate a new encrypting key and make a copy of all the backend data using this key")
	backendKeyGenerateName := backendKeyGenerate.Flag("name", "key name").Required().String()

	backendKeyImport := backendKey.Command("import", "Import key from file")
	backendKeyImportFile := backendKeyImport.Flag("file", "filename").Required().ExistingFile()

	backendKeyExport := backendKey.Command("export", "Export key to file")
	backendKeyExportFile := backendKeyExport.Flag("dir", "output directory").Required().ExistingFileOrDir()
	backendKeyExportID := backendKeyExport.Flag("id", "key id").Required().String()

	backendKeyDelete := backendKey.Command("delete", "Delete key from that server storage and delete all the data encrypted using this key from backend")
	backendKeyDeleteID := backendKeyDelete.Flag("id", "key id").Required().String()

	// Teleagent
	agent := app.Command("agent", "Teleport ssh agent")

	agentStart := agent.Command("start", "Generate remote server certificate for teleagent ssh agent using your credentials")
	agentStartAgentAddr := agentStart.Flag("agent-addr", "ssh agent listening address").Default(teleagent.DefaultAgentAddress).String()
	agentStartAPIAddr := agentStart.Flag("api-addr", "api listening address").Default(teleagent.DefaultAgentAPIAddress).String()

	agentLogin := agent.Command("login", "Generate remote server certificate for teleagent ssh agent using your credentials")
	agentLoginAgentAddr := agentLogin.Flag("agent-api-addr", "ssh agent api address").Default(teleagent.DefaultAgentAPIAddress).String()
	agentLoginProxyAddr := agentLogin.Flag("proxy-addr", "FQDN of the remote proxy").Required().String()
	agentLoginTTL := agentLogin.Flag("ttl", "Certificate duration").Default("10h").Duration()

	selectedCommand := kingpin.MustParse(app.Parse(args[1:]))

	if !strings.HasPrefix(selectedCommand, agent.FullCommand()) {
		var cfg service.Config
		if err := service.ParseYAMLFile(*configPath, &cfg); err != nil {
			return trace.Wrap(err)
		}
		service.SetDefaults(&cfg)
		if cfg.Auth.Enabled && len(cfg.AuthServers) == 0 {
			cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}
		}

		signer, err := auth.ReadKeys(cfg.Hostname, cfg.DataDir)
		if err != nil {
			return trace.Wrap(err)
		}

		if len(cfg.AuthServers) == 0 {
			return fmt.Errorf("provide auth server address")
		}

		cmd.client, err = auth.NewTunClient(
			cfg.AuthServers[0],
			cfg.Hostname,
			[]ssh.AuthMethod{ssh.PublicKeys(signer)})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var err error

	switch selectedCommand {
	// Host CA
	case keyPairNew.FullCommand():
		err = cmd.GenerateKeyPair(*keyPairNewPrivate, *keyPairNewPublic, *keyPairNewPass)

	// Host CA
	case hostCaReset.FullCommand():
		cmd.ResetHostCertificateAuthority(*hostCaResetConfirm)
	case hostCaPubKey.FullCommand():
		cmd.GetHostPublicCertificate()

	// User CA
	case userCaReset.FullCommand():
		cmd.ResetUserCertificateAuthority(*userCaResetConfirm)
	case userCaPubKey.FullCommand():
		cmd.GetUserPublicCertificate()

	// Remote CA
	case remoteCaUpsert.FullCommand():
		cmd.UpsertRemoteCertificate(*remoteCaUpsertID, *remoteCaUpsertFQDN,
			*remoteCaUpsertType, *remoteCaUpsertPath, *remoteCaUpsertTTL)
	case remoteCaLs.FullCommand():
		cmd.GetRemoteCertificates(*remoteCaLsFQDN, *remoteCaLsType)
	case remoteCaRm.FullCommand():
		cmd.DeleteRemoteCertificate(*remoteCaRmID, *remoteCaRmFQDN, *remoteCaRmType)

	// Secret
	case secretNew.FullCommand():
		cmd.NewKey(*secretNewKeyFileName)

	// Token
	case tokenGenerate.FullCommand():
		err = cmd.GenerateToken(*tokenGenerateFQDN, *tokenGenerateRole,
			*tokenGenerateTTL, *tokenGenerateOutput, *tokenGenerateSecret)

	// User
	case userLs.FullCommand():
		cmd.GetUsers()
	case userDelete.FullCommand():
		cmd.DeleteUser(*userDeleteUser)
	case userUpsertKey.FullCommand():
		cmd.UpsertKey(*userUpsertKeyUser, *userUpsertKeyKeyID,
			*userUpsertKeyKey, *userUpsertKeyTTL)
	case userLsKeys.FullCommand():
		cmd.GetUserKeys(*userLsKeysUser)
	case userSetPass.FullCommand():
		cmd.SetPass(*userSetPassUser, *userSetPassPass)

	// Backend keys
	case backendKeyLs.FullCommand():
		cmd.GetBackendKeys()
	case backendKeyGenerate.FullCommand():
		cmd.GenerateBackendKey(*backendKeyGenerateName)
	case backendKeyImport.FullCommand():
		cmd.ImportBackendKey(*backendKeyImportFile)
	case backendKeyExport.FullCommand():
		cmd.ExportBackendKey(*backendKeyExportFile, *backendKeyExportID)
	case backendKeyDelete.FullCommand():
		cmd.DeleteBackendKey(*backendKeyDeleteID)

	// Teleagent
	case agentStart.FullCommand():
		cmd.AgentStart(*agentStartAgentAddr, *agentStartAPIAddr)

	case agentLogin.FullCommand():
		cmd.AgentLogin(*agentLoginAgentAddr, *agentLoginProxyAddr,
			*agentLoginTTL)
	}

	return err
}

func (cmd *Command) readInput(path string) ([]byte, error) {
	if path != "" {
		return utils.ReadPath(path)
	}
	reader := bufio.NewReader(cmd.in)
	return reader.ReadSlice('\n')
}

func (cmd *Command) readPassword() (string, error) {
	password, err := terminal.ReadPassword(0)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(password), nil
}

func (cmd *Command) confirm(message string) bool {
	reader := bufio.NewReader(cmd.in)
	fmt.Fprintf(cmd.out, fmt.Sprintf("%v (Y/N): ", message))
	text, _ := reader.ReadString('\n')
	text = strings.Trim(text, "\n\r\t")
	return text == "Y" || text == "yes" || text == "y"
}

func (cmd *Command) printResult(format string, in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOK(format, fmt.Sprintf("%v", in))
	}
}

func (cmd *Command) printStatus(in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOK("%s", in)
	}
}

func (cmd *Command) printError(err error) {
	fmt.Fprint(cmd.out, goterm.Color(fmt.Sprintf("ERROR: %s", err), goterm.RED)+"\n")
}

func (cmd *Command) printOK(message string, params ...interface{}) {
	fmt.Fprintf(cmd.out,
		goterm.Color(
			fmt.Sprintf("OK: %s\n", fmt.Sprintf(message, params...)), goterm.GREEN)+"\n")
}

func (cmd *Command) printInfo(message string, params ...interface{}) {
	fmt.Fprintf(cmd.out, "INFO: %s\n", fmt.Sprintf(message, params...))
}

const DefaultTeleportURL = "unix:///tmp/teleport.auth.sock"
const DefaultConfigPath = "/var/lib/teleport/teleport.yaml"
