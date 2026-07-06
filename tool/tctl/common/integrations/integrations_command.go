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

package integrations

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// Initialize registers the `tctl integrations` command group.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	integrationsCmd := app.Command("integrations", "Integrations helpers and test utilities.")
	integrationsCmd.Alias("integration")

	c.testCmd = integrationsCmd.Command("test", "Verify an integration.")
	c.testCmd.Arg("integration", "Name of the integration to test. Use `tctl get integrations` to list integrations.").Required().StringVar(&c.testArgs.integration)
	c.testCmd.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.testArgs.format, teleport.Text, teleport.JSON, teleport.YAML)

	awsicCmd := integrationsCmd.Command("awsic", "Operate on AWS Identity Center resources synced with the cluster.")
	c.awsicAccountsCmd = awsicCmd.Command("accounts", "List AWS Identity Center accounts and their permission sets.")
	c.awsicAccountsCmd.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.awsicArgs.format, teleport.Text, teleport.JSON, teleport.YAML)

	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
}

// TryRun executes the matched subcommand.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.testCmd.FullCommand():
		commandFunc = func(ctx context.Context, client *authclient.Client) error {
			return c.test(ctx, testClientAdapter{client: client})
		}
	case c.awsicAccountsCmd.FullCommand():
		commandFunc = c.listAWSICAccounts
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer closeFn(ctx)
	err = commandFunc(ctx, client)

	return true, trace.Wrap(err)
}

func (c *Command) test(ctx context.Context, client testClient) error {
	integration, err := client.IntegrationsClient().GetIntegration(ctx, c.testArgs.integration)
	if err != nil {
		return trace.Wrap(err)
	}

	var output fmt.Stringer
	switch integration.GetSubKind() {
	case types.IntegrationSubKindAWSOIDC:
		output, err = c.testAWSOIDC(ctx, client.AWSOIDCClient())
		if err != nil {
			return trace.Wrap(err, "testing AWS OIDC integration")
		}
	default:
		return trace.BadParameter("unsupported integration subkind: %s", integration.GetSubKind())
	}

	switch c.testArgs.format {
	case teleport.Text:
		fmt.Fprint(c.Stdout, output)
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSON(c.Stdout, output))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(c.Stdout, output))
	default:
		return trace.BadParameter("unknown value for --format flag: %s", c.testArgs.format)
	}

	return nil
}

// Command implements integrations helper commands.
type Command struct {
	testCmd  *kingpin.CmdClause
	testArgs testArgs

	awsicAccountsCmd *kingpin.CmdClause
	awsicArgs        awsicArgs

	// Stdout allows to switch the standard output source. Used in tests.
	Stdout io.Writer
}

type testArgs struct {
	integration string
	format      string
}

type testClient interface {
	IntegrationsClient() integrationsFetcher
	AWSOIDCClient() awsOIDCPinger
}

type integrationsFetcher interface {
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

type testClientAdapter struct {
	client *authclient.Client
}

func (a testClientAdapter) IntegrationsClient() integrationsFetcher {
	return a.client
}

func (a testClientAdapter) AWSOIDCClient() awsOIDCPinger {
	return a.client.IntegrationAWSOIDCClient()
}

// bold wraps the given text in an ANSI escape to bold it
func bold(text string) string {
	return utils.Color(utils.Bold, text)
}
