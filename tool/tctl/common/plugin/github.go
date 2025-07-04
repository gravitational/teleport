/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
)

type githubArgs struct {
	cmd       *kingpin.CmdClause
	startDate string
}

func (p *PluginsCommand) initInstallGithub(parent *kingpin.CmdClause) {
	p.install.github.cmd = parent.Command("github", "Install an Access Graph Github integration.")
	p.install.github.cmd.Flag("start-date", "Start date for the audit log ingest in the YYYY-MM-DD format.").
		Default(time.Now().Add(-10 * 24 * time.Hour).UTC().Format(time.DateOnly)).
		StringVar(&p.install.github.startDate)
}

type githubSettings struct {
	orgName        string
	clientID       string
	privateKeyData []byte
	startDate      time.Time
}

func (p *PluginsCommand) gitubSetupGuide(ctx context.Context) (githubSettings, error) {
	settings := githubSettings{}
	var err error

	settings.orgName, err = promptForInput(
		ctx,
		os.Stdout,
		"",
		"Please enter the Github Organization name",
	)
	if err != nil {
		return githubSettings{}, trace.Wrap(err)
	}

	settings.clientID, err = promptForInput(
		ctx,
		os.Stdout,
		"",
		"Please enter the Github App ClientID",
	)
	if err != nil {
		return githubSettings{}, trace.Wrap(err)
	}

	settings.startDate, err = time.Parse(time.DateOnly, p.install.github.startDate)
	if err != nil {
		return githubSettings{}, trace.Wrap(err, "failed to parse start date")
	}

	privateKey, err := promptForInput(
		ctx,
		os.Stdout,
		"",
		"Please enter the Github App Private Key file path",
	)
	if err != nil {
		return githubSettings{}, trace.Wrap(err)
	}
	privateKeyData, err := os.ReadFile(privateKey)
	if err != nil {
		return githubSettings{}, trace.Wrap(err, "failed to read private key file")
	}
	settings.privateKeyData = privateKeyData

	return settings, nil
}

func (p *PluginsCommand) InstallGithub(ctx context.Context, args installPluginArgs) error {
	settings, err := p.gitubSetupGuide(ctx)
	if err != nil {
		if errors.Is(err, errCancel) {
			return nil
		}
		return trace.Wrap(err)
	}

	plugin, err := createGithubPlugin(&settings)
	if err != nil {
		return trace.Wrap(err, "failed to create Github plugin")
	}

	creds := buildPrvKeyCredentials(settings.orgName, settings.privateKeyData)

	createPluginRequest := &pluginspb.CreatePluginRequest{
		Plugin:                plugin,
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{creds},
		CredentialLabels: map[string]string{
			"github": plugin.GetName(),
		},
	}

	if _, err = args.plugins.CreatePlugin(ctx, createPluginRequest); err != nil {
		return trace.Wrap(err, "failed to create Github plugin")
	}

	fmt.Println("Github plugin has been successfully installed.")

	return nil
}

func buildPrvKeyCredentials(orgName string, privateKey []byte) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   types.PluginTypeGithub + "-" + orgName + "-private-key",
				Labels: map[string]string{},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_PrivateKey{
				PrivateKey: privateKey,
			},
		},
	}
}

func createGithubPlugin(params *githubSettings) (*types.PluginV1, error) {
	return &types.PluginV1{
		SubKind: types.PluginSubkindAccessGraph,
		Metadata: types.Metadata{
			Labels: map[string]string{
				types.TeleportNamespace + "/hosted-plugin": "true",
			},
			Name: params.orgName,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Github{
				Github: &types.PluginGithubSettings{
					ApiEndpoint: "", /* TODO: set the API endpoint */

					ClientId:         params.clientID,
					OrganizationName: params.orgName,
					StartDate:        params.startDate,
				},
			},
		},
	}, nil
}
