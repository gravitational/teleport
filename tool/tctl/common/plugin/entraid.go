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

package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/fatih/color"
	"github.com/google/safetext/shsprintf"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	entraapiutils "github.com/gravitational/teleport/api/utils/entraid"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/teleport/lib/utils/oidc"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

var (
	bold    = color.New(color.Bold).SprintFunc()
	boldRed = color.New(color.Bold, color.FgRed).SprintFunc()

	step1Template = bold("Step 1: Run the Setup Script") + `

1. Open ` + bold("Azure Cloud Shell") + ` (Bash) [https://portal.azure.com/#cloudshell/] using ` + bold("Google Chrome") + ` or ` + bold("Safari") + ` for the best compatibility.
2. Upload the setup script in ` + boldRed("%s") + ` using the ` + bold("Upload") + ` button in the Cloud Shell toolbar.
3. Once uploaded, execute the script by running the following command:
   $ bash %s

` + bold("Important Considerations") + `:
- You must have ` + bold("Azure privileged administrator permissions") + ` to complete the integration.
- Ensure you're using the ` + bold("Bash") + ` environment in Cloud Shell.
- During the script execution, you'll be prompted to run 'az login' to authenticate with Azure. ` + bold("Teleport") + ` does not store or persist your credentials.
- ` + bold("Mozilla Firefox") + ` users may experience connectivity issues in Azure Cloud Shell; using Chrome or Safari is recommended.

To rerun the script, type 'exit' to close and then restart the process.

`

	step2Template = `

` + bold("Step 2: Input Tenant ID and Client ID") + `

With the output of Step 1, please copy and paste the following information:
`

	manualConfigurationTemplate = `

` + bold("Step 1: Input Tenant ID and Client ID") + `

To finish the Entra ID integration, manually configure the Application in Microsoft Entra ID.

Follow the instructions provided in the Teleport documentation: [https://goteleport.com/docs/admin-guides/teleport-policy/integrations/entra-id/].

After completing the Entra ID setup, copy and paste the following information:
`
)

type entraArgs struct {
	cmd                  *kingpin.CmdClause
	authConnectorName    string
	defaultOwners        []string
	useSystemCredentials bool
	accessGraph          bool
	force                bool
	manualEntraIDSetup   bool
}

func (p *PluginsCommand) initInstallEntra(parent *kingpin.CmdClause) {
	p.install.entraID.cmd = parent.Command("entraid", "Install an Microsoft Entra ID integration.")
	cmd := p.install.entraID.cmd
	cmd.
		Flag("name", "Name of the plugin resource to create").
		Default("entra-id").
		StringVar(&p.install.name)

	cmd.
		Flag("auth-connector-name", "Name of the SAML connector resource to create").
		Default("entra-id-default").
		StringVar(&p.install.entraID.authConnectorName)

	cmd.
		Flag("use-system-credentials", "Uses system credentials instead of OIDC.").
		BoolVar(&p.install.entraID.useSystemCredentials)

	cmd.Flag("default-owner", "List of Teleport users that are default owners for the imported access lists. Multiple flags allowed.").
		Required().
		StringsVar(&p.install.entraID.defaultOwners)

	cmd.
		Flag("access-graph", "Enables Access Graph cache build.").
		Default("true").
		BoolVar(&p.install.entraID.accessGraph)

	cmd.
		Flag("force", "Proceed with installation even if plugin already exists.").
		Short('f').
		Default("false").
		BoolVar(&p.install.entraID.force)

	cmd.
		Flag("manual-setup", "Manually set up the EntraID integration.").
		Short('m').
		Default("false").
		BoolVar(&p.install.entraID.manualEntraIDSetup)
}

type entraSettings struct {
	accessGraphCache *azureoidc.TAGInfoCache
	clientID         string
	tenantID         string
}

var errCancel = trace.BadParameter("operation canceled")

