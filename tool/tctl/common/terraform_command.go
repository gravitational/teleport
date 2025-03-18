/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

const (
	terraformHelperDefaultResourcePrefix = "tctl-terraform-env-"
	terraformHelperDefaultTTL            = "1h"

	// importantText is the ANSI escape sequence used to make the terminal text bold.
	importantText = "\033[1;31m"
	// resetText is the ANSI escape sequence used to reset the terminal text style.
	resetText = "\033[0m"
)

var terraformEnvCommandLabels = map[string]string{
	common.TeleportNamespace + "/" + "created-by": "tctl-terraform-env",
}

// TerraformCommand is a tctl command providing helpers for users to run the Terraform provider.
type TerraformCommand struct {
	resourcePrefix string
	existingRole   string
	botTTL         time.Duration

	cfg *servicecfg.Config

	envCmd *kingpin.CmdClause

	// envOutput is where we write the `export env=value`, its value is os.Stdout when run via tctl, a custom buffer in tests.
	envOutput io.Writer
	// envOutput is where we write the progress updates, its value is os.Stderr run via tctl.
	userOutput io.Writer

	log *slog.Logger
}

// Initialize sets up the "tctl bots" command.
func (c *TerraformCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) {
	tfCmd := app.Command("terraform", "Helpers to run the Teleport Terraform Provider.")

	c.envCmd = tfCmd.Command("env", "Obtain certificates and load them into environments variables. This creates a temporary MachineID bot.")
	c.envCmd.Flag(
		"resource-prefix",
		fmt.Sprintf("Resource prefix to use when creating the Terraform role and bots. Defaults to [%s]", terraformHelperDefaultResourcePrefix),
	).Default(terraformHelperDefaultResourcePrefix).StringVar(&c.resourcePrefix)
	c.envCmd.Flag(
		"bot-ttl",
		fmt.Sprintf("Time-to-live of the Bot resource. The bot will be removed after this period. Defaults to [%s]", terraformHelperDefaultTTL),
	).Default(terraformHelperDefaultTTL).DurationVar(&c.botTTL)
	c.envCmd.Flag(
		"role",
		fmt.Sprintf("Role used by Terraform. The role must already exist in Teleport. When not specified, uses the default role %q", teleport.PresetTerraformProviderRoleName),
	).StringVar(&c.existingRole)

	// Save a pointer to the config to be able to recover the Debug config later
	c.cfg = cfg
}

// TryRun attempts to run subcommands.
func (c *TerraformCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	switch cmd {
	case c.envCmd.FullCommand():
		client, closeFn, err := clientFunc(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}
		err = c.RunEnvCommand(ctx, client, os.Stdout, os.Stderr)
		closeFn(ctx)
		return true, trace.Wrap(err)
	default:
		return false, nil
	}
}

