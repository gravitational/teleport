package command

import (
	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
	"github.com/gravitational/teleport/tctl/command"
)

func RunCmd(cmd *command.Command, args []string) error {
	app := kingpin.New("tscopectl", "CLI for Telescope key management")
	telescopeUrl := app.Flag("telescope", "Telescope URL").Default(DefaultTelescopeURL).String()

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

	// Secret
	secret := app.Command("secret", "Operations with secret tokens")

	secretNew := secret.Command("new", "Generate new secret key")

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

	// Secret
	case secretNew.FullCommand():
		cmd.NewKey()

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
	}

	return nil
}

const DefaultTelescopeURL = "unix:///tmp/telescope.auth.sock"
