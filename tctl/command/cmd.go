package command

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/utils"
	"gopkg.in/alecthomas/kingpin.v2"
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
	hostca := app.Command("host-ca", "Operations with host certificate authority")

	hostcaReset := hostca.Command("reset", "Reset host certificate authority keys")
	hostcaResetConfirm := hostcaReset.Flag("confirm", "Automatically apply the operation without confirmation").Bool()

	hostcaPubkey := hostca.Command("pub-key", "print host certificate authority public key")

	// User CA
	userca := app.Command("user-ca", "Operations with user certificate authority")

	usercaReset := userca.Command("reset", "Reset user certificate authority keys")
	usercaResetConfirm := usercaReset.Flag("confirm", "Automatically apply the operation without confirmation").Bool()

	usercaPubkey := userca.Command("pub-key", "Print user certificate authority public key")

	// Remote CA
	remoteca := app.Command("remote-ca", "Operations with remote certificate authority")

	remotecaUpsert := remoteca.Command("upsert", "Upsert remote certificate to trust")
	remotecaUpsertId := remotecaUpsert.Flag("id", "Certificate id").Required().String()
	remotecaUpsertFqdn := remotecaUpsert.Flag("fqdn", "FQDN of the remote party").Required().String()
	remotecaUpsertType := remotecaUpsert.Flag("type", "Cert type (host or user)").Required().String()
	remotecaUpsertPath := remotecaUpsert.Flag("path", "Cert path (reads from stdout if omitted)").Required().ExistingFile()
	remotecaUpsertTtl := remotecaUpsert.Flag("ttl", "ttl for certificate to be trusted").Required().Duration()

	remotecaLs := remoteca.Command("ls", "List trusted remote certificates")
	remotecaLsFqdn := remotecaLs.Flag("fqdn", "FQDN of the remote party").Required().String()
	remotecaLsType := remotecaLs.Flag("type", "Cert type (host or user)").Required().String()

	remotecaRm := remoteca.Command("rm", "Remote remote CA from list of trusted certs")
	remotecaRmId := remotecaRm.Flag("id", "Certificate id").Required().String()
	remotecaRmFqdn := remotecaRm.Flag("fqdn", "FQDN of the remote party").Required().String()
	remotecaRmType := remotecaRm.Flag("type", "Cert type (host or user)").Required().String()

	// Secret
	secret := app.Command("secret", "Operations with secret tokens")

	secretNew := secret.Command("new", "Generate new secret key")

	// Token
	token := app.Command("token", "Generates provisioning tokens")

	tokenGenerate := token.Command("generate", "Generate provisioning token for server with fqdn")
	tokenGenerateFqdn := tokenGenerate.Flag("fqdn", "FQDN of the server").Required().String()
	tokenGenerateTtl := tokenGenerate.Flag("ttl", "Time to live").Default("120").Duration()
	tokenGenerateOutput := tokenGenerate.Flag("output", "Optional output file").String()

	// User
	user := app.Command("user", "Operations with registered users")

	userLs := user.Command("ls", "List users registered in teleport")

	userDelete := user.Command("delete", "Delete user")
	userDeleteUser := userDelete.Flag("user", "User to delete").Required().String()

	userUpsertkey := user.Command("upsert-key", "Grant access to the user key, returns signed certificate")
	userUpsertkeyUser := userUpsertkey.Flag("user", "User holding the key").Required().String()
	userUpsertkeyKeyid := userUpsertkey.Flag("key-id", "SSH key ID").Required().String()
	userUpsertkeyKey := userUpsertkey.Flag("key", "Path to public key").Required().ExistingFile()
	userUpsertkeyTtl := userUpsertkey.Flag("ttl", "Access time to live, certificate and access entry will expire when set").Required().Duration()

	userLskeys := user.Command("ls-keys", "List user's keys registered in teleport")
	userLskeysUser := userLskeys.Flag("user", "User to list keys form").Required().String()

	userSetpass := user.Command("set-pass", "Set user password")
	userSetpassUser := userSetpass.Flag("user", "User name").Required().String()
	userSetpassPass := userSetpass.Flag("pass", "Password").Required().String()

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
	case hostcaReset.FullCommand():
		cmd.resetHostCA(*hostcaResetConfirm)
	case hostcaPubkey.FullCommand():
		cmd.getHostCAPub()

	// User CA
	case usercaReset.FullCommand():
		cmd.resetUserCA(*usercaResetConfirm)
	case usercaPubkey.FullCommand():
		cmd.getUserCAPub()

	// Remote CA
	case remotecaUpsert.FullCommand():
		cmd.upsertRemoteCert(*remotecaUpsertId, *remotecaUpsertFqdn,
			*remotecaUpsertType, *remotecaUpsertPath, *remotecaUpsertTtl)
	case remotecaLs.FullCommand():
		cmd.getRemoteCerts(*remotecaLsFqdn, *remotecaLsType)
	case remotecaRm.FullCommand():
		cmd.deleteRemoteCert(*remotecaRmId, *remotecaRmFqdn, *remotecaRmType)

	// Secret
	case secretNew.FullCommand():
		cmd.newKey()

	// Token
	case tokenGenerate.FullCommand():
		cmd.generateToken(*tokenGenerateFqdn, *tokenGenerateTtl,
			*tokenGenerateOutput)

	// User
	case userLs.FullCommand():
		cmd.getUsers()
	case userDelete.FullCommand():
		cmd.deleteUser(*userDeleteUser)
	case userUpsertkey.FullCommand():
		cmd.upsertKey(*userUpsertkeyUser, *userUpsertkeyKeyid,
			*userUpsertkeyKey, *userUpsertkeyTtl)
	case userLskeys.FullCommand():
		cmd.getUserKeys(*userLskeysUser)
	case userSetpass.FullCommand():
		cmd.setPass(*userSetpassUser, *userSetpassPass)
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