// RunEnvCommand contains all the Terraform helper logic. It:
// - passes the MFA Challenge
// - creates the Terraform role
// - creates a temporary Terraform bot
// - uses the bot to obtain certificates for Terraform
// - exports certificates and Terraform configuration in environment variables
// envOutput and userOutput parameters are respectively stdout and stderr,
// except during tests where we want to catch the command output.
func (c *TerraformCommand) RunEnvCommand(ctx context.Context, client *authclient.Client, envOutput, userOutput io.Writer) error {
	// If we're not actively debugging, suppress any kind of logging from other teleport components
	if !c.cfg.Debug {
		utils.InitLogger(utils.LoggingForCLI, slog.LevelError)
	}
	c.envOutput = envOutput
	c.userOutput = userOutput
	c.log = slog.Default()

	// Validate that the bot expires
	if c.botTTL == 0 {
		return trace.BadParameter("--bot-ttl must be greater than zero")
	}

	addrs := c.cfg.AuthServerAddresses()
	if len(addrs) == 0 {
		return trace.BadParameter("no auth server addresses found")
	}
	addr := addrs[0]

	// Prompt for admin action MFA if required, allowing reuse for UpsertRole, UpsertToken and CreateBot.
	c.showProgress("🔑 Detecting if MFA is required")
	mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
	if err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	// Checking Terraform role
	roleName, err := c.checkIfRoleExists(ctx, client)
	if err != nil {
		switch {
		case trace.IsNotFound(err) && c.existingRole == "":
			return trace.Wrap(err, `The Terraform role %q does not exist in your Teleport cluster.
This default role is included in Teleport clusters whose version is higher than v16.2 or v17.
If you want to use "tctl terraform env" against an older Teleport cluster, you must create the Terraform role
yourself and set the flag --role <your-terraform-role-name>.`, roleName)
		case trace.IsNotFound(err) && c.existingRole != "":
			return trace.Wrap(err, `The Terraform role %q specified with --role does not exist in your Teleport cluster.
Please check that the role exists in the cluster.`, roleName)
		case trace.IsAccessDenied(err):
			return trace.Wrap(err, `Failed to validate if the role %q exists.
To use the "tctl terraform env" command you must have rights to list and read Teleport roles.
If you got a role granted recently, you might have to run "tsh logout" and login again.`, roleName)
		default:
			return trace.Wrap(err, "Unexpected error while trying to validate if the role %q exists.", roleName)
		}
	}

	// Create temporary bot and token
	tokenName, err := c.createTransientBotAndToken(ctx, client, roleName)
	if trace.IsAccessDenied(err) {
		return trace.Wrap(err, `Failed to create the temporary Terraform bot or its token.
To use the "tctl terraform env" command you must have rights to create Teleport bot  and token resources.
If you got a role granted recently, you might have to run "tsh logout" and login again.`)
	}
	if err != nil {
		return trace.Wrap(err, "bootstrapping bot")
	}

	// Now run tbot
	c.showProgress("🤖 Using the temporary bot to obtain certificates")
	id, sshHostCACerts, err := c.useBotToObtainIdentity(ctx, addr, tokenName, client)
	if err != nil {
		return trace.Wrap(err, "The temporary bot failed to connect to Teleport.")
	}

	envVars, err := identityToTerraformEnvVars(addr.String(), id, sshHostCACerts)
	if err != nil {
		return trace.Wrap(err, "exporting identity into environment variables")
	}

	// Export environment variables
	c.showProgress(fmt.Sprintf("🚀 Certificates obtained, you can now use Terraform in this terminal for %s", c.botTTL.String()))
	for env, value := range envVars {
		fmt.Fprintf(c.envOutput, "export %s=%q\n", env, value)
	}
	fmt.Fprintln(c.envOutput, "#")
	fmt.Fprintf(c.envOutput, "# %sYou must invoke this command in an eval: eval $(tctl terraform env)%s\n", importantText, resetText)
	return nil
}

// createTransientBotAndToken creates a Bot resource and a secret Token.
// The token is single use (secret tokens are consumed on MachineID join)
// and the bot expires after the given TTL.
func (c *TerraformCommand) createTransientBotAndToken(ctx context.Context, client *authclient.Client, roleName string) (string, error) {
	// Create token and bot name
	suffix, err := utils.CryptoRandomHex(4)
	if err != nil {
		return "", trace.Wrap(err)
	}

	botName := c.resourcePrefix + suffix
	c.showProgress(fmt.Sprintf("⚙️ Creating temporary bot %q and its token", botName))

	// Generate a token
	tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err, "generating random token")
	}
	tokenSpec := types.ProvisionTokenSpecV2{
		Roles:      types.SystemRoles{types.RoleBot},
		JoinMethod: types.JoinMethodToken,
		BotName:    botName,
	}
	// Token should be consumed on bot join in a few seconds. If the bot fails to join for any reason,
	// the token should not outlive the bot.
	token, err := types.NewProvisionTokenFromSpec(tokenName, time.Now().Add(c.botTTL), tokenSpec)
	if err != nil {
		return "", trace.Wrap(err)
	}
	token.SetLabels(terraformEnvCommandLabels)
	if err := client.UpsertToken(ctx, token); err != nil {
		return "", trace.Wrap(err, "upserting token")
	}

	// Create bot
	bot := &machineidv1pb.Bot{
		Metadata: &headerv1.Metadata{
			Name:    botName,
			Expires: timestamppb.New(time.Now().Add(c.botTTL)),
			Labels:  terraformEnvCommandLabels,
		},
		Spec: &machineidv1pb.BotSpec{
			Roles: []string{roleName},
		},
	}

	_, err = client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: bot,
	})
	if err != nil {
		return "", trace.Wrap(err, "creating bot")
	}
	return tokenName, nil
}

// roleClient describes the minimal set of operations that the helper uses to
// create the Terraform provider role.
type roleClient interface {
	UpsertRole(context.Context, types.Role) (types.Role, error)
	GetRole(context.Context, string) (types.Role, error)
}

