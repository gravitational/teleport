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
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

func (p *PluginsCommand) initInstallSlack(parent *kingpin.CmdClause) {
	p.install.slack.cmd = parent.Command("slack", "Install a Teleport Slack Access Request plugin.")
	cmd := p.install.slack.cmd

	cmd.Flag("app-token", "Slack app token used by the integration.").
		Required().
		StringVar(&p.install.slack.appToken)
}

// InstallSlack implements `tctl plugins install slack`, installing a Slack access
// plugin into the teleport cluster
func (p *PluginsCommand) InstallSlack(ctx context.Context, args pluginServices) error {
	pluginName := "slack-default"
	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				"teleport.dev/hosted-plugin": "true",
			},
			Name: pluginName,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_SlackAccessPlugin{
				SlackAccessPlugin: &types.PluginSlackAccessSettings{
					FallbackChannel: "#testing",
				},
			},
		},
	}

	creds, err := types.NewPluginStaticCredentials(types.Metadata{
		Name: "slack-default",
	},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: p.install.slack.appToken,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err, "validating plugin static credentials")
	}

	credsv1, ok := creds.(*types.PluginStaticCredentialsV1)
	if !ok {
		return trace.BadParameter("expected type *types.PluginStaticCredentialsV1 (this is a bug)")
	}

	req := &pluginspb.CreatePluginRequest{
		Plugin:                plugin,
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{credsv1},
	}
	if _, err := args.plugins.CreatePlugin(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Successfully installed Slack access plugin %q\n", pluginName)
	return nil

}
