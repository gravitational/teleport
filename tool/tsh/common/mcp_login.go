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

package common

import (
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/client"
	mcpoauth "github.com/gravitational/teleport/lib/client/mcp/oauth"
	"github.com/gravitational/teleport/lib/client/sso"
)

// mcpLoginCommand implements `tsh mcp login`. It runs the interactive OAuth
// ceremony for an OAuth-protected HTTP MCP server and stores the resulting
// tokens so `tsh mcp connect` can inject and silently refresh them.
type mcpLoginCommand struct {
	*kingpin.CmdClause
	cf *CLIConf
}

func newMCPLoginCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpLoginCommand {
	cmd := &mcpLoginCommand{
		CmdClause: parent.Command("login", "Authorize an OAuth-protected MCP server."),
		cf:        cf,
	}
	cmd.Arg("name", "Name of the MCP server.").Required().StringVar(&cf.AppName)
	return cmd
}

func (c *mcpLoginCommand) run() error {
	ctx := c.cf.Context
	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	dialer := client.NewMCPServerDialer(tc, c.cf.AppName)
	var app types.Application
	if err := client.RetryWithRelogin(ctx, tc, func() error {
		app, err = dialer.GetApp(ctx)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

	upstreamURL, err := mcpoauth.UpstreamURL(app)
	if err != nil {
		return trace.BadParameter("OAuth login applies only to HTTP MCP servers; %q uses %q transport", app.GetName(), types.GetMCPServerTransportType(app.GetURI()))
	}

	httpClient, err := mcpoauth.NewHTTPClient(
		func(context.Context) (string, error) { return upstreamURL.Host, nil },
		dialer.DialALPN,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.cf.Stdout(), "Opening browser for authorization of MCP server %q...\n", app.GetName())
	creds, err := mcpoauth.RunLoginCeremony(ctx, mcpoauth.CeremonyConfig{
		UpstreamURL: upstreamURL,
		HTTPClient:  httpClient,
		OpenURL: func(authURL string) error {
			fmt.Fprintf(c.cf.Stdout(), "If the browser does not open automatically, visit:\n %v\n", authURL)
			if err := sso.OpenURLInBrowser(c.cf.Browser, authURL); err != nil {
				fmt.Fprintf(c.cf.Stderr(), "Failed to open a browser: %v\n", err)
			}
			return nil
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	store := mcpoauth.NewStore(
		keypaths.MCPOAuthCredentialPath(profile.Dir, profile.Name, profile.Username, tc.SiteName, app.GetName()),
		keypaths.MCPOAuthLockPath(profile.Dir, profile.Name, profile.Username, tc.SiteName, app.GetName()),
		nil,
	)
	if err := store.SaveLocked(ctx, creds); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintln(c.cf.Stdout(), "Authorization complete. Tokens stored.")
	fmt.Fprintf(c.cf.Stdout(), "MCP server %q is ready; restart your MCP clients if already running.\n", app.GetName())
	return nil
}
