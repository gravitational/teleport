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
	"github.com/gravitational/teleport/api/types"
	entraapiutils "github.com/gravitational/teleport/api/utils/entraid"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/teleport/lib/utils/oidc"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

type entraArgs struct {
	cmd                  *kingpin.CmdClause
	authConnectorName    string
	defaultOwners        []string
	useSystemCredentials bool
	accessGraph          bool
	force                bool
}

func (p *PluginsCommand) initInstallEntra(parent *kingpin.CmdClause) {
	p.install.entraID.cmd = parent.Command("entraid", "Install an EntraId integration.")
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
}

type entraSettings struct {
	accessGraphCache *azureoidc.TAGInfoCache
	clientID         string
	tenantID         string
}

var (
	errCancel = trace.BadParameter("operation canceled")
)

func (p *PluginsCommand) entraSetupGuide(proxyPublicAddr string) (entraSettings, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return entraSettings{}, trace.Wrap(err)
	}
	f, err := os.CreateTemp(pwd, "entraid-setup-*.sh")
	if err != nil {
		return entraSettings{}, trace.Wrap(err, "failed to create temp file")
	}

	defer os.Remove(f.Name())

	buildScript, err := buildScript(proxyPublicAddr, p.install.entraID.authConnectorName, p.install.entraID.accessGraph, p.install.entraID.useSystemCredentials)
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

	bold := color.New(color.Bold).SprintFunc()
	boldRed := color.New(color.Bold, color.FgRed).SprintFunc()

	tmpl := `Step 1: Run the Setup Script

1. Open ` + bold("Azure Cloud Shell") + ` (Bash) using ` + bold("Google Chrome") + ` or ` + bold("Safari") + ` for the best compatibility.
2. Upload the setup script in ` + boldRed(fileLoc) + ` using the ` + bold("Upload") + ` button in the Cloud Shell toolbar.
3. Once uploaded, execute the script by running the following command:
   $ bash %s

` + bold("Important Considerations") + `:
- You must have ` + bold("Azure privileged administrator permissions") + ` to complete the integration.
- Ensure you're using the ` + bold("Bash") + ` environment in Cloud Shell.
- During the script execution, you'll be prompted to run 'az login' to authenticate with Azure. ` + bold("Teleport") + ` does not store or persist your credentials.
- ` + bold("Mozilla Firefox") + ` users may experience connectivity issues in Azure Cloud Shell; using Chrome or Safari is recommended.

`

	fmt.Fprintf(os.Stdout, tmpl, filepath.Base(fileLoc))

	op, err := readData(os.Stdin, os.Stdout,
		"Once the script completes, type 'continue' to proceed, 'exit' to quit",
		func(input string) bool {
			return input == "continue" || input == "exit"
		}, "Invalid input. Please enter 'continue' or 'exit'.")
	if err != nil {
		return entraSettings{}, trace.Wrap(err, "failed to read operation")
	}
	if op == "exit" { // User chose to exit
		return entraSettings{}, errCancel
	}

	validUUID := func(input string) bool {
		_, err := uuid.Parse(input)
		return err == nil
	}

	tmpl = `
	
Step 2: Input Tenant ID and Client ID

With the output of Step 1, please copy and paste the following information:
`
	fmt.Fprint(os.Stdout, tmpl)
	var settings entraSettings
	settings.tenantID, err = readData(os.Stdin, os.Stdout, "Enter the Tenant ID", validUUID, "Invalid Tenant ID")
	if err != nil {
		return settings, trace.Wrap(err, "failed to read Tenant ID")
	}

	settings.clientID, err = readData(os.Stdin, os.Stdout, "Enter the Client ID", validUUID, "Invalid Client ID")
	if err != nil {
		return settings, trace.Wrap(err, "failed to read Client ID")
	}

	if p.install.entraID.accessGraph {
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

func (p *PluginsCommand) InstallEntra(ctx context.Context, args installPluginArgs) error {
	inputs := p.install

	proxyPublicAddr, err := getProxyPublicAddr(ctx, args.authClient)
	if err != nil {
		return trace.Wrap(err)
	}

	settings, err := p.entraSetupGuide(proxyPublicAddr)
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

	saml, err := types.NewSAMLConnector(inputs.entraID.authConnectorName, types.SAMLConnectorSpecV2{
		AssertionConsumerService: proxyPublicAddr + "/v1/webapi/saml/acs/" + inputs.entraID.authConnectorName,
		AllowIDPInitiated:        true,
		// AttributesToRoles is required, but Entra ID does not have a default group (like Okta's "Everyone"),
		// so we add a dummy claim that will never be fulfilled with the default configuration instead,
		// and expect the user to modify it per their requirements.
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "https://example.com/my_attribute",
				Value: "my_value",
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
			if err = args.authClient.DeleteIntegration(ctx, integrationSpec.GetName()); err != nil {
				return trace.Wrap(err, "failed to delete Azure OIDC integration")
			}
			if _, err = args.authClient.CreateIntegration(ctx, integrationSpec); err != nil {
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
		if _, err = args.plugins.DeletePlugin(ctx, &pluginspb.DeletePluginRequest{
			Name: inputs.name,
		}); err != nil {
			return trace.Wrap(err)
		}
		if _, err = args.plugins.CreatePlugin(ctx, req); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Printf("Successfully created EntraID plugin %q\n\n", p.install.name)

	return nil
}

func buildScript(proxyPublicAddr string, authConnectorName string, accessGraph, skipOIDCSetup bool) (string, error) {
	oidcIssuer, err := oidc.IssuerFromPublicAddress(proxyPublicAddr, "")
	if err != nil {
		return "", trace.Wrap(err)
	}

	// The script must execute the following command:
	argsList := []string{
		"integration", "configure", "azure-oidc",
		fmt.Sprintf("--proxy-public-addr=%s", shsprintf.EscapeDefaultContext(oidcIssuer)),
		fmt.Sprintf("--auth-connector-name=%s", shsprintf.EscapeDefaultContext(authConnectorName)),
	}

	if accessGraph {
		argsList = append(argsList, "--access-graph")
	}

	if skipOIDCSetup {
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
	return proxyPublicAddr, nil
}

func readTAGCache(fileLoc string) (*azureoidc.TAGInfoCache, error) {
	if fileLoc == "" {
		return nil, trace.BadParameter("no TAG cache file found")
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
