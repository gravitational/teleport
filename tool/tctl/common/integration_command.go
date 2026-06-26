/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// IntegrationCommand implements `tctl integration`.
type IntegrationCommand struct {
	config *servicecfg.Config

	testCmd           *kingpin.CmdClause
	testIntegration   string
	testBrowser       string
}

// Initialize sets up the command.
func (cmd *IntegrationCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) {
	cmd.config = cfg

	integration := app.Command("integration", "Manage integrations.")
	cmd.testCmd = integration.Command("test", "Test a GitHub integration by performing the OAuth flow without saving credentials.")
	cmd.testCmd.Arg("name", "Name of the GitHub integration to test (or the GitHub organization name).").Required().StringVar(&cmd.testIntegration)
	cmd.testCmd.Flag("browser", "Set to 'none' to suppress browser opening.").StringVar(&cmd.testBrowser)
}

// TryRun attempts to run the command.
func (cmd *IntegrationCommand) TryRun(ctx context.Context, selectedCommand string, clientFunc commonclient.InitFunc) (match bool, err error) {
	switch selectedCommand {
	case cmd.testCmd.FullCommand():
		authClient, closeFn, err := clientFunc(ctx)
		if err != nil {
			return true, trace.Wrap(err)
		}
		defer closeFn(ctx)
		return true, trace.Wrap(cmd.runTest(ctx, authClient))
	default:
		return false, nil
	}
}

func (cmd *IntegrationCommand) runTest(ctx context.Context, c *authclient.Client) error {
	// Resolve the integration -- accept either integration name or org name.
	ig, err := c.GetIntegration(ctx, cmd.testIntegration)
	if err != nil {
		return trace.Wrap(err, "integration %q not found", cmd.testIntegration)
	}

	if ig.GetSubKind() != types.IntegrationSubKindGitHub {
		return trace.BadParameter("integration %q is not a GitHub integration (got %q)", ig.GetName(), ig.GetSubKind())
	}

	spec := ig.GetGitHubIntegrationSpec()
	if spec == nil {
		return trace.BadParameter("integration %q has no GitHub spec", ig.GetName())
	}

	fmt.Printf("Testing GitHub integration %q (org: %s)\n", ig.GetName(), spec.Organization)
	fmt.Printf("Protocols: %s\n\n", strings.Join(spec.AllowProtocols, ", "))

	rd, err := sso.NewRedirector(sso.RedirectorConfig{
		ProxyAddr: cmd.proxyAddr(ctx, c),
		Browser:   cmd.testBrowser,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer rd.Close()

	var stateToken string
	initSSO := func(ctx context.Context, clientCallbackURL string) (string, error) {
		authReq, err := c.GitServerClient().CreateGitHubAuthRequest(ctx, &types.GithubAuthRequest{
			SSOTestFlow:       true,
			ClientRedirectURL: clientCallbackURL,
		}, spec.Organization)
		if err != nil {
			return "", trace.Wrap(err, "failed to create auth request")
		}
		stateToken = authReq.StateToken
		fmt.Println("[1/3] Auth request created. Starting OAuth flow...")
		return authReq.RedirectURL, nil
	}

	ceremony := sso.NewCLICeremony(rd, initSSO)
	loginResp, err := ceremony.Run(ctx)
	if err != nil {
		fmt.Println("[2/3] OAuth flow FAILED")
		return trace.Wrap(err, "OAuth flow failed")
	}

	fmt.Printf("[2/3] OAuth flow completed. Logged in as: %s\n", loginResp.Username)

	diag, err := c.GetSSODiagnosticInfo(ctx, constants.Github, stateToken)
	if err != nil {
		fmt.Printf("[3/3] Could not retrieve diagnostic info: %v\n", err)
		return nil
	}

	if diag.Success {
		fmt.Println("[3/3] Integration test PASSED")
	} else {
		fmt.Println("[3/3] Integration test FAILED")
	}

	fmt.Println()
	if diag.GithubClaims != nil {
		fmt.Printf("GitHub username: %s\n", diag.GithubClaims.Username)
	}
	if diag.GithubTokenInfo != nil {
		fmt.Printf("Token type: %s\n", diag.GithubTokenInfo.TokenType)
	}
	if diag.Error != "" {
		fmt.Printf("Error: %s\n", diag.Error)
	}

	fmt.Println()
	fmt.Println("Note: no credentials were saved. Run 'tsh git login' to set up credentials for use.")

	if !diag.Success {
		return trace.Errorf("integration test failed")
	}
	return nil
}

func (cmd *IntegrationCommand) proxyAddr(ctx context.Context, c *authclient.Client) string {
	proxies, err := c.GetProxies()
	if err != nil || len(proxies) == 0 {
		return ""
	}
	return proxies[0].GetPublicAddr()
}
