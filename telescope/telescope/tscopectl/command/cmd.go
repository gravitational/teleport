package command

import (
	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
	"github.com/gravitational/teleport/tctl/command"
)

func RunCmd(cmd *command.Command, args []string) error {
	app := kingpin.New("tscopectl", "CLI for Telescope key management")
	telescopeUrl := app.Flag("telescope", "Telescope auth URL").Default(DefaultTelescopeURL).String()

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
	secretNewKeyFileName := secretNew.Flag("filename", "If filename is provided, the key will be saved to that file").Default("").String()

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

	selectedCommand := kingpin.MustParse(app.Parse(args[1:]))

	a, err := utils.ParseAddr(*telescopeUrl)
	if err != nil {
		return err
	}
	clt, err := auth.NewClientFromNetAddr(*a)
	if err != nil {
		return err
	}

	cmd.SetClient(clt)

	switch selectedCommand {
	// Host CA
	case hostCaReset.FullCommand():
		cmd.ResetHostCA(*hostCaResetConfirm)
	case hostCaPubKey.FullCommand():
		cmd.GetHostCAPub()

	// User CA
	case userCaReset.FullCommand():
		cmd.ResetUserCA(*userCaResetConfirm)
	case userCaPubKey.FullCommand():
		cmd.GetUserCAPub()

	// Remote CA
	case remoteCaUpsert.FullCommand():
		cmd.UpsertRemoteCert(*remoteCaUpsertID, *remoteCaUpsertFQDN,
			*remoteCaUpsertType, *remoteCaUpsertPath, *remoteCaUpsertTTL)
	case remoteCaLs.FullCommand():
		cmd.GetRemoteCerts(*remoteCaLsFQDN, *remoteCaLsType)
	case remoteCaRm.FullCommand():
		cmd.DeleteRemoteCert(*remoteCaRmID, *remoteCaRmFQDN, *remoteCaRmType)

	// Secret
	case secretNew.FullCommand():
		cmd.NewKey(*secretNewKeyFileName)

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

	//Backend keys
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
	}

	return nil
}

const DefaultTelescopeURL = "unix:///tmp/telescope.auth.sock"