// createRoleIfNeeded checks if the terraform role exists.
// Returns the Terraform role name even in case of error, so this can be used to craft nice error messages.
func (c *TerraformCommand) checkIfRoleExists(ctx context.Context, client roleClient) (string, error) {
	roleName := c.existingRole

	if roleName == "" {
		roleName = teleport.PresetTerraformProviderRoleName
	}
	_, err := client.GetRole(ctx, roleName)

	return roleName, trace.Wrap(err)
}

// useBotToObtainIdentity takes secret bot token and runs a one-shot in-process tbot to trade the token
// against valid certificates. Those certs are then serialized into an identity file.
// The output is a set of environment variables, one of them including the base64-encoded identity file.
// Later, the Terraform provider will read those environment variables to build its Teleport client.
// Note: the function also returns the SSH Host CA cert encoded in the known host format.
// The identity.Identity uses a different format (authorized keys).
func (c *TerraformCommand) useBotToObtainIdentity(ctx context.Context, addr utils.NetAddr, token string, clt *authclient.Client) (*identity.Identity, [][]byte, error) {
	credential := &config.UnstableClientCredentialOutput{}
	cfg := &config.BotConfig{
		Version: "",
		Onboarding: config.OnboardingConfig{
			TokenValue: token,
			JoinMethod: types.JoinMethodToken,
		},
		Storage:        &config.StorageConfig{Destination: &config.DestinationMemory{}},
		Services:       config.ServiceConfigs{credential},
		CertificateTTL: c.botTTL,
		Oneshot:        true,
		// If --insecure is passed, the bot will trust the certificate on first use.
		// This does not truly disable TLS validation, only trusts the certificate on first connection.
		Insecure: clt.Config().InsecureSkipVerify,
	}

	// When invoked only with auth address, tbot will try both joining as an auth and as a proxy.
	// This allows us to not care about how the user connects to Teleport (auth vs proxy joining).
	cfg.AuthServer = addr.String()

	// Insecure joining is not compatible with CA pinning
	if !cfg.Insecure {
		// We use the client to get the TLS CA and compute its fingerprint.
		// In case of auth joining, this ensures that tbot connects to the same Teleport auth as we do
		// (no man in the middle possible between when we build the auth client and when we run tbot).
		localCAResponse, err := clt.GetClusterCACert(ctx)
		if err != nil {
			return nil, nil, trace.Wrap(err, "getting cluster CA certificate")
		}
		caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
		if err != nil {
			return nil, nil, trace.Wrap(err, "calculating CA pins")
		}
		cfg.Onboarding.CAPins = caPins
	}

	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, nil, trace.Wrap(err, "checking the bot's configuration")
	}

	// Run the bot
	bot := tbot.New(cfg, c.log)
	err = bot.Run(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err, "running the bot")
	}

	// Retrieve the credentials obtained by tbot.
	facade, err := credential.Facade()
	if err != nil {
		return nil, nil, trace.Wrap(err, "accessing credentials")
	}

	id := facade.Get()

	clusterName, err := clt.GetClusterName()
	if err != nil {
		return nil, nil, trace.Wrap(err, "retrieving cluster name")
	}
	knownHosts, _, err := ssh.GenerateKnownHosts(ctx, clt, []string{clusterName.GetClusterName()}, addr.Host())
	if err != nil {
		return nil, nil, trace.Wrap(err, "retrieving SSH Host CA")
	}
	sshHostCACerts := [][]byte{[]byte(knownHosts)}

	return id, sshHostCACerts, nil
}

// showProgress sends status update messages ot the user.
func (c *TerraformCommand) showProgress(update string) {
	_, _ = fmt.Fprintln(c.userOutput, update)
}

// identityToTerraformEnvVars takes an identity and builds environment variables
// configuring the Terraform provider to use this identity.
// The sshHostCACerts must be in the "known hosts" format.
func identityToTerraformEnvVars(addr string, id *identity.Identity, sshHostCACerts [][]byte) (map[string]string, error) {
	idFile := &identityfile.IdentityFile{
		PrivateKey: id.PrivateKeyBytes,
		Certs: identityfile.Certs{
			SSH: id.CertBytes,
			TLS: id.TLSCertBytes,
		},
		CACerts: identityfile.CACerts{
			SSH: sshHostCACerts,
			TLS: id.TLSCACertsBytes,
		},
	}
	idBytes, err := identityfile.Encode(idFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	idBase64 := base64.StdEncoding.EncodeToString(idBytes)
	return map[string]string{
		constants.EnvVarTerraformAddress:            addr,
		constants.EnvVarTerraformIdentityFileBase64: idBase64,
	}, nil
}
