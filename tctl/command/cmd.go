package command

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/utils"
)

type Command struct {
	client *auth.Client
	out    io.Writer
	in     io.Reader
}

func NewCommand() *Command {
	return &Command{
		out: os.Stdout,
		in:  os.Stdin,
	}
}

func (cmd *Command) Run(args []string) error {
	app := kingpin.New("tctl", "CLI for key management of teleport SSH cluster")
	authUrl := app.Flag("auth", "Teleport URL").Default(DefaultTeleportURL).String()

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

	remoteCaRm := remoteCa.Command("rm", "Remote remote CA from list of trusted certs")
	remoteCaRmID := remoteCaRm.Flag("id", "Certificate id").Required().String()
	remoteCaRmFQDN := remoteCaRm.Flag("fqdn", "FQDN of the remote party").Required().String()
	remoteCaRmType := remoteCaRm.Flag("type", "Cert type (host or user)").Required().String()

	// Secret
	secret := app.Command("secret", "Operations with secret tokens")

	secretNew := secret.Command("new", "Generate new secret key")

	// Token
	token := app.Command("token", "Generates provisioning tokens")

	tokenGenerate := token.Command("generate", "Generate provisioning token for server with fqdn")
	tokenGenerateFQDN := tokenGenerate.Flag("fqdn", "FQDN of the server").Required().String()
	tokenGenerateTTL := tokenGenerate.Flag("ttl", "Time to live").Default("120").Duration()
	tokenGenerateOutput := tokenGenerate.Flag("output", "Optional output file").String()

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

	selectedCommand := kingpin.MustParse(app.Parse(args[1:]))

	a, err := utils.ParseAddr(*authUrl)
	if err != nil {
		return err
	}
	clt, err := auth.NewClientFromNetAddr(*a)
	if err != nil {
		return err
	}

	cmd.client = clt

	switch selectedCommand {
	// Host CA
	case hostCaReset.FullCommand():
		cmd.resetHostCA(*hostCaResetConfirm)
	case hostCaPubKey.FullCommand():
		cmd.getHostCAPub()

	// User CA
	case userCaReset.FullCommand():
		cmd.resetUserCA(*userCaResetConfirm)
	case userCaPubKey.FullCommand():
		cmd.getUserCAPub()

	// Remote CA
	case remoteCaUpsert.FullCommand():
		cmd.upsertRemoteCert(*remoteCaUpsertID, *remoteCaUpsertFQDN,
			*remoteCaUpsertType, *remoteCaUpsertPath, *remoteCaUpsertTTL)
	case remoteCaLs.FullCommand():
		cmd.getRemoteCerts(*remoteCaLsFQDN, *remoteCaLsType)
	case remoteCaRm.FullCommand():
		cmd.deleteRemoteCert(*remoteCaRmID, *remoteCaRmFQDN, *remoteCaRmType)

	// Secret
	case secretNew.FullCommand():
		cmd.newKey()

	// Token
	case tokenGenerate.FullCommand():
		cmd.generateToken(*tokenGenerateFQDN, *tokenGenerateTTL,
			*tokenGenerateOutput)

	// User
	case userLs.FullCommand():
		cmd.getUsers()
	case userDelete.FullCommand():
		cmd.deleteUser(*userDeleteUser)
	case userUpsertKey.FullCommand():
		cmd.upsertKey(*userUpsertKeyUser, *userUpsertKeyKeyID,
			*userUpsertKeyKey, *userUpsertKeyTTL)
	case userLsKeys.FullCommand():
		cmd.getUserKeys(*userLsKeysUser)
	case userSetPass.FullCommand():
		cmd.setPass(*userSetPassUser, *userSetPassPass)
	}

	return nil
}

func (cmd *Command) readInput(path string) ([]byte, error) {
	if path != "" {
		return utils.ReadPath(path)
	}
	reader := bufio.NewReader(cmd.in)
	return reader.ReadSlice('\n')
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

func cut(i, j int, args []string) []string {
	s := []string{}
	s = append(s, args[:i]...)
	return append(s, args[j:]...)
}

func flags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "auth", Value: DefaultTeleportURL, Usage: "Teleport URL"},
	}
}

const DefaultTeleportURL = "unix:///tmp/teleport.auth.sock"
