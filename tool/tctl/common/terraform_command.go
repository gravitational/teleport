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
	"github.com/gravitational/teleport/api/types/common"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	terraformHelperDefaultResourcePrefix = "terraform-env-"
	terraformHelperDefaultTTL            = "1h"
)

var terraformRoleSpec = types.RoleSpecV6{
	Allow: types.RoleConditions{
		AppLabels:      map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
		DatabaseLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
		NodeLabels:     map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
		Rules: []types.Rule{
			{
				Resources: []string{
					types.KindAccessList,
					types.KindApp,
					types.KindClusterAuthPreference,
					types.KindClusterMaintenanceConfig,
					types.KindClusterNetworkingConfig,
					types.KindDatabase,
					types.KindDevice,
					types.KindGithub,
					types.KindLoginRule,
					types.KindNode,
					types.KindOIDC,
					types.KindOktaImportRule,
					types.KindRole,
					types.KindSAML,
					types.KindSessionRecordingConfig,
					types.KindToken,
					types.KindTrustedCluster,
					types.KindUser,
				},
				Verbs: []string{types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete},
			},
		},
	},
}

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
}

// Initialize sets up the "tctl bots" command.
func (c *TerraformCommand) Initialize(app *kingpin.Application, cfg *servicecfg.Config) {
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
		"use-existing-role",
		"Existing Terraform role to use instead of creating a new one.",
	).StringVar(&c.existingRole)

	// Save a pointer to the config to be able to recover the Debug config later
	c.cfg = cfg
}

