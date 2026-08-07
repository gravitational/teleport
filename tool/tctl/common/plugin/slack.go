/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
)

type slackArgs struct {
	cmd             *kingpin.CmdClause
	botTokenFile    string
	fallbackChannel string
}

func (p *PluginsCommand) initInstallSlack(parent *kingpin.CmdClause) {
	p.install.slack.cmd = parent.Command("slack", "Install a Teleport Slack Access Request plugin.")
	cmd := p.install.slack.cmd

	cmd.Flag("name", "Name of the Slack plugin instance.").
		Default("slack-default").
		StringVar(&p.install.name)

	cmd.Flag("bot-token-file", "Slack bot token used by the plugin. Accepts path to a file containing the token.").
		Required().
		ExistingFileVar(&p.install.slack.botTokenFile)

	cmd.Flag("fallback-channel", "Slack channel where unrouted alerts should be sent. For example #access-requests.").
		Required().
		StringVar(&p.install.slack.fallbackChannel)
}

// InstallSlack implements `tctl plugins install slack`, installing a Slack access
// plugin into the teleport cluster.
func (p *PluginsCommand) InstallSlack(ctx context.Context, args pluginServices) error {
	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				types.HostedPluginLabel: "true",
			},
			Name: p.install.name,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_SlackAccessPlugin{
				SlackAccessPlugin: &types.PluginSlackAccessSettings{
					FallbackChannel: p.install.slack.fallbackChannel,
				},
			},
		},
	}

	creds, err := generateSlackCredentials(p.install.name, p.install.slack)
	if err != nil {
		return trace.Wrap(err)
	}

	req := pluginsv1.CreatePluginRequest_builder{
		Plugin:                plugin,
		StaticCredentialsList: creds,
	}.Build()

	if _, err := args.plugins.CreatePlugin(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Successfully installed Slack access plugin %q\n", p.install.name)
	return nil
}

// generateSlackCredentials generates static credentials for the Slack plugin (currently only bot token).
func generateSlackCredentials(pluginName string, slack slackArgs) ([]*types.PluginStaticCredentialsV1, error) {
	var creds []*types.PluginStaticCredentialsV1

	botTokenBytes, err := os.ReadFile(slack.botTokenFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	botToken := strings.TrimSpace(string(botTokenBytes))

	botTokenCreds, err := types.NewPluginStaticCredentials(types.Metadata{
		Name: pluginName,
	},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: botToken,
			},
		},
	)
	if err != nil {
		return nil, trace.Wrap(err, "validating plugin static credentials")
	}

	botTokenCredsv1, ok := botTokenCreds.(*types.PluginStaticCredentialsV1)
	if !ok {
		return nil, trace.BadParameter("expected type *types.PluginStaticCredentialsV1 (this is a bug)")
	}
	creds = append(creds, botTokenCredsv1)
	return creds, nil
}