func (p *PluginsCommand) entraSetupGuide(proxyPublicAddr string, manualEntraIDSetup bool) (entraSettings, error) {
	if manualEntraIDSetup {
		fmt.Fprint(os.Stdout, manualConfigurationTemplate)
		settings, err := readAzureInputs(p.install.entraID.accessGraph)
		return settings, trace.Wrap(err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return entraSettings{}, trace.Wrap(err, "failed to get working dir")
	}
	f, err := os.CreateTemp(pwd, "entraid-setup-*.sh")
	if err != nil {
		return entraSettings{}, trace.Wrap(err, "failed to create temp file")
	}

	defer os.Remove(f.Name())

	buildScript, err := buildScript(proxyPublicAddr, p.install.entraID)
	if err != nil {
		return entraSettings{}, trace.Wrap(err, "failed to build script")
	}

	if _, err := f.Write([]byte(buildScript)); err != nil {
		return entraSettings{}, trace.Wrap(err, "failed to write script to file")
	}

	if err := f.Close(); err != nil {
		return entraSettings{}, trace.Wrap(err, "failed to close file")
	}
	fileLoc := f.Name()

	fmt.Fprintf(os.Stdout, step1Template, fileLoc, filepath.Base(fileLoc))

	op, err := readData(os.Stdin, os.Stdout,
		`Once the script completes, type 'continue' to proceed, 'exit' to quit`,
		func(input string) bool {
			return input == "continue" || input == "exit"
		}, "Invalid input. Please enter 'continue' or 'exit'.")
	if err != nil {
		return entraSettings{}, trace.Wrap(err, "failed to read operation")
	}
	if op == "exit" { // User chose to exit
		return entraSettings{}, errCancel
	}

	fmt.Fprint(os.Stdout, step2Template)

	settings, err := readAzureInputs(p.install.entraID.accessGraph)
	return settings, trace.Wrap(err)
}

func readAzureInputs(acessGraph bool) (entraSettings, error) {
	validUUID := func(input string) bool {
		_, err := uuid.Parse(input)
		return err == nil
	}
	var settings entraSettings
	var err error
	settings.tenantID, err = readData(os.Stdin, os.Stdout, "Enter the Tenant ID", validUUID, "Invalid Tenant ID")
	if err != nil {
		return settings, trace.Wrap(err, "failed to read Tenant ID")
	}

	settings.clientID, err = readData(os.Stdin, os.Stdout, "Enter the Client ID", validUUID, "Invalid Client ID")
	if err != nil {
		return settings, trace.Wrap(err, "failed to read Client ID")
	}

	if acessGraph {
		dataValidator := func(input string) bool {
			settings.accessGraphCache, err = readTAGCache(input)
			return err == nil
		}
		_, err = readData(os.Stdin, os.Stdout, "Enter the Access Graph Cache file location", dataValidator, "File does not exist or is invalid")
		if err != nil {
			return settings, trace.Wrap(err, "failed to read Access Graph Cache file")
		}
	}
	return settings, nil
}

// InstallEntra is the entry point for the `tctl plugins install entraid` command.
// This function guides users through an interactive setup process to configure EntraID integration,
// directing them to execute a script in Azure Cloud Shell and provide the required configuration inputs.
// The script creates an Azure EntraID Enterprise Application, enabling SAML logins in Teleport with
// the following claims:
// - givenname: user.givenname
// - surname: user.surname
// - emailaddress: user.mail
// - name: user.userprincipalname
// - groups: user.groups
// Additionally, the script establishes a Trust Policy in the application to allow Teleport
// to be recognized as a credential issuer when system credentials are not used.
// If system credentials are present, the script will skip the Trust policy creation using
// system credentials for EntraID authentication.
// Finally, if no system credentials are in use, the script will set up an Azure OIDC integration
// in Teleport and a Teleport plugin to synchronize access lists from EntraID to Teleport.
func (p *PluginsCommand) InstallEntra(ctx context.Context, args installPluginArgs) error {
	inputs := p.install

	proxyPublicAddr, err := getProxyPublicAddr(ctx, args.authClient)
	if err != nil {
		return trace.Wrap(err)
	}

	settings, err := p.entraSetupGuide(proxyPublicAddr, inputs.entraID.manualEntraIDSetup)
	if err != nil {
		if errors.Is(err, errCancel) {
			return nil
		}
		return trace.Wrap(err)
	}

	var tagSyncSettings *types.PluginEntraIDAccessGraphSettings
	if settings.accessGraphCache != nil {
		tagSyncSettings = &types.PluginEntraIDAccessGraphSettings{
			AppSsoSettingsCache: settings.accessGraphCache.AppSsoSettingsCache,
		}
	}

	// Prompt for admin action MFA if required, allowing reuse for UpsertToken and CreateBot.
	mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, args.authClient.PerformMFACeremony, true /*allowReuse*/)
	if err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	saml, err := types.NewSAMLConnector(inputs.entraID.authConnectorName, types.SAMLConnectorSpecV2{
		AssertionConsumerService: strings.TrimRight(proxyPublicAddr, "/") + "/v1/webapi/saml/acs/" + inputs.entraID.authConnectorName,
		AllowIDPInitiated:        true,
		// AttributesToRoles is required, but Entra ID does not have a default group (like Okta's "Everyone"),
		// so we add a dummy claim that will always be fulfilled and map them to the "requester" role.
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
				Value: "*",
				Roles: []string{"requester"},
			},
		},
		Display:             "Entra ID",
		EntityDescriptorURL: entraapiutils.FederationMetadataURL(settings.tenantID, settings.clientID),
	})
	if err != nil {
		return trace.Wrap(err, "failed to create SAML connector")
	}

	if _, err = args.authClient.CreateSAMLConnector(ctx, saml); err != nil {
		if !trace.IsAlreadyExists(err) || !inputs.entraID.force {
			return trace.Wrap(err, "failed to create SAML connector")
		}
		if _, err = args.authClient.UpsertSAMLConnector(ctx, saml); err != nil {
			return trace.Wrap(err, "failed to upsert SAML connector")
		}
	}

	if !inputs.entraID.useSystemCredentials {
		integrationSpec, err := types.NewIntegrationAzureOIDC(
			types.Metadata{Name: inputs.name},
			&types.AzureOIDCIntegrationSpecV1{
				TenantID: settings.tenantID,
				ClientID: settings.clientID,
			},
		)
		if err != nil {
			return trace.Wrap(err, "failed to create Azure OIDC integration")
		}

		if _, err = args.authClient.CreateIntegration(ctx, integrationSpec); err != nil {
			if !trace.IsAlreadyExists(err) || !inputs.entraID.force {
				return trace.Wrap(err, "failed to create Azure OIDC integration")
			}

			integration, err := args.authClient.GetIntegration(ctx, integrationSpec.GetName())
			if err != nil {
				return trace.Wrap(err, "failed to get Azure OIDC integration")
			}
			integration.SetAWSOIDCIntegrationSpec(integrationSpec.GetAWSOIDCIntegrationSpec())
			if _, err = args.authClient.UpdateIntegration(ctx, integration); err != nil {
				return trace.Wrap(err, "failed to create Azure OIDC integration")
			}
		}
	}

	credentialsSource := types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC
	if inputs.entraID.useSystemCredentials {
		credentialsSource = types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS
	}
	req := &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Metadata: types.Metadata{
				Name: inputs.name,
				Labels: map[string]string{
					"teleport.dev/hosted-plugin": "true",
				},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_EntraId{
					EntraId: &types.PluginEntraIDSettings{
						SyncSettings: &types.PluginEntraIDSyncSettings{
							DefaultOwners:     inputs.entraID.defaultOwners,
							SsoConnectorId:    inputs.entraID.authConnectorName,
							CredentialsSource: credentialsSource,
							TenantId:          settings.tenantID,
							EntraAppId:        settings.clientID,
						},
						AccessGraphSettings: tagSyncSettings,
					},
				},
			},
		},
	}

	_, err = args.plugins.CreatePlugin(ctx, req)
	if err != nil {
		if !trace.IsAlreadyExists(err) || !inputs.entraID.force {
			return trace.Wrap(err)
		}
		plugin := req.GetPlugin()
		{
			oldPlugin, err := args.plugins.GetPlugin(ctx, &pluginspb.GetPluginRequest{
				Name: inputs.name,
			})
			if err != nil {
				return trace.Wrap(err)
			}
			plugin.Metadata.Revision = oldPlugin.GetMetadata().Revision
		}
		if _, err = args.plugins.UpdatePlugin(ctx, &pluginspb.UpdatePluginRequest{
			Plugin: plugin,
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Printf("Successfully created EntraID plugin %q\n\n", p.install.name)

	return nil
}

func buildScript(proxyPublicAddr string, entraCfg entraArgs) (string, error) {
	// The script must execute the following command:
	argsList := []string{
		"integration", "configure", "azure-oidc",
		fmt.Sprintf("--proxy-public-addr=%s", shsprintf.EscapeDefaultContext(proxyPublicAddr)),
		fmt.Sprintf("--auth-connector-name=%s", shsprintf.EscapeDefaultContext(entraCfg.authConnectorName)),
	}

	if entraCfg.accessGraph {
		argsList = append(argsList, "--access-graph")
	}

	if entraCfg.useSystemCredentials {
		argsList = append(argsList, "--skip-oidc-integration")
	}

	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to use the integration with Azure.",
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return script, nil
}

func getProxyPublicAddr(ctx context.Context, authClient authClient) (string, error) {
	pingResp, err := authClient.Ping(ctx)
	if err != nil {
		return "", trace.Wrap(err, "failed fetching cluster info")
	}
	proxyPublicAddr := pingResp.GetProxyPublicAddr()
	oidcIssuer, err := oidc.IssuerFromPublicAddress(proxyPublicAddr, "")
	return oidcIssuer, trace.Wrap(err)
}

// readTAGCache reads the TAG cache file and returns the TAGInfoCache object.
// azureoidc.TAGInfoCache is a struct that contains the information necessary for Access Graph to analyze Azure SSO.
// It contains a list of AppID and their corresponding FederatedSsoV2 information.
func readTAGCache(fileLoc string) (*azureoidc.TAGInfoCache, error) {
	if fileLoc == "" {
		return nil, trace.BadParameter("no TAG cache file specified")
	}

	file, err := os.Open(fileLoc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()

	var result azureoidc.TAGInfoCache
	if err := json.NewDecoder(file).Decode(&result); err != nil {
		return nil, trace.Wrap(err)
	}

	return &result, nil
}

func readData(r io.Reader, w io.Writer, message string, validate func(string) bool, errorMessage string) (string, error) {
	reader := bufio.NewReader(r)
	for {
		fmt.Fprintf(w, "%s: ", message)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input) // Clean up any extra newlines or spaces

		if !validate(input) {
			fmt.Fprintf(w, "%s\n", errorMessage)
			continue
		}
		return input, nil
	}
}