// TryRun attempts to run subcommands.
func (c *TerraformCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case c.envCmd.FullCommand():
		err = c.RunEnvCommand(ctx, client, os.Stdout, os.Stderr)
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
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
	log := slog.Default()

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
	c.showProgress("Detecting if MFA is required")
	mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
	if err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	// Upsert Terraform role
	roleName, err := c.createRoleIfNeeded(ctx, client, log)
	if trace.IsAccessDenied(err) {
		return trace.Wrap(err, `Failed to create/update the Terraform role.
You must have the rights to get/create/update roles to rely on automatic role creation.
If you don't have those rights, you can set the flag --use-existing-role "your-existing-terraform-role".
If you got a role granted recently, you might have to run "tsh logout" and login again.`)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// Create temporary bot and token
	tokenName, err := c.createTransientBotAndToken(ctx, client, roleName)
	if trace.IsAccessDenied(err) {
		return trace.Wrap(err, `Failed to create the temporary Terraform bot.
To use the "tctl terraform env" command you must have rights to create Teleport bot resources.
If you got a role granted recently, you might have to run "tsh logout" and login again.`)
	}
	if err != nil {
		return trace.Wrap(err, "bootstrapping bot")
	}

	// Now run tbot
	c.showProgress("Using the temporary bot to obtain certificates 🤖")
	id, err := c.useBotToObtainIdentity(ctx, addr, tokenName, client, log)
	if err != nil {
		return trace.Wrap(err, "The temporary bot failed to connect to Teleport.")
	}

	envVars, err := identityToTerraformEnvVars(addr.String(), id)
	if err != nil {
		return trace.Wrap(err, "exporting identity into environment variables")
	}

	// Export environment variables
	c.showProgress("Certificates obtained, you can now use Terraform in this terminal 🚀")
	for env, value := range envVars {
		fmt.Fprintf(c.envOutput, "export %s=%q\n", env, value)
	}
	fmt.Fprintln(c.envOutput, "# You must invoke this command in an eval: eval $(tctl terraform env)")
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
	c.showProgress(fmt.Sprintf("Creating temporary bot %q and its token", botName))

	var token types.ProvisionToken

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
	token, err = types.NewProvisionTokenFromSpec(tokenName, time.Now().Add(c.botTTL), tokenSpec)
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

	bot, err = client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
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

// createRoleIfNeeded upserts the Terraform role, or checks if the role exists.
// Returns the Terraform role name.
func (c *TerraformCommand) createRoleIfNeeded(ctx context.Context, client roleClient, log *slog.Logger) (string, error) {
	roleName := c.existingRole

	// If roleName is specified, we don't attempt to create the role but we still check that it exists.
	if roleName != "" {
		_, err := client.GetRole(ctx, roleName)
		if trace.IsNotFound(err) {
			log.ErrorContext(ctx, "Role not found", "role", roleName)
			return "", trace.Wrap(err)
		} else if err != nil {
			return "", trace.Wrap(err, "getting role")
		}

		log.InfoContext(ctx, "Using existing Terraform role", "role", roleName)
		c.showProgress(fmt.Sprintf("Using existing Terraform Provider role: %q", roleName))
		return roleName, nil
	}

	roleName = c.resourcePrefix + "provider"
	log.InfoContext(ctx, "Creating/Updating the Terraform Provider role", "role", roleName)

	role, err := types.NewRole(roleName, terraformRoleSpec)
	if err != nil {
		return "", trace.Wrap(err)
	}
	role.SetStaticLabels(terraformEnvCommandLabels)

	_, err = client.UpsertRole(ctx, role)
	if err != nil {
		return "", trace.Wrap(err, "upserting role")
	}
	c.showProgress(fmt.Sprintf("Created Terraform Provider role: %q", roleName))

	return roleName, nil
}

// useBotToObtainIdentity takes secret bot token and runs a one-shot in-process tbot to trade the token
// against valid certificates. Those certs are then serialized into an identity file.
// The output is a set of environment variables, one of them including the base64-encoded identity file.
// Later, the Terraform provider will read those environment variables to build its Teleport client.
func (c *TerraformCommand) useBotToObtainIdentity(ctx context.Context, addr utils.NetAddr, token string, clt *authclient.Client, log *slog.Logger) (*identity.Identity, error) {
	credential := &config.UnstableClientCredentialOutput{}
	cfg := &config.BotConfig{
		Version: "",
		Onboarding: config.OnboardingConfig{
			TokenValue: token,
			JoinMethod: types.JoinMethodToken,
		},
		Storage:        &config.StorageConfig{Destination: &config.DestinationMemory{}},
		Outputs:        []config.Output{credential},
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
			return nil, trace.Wrap(err, "getting cluster CA certificate")
		}
		caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
		if err != nil {
			return nil, trace.Wrap(err, "calculating CA pins")
		}
		cfg.Onboarding.CAPins = caPins
	}

	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err, "checking the bot's configuration")
	}

	// Run the bot
	bot := tbot.New(cfg, log)
	err = bot.Run(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "running the bot")
	}

	// Retrieve the credentials obtained by tbot.
	facade, err := credential.Facade()
	if err != nil {
		return nil, trace.Wrap(err, "accessing credentials")
	}

	id := facade.Get()

	clusterName, err := clt.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err, "retrieving cluster name")
	}
	knownHosts, err := ssh.GenerateKnownHosts(ctx, clt, []string{clusterName.GetClusterName()}, addr.Host())
	if err != nil {
		return nil, trace.Wrap(err, "retrieving SSH Host CA")
	}
	id.SSHCACertBytes = [][]byte{
		[]byte(knownHosts),
	}
	// End of workaround

	return id, nil
}

// showProgress sends status update messages ot the user.
func (c *TerraformCommand) showProgress(update string) {
	_, _ = fmt.Fprintln(c.userOutput, update)
}

// identityToTerraformEnvVars takes an identity and builds environment variables
// configuring the Terraform provider to use this identity.
func identityToTerraformEnvVars(addr string, id *identity.Identity) (map[string]string, error) {
	idFile := &identityfile.IdentityFile{
		PrivateKey: id.PrivateKeyBytes,
		Certs: identityfile.Certs{
			SSH: id.CertBytes,
			TLS: id.TLSCertBytes,
		},
		CACerts: identityfile.CACerts{
			SSH: id.SSHCACertBytes,
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
