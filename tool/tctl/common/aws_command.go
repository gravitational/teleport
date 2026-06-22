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

package common

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// Initialize registers the `tctl aws` command group.
func (c *AWSCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	awsCmd := app.Command("aws", "AWS integration helpers.")

	c.testOIDCCmd = awsCmd.Command("test-oidc", "Validate an AWS OIDC integration by calling AWS STS GetCallerIdentity.")
	c.testOIDCCmd.Flag("integration", "Name of the AWS OIDC integration to test.").Required().StringVar(&c.testOIDCArgs.integration)
	c.testOIDCCmd.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.testOIDCArgs.format, teleport.Text, teleport.JSON, teleport.YAML)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun executes the matched subcommand.
func (c *AWSCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	if cmd != c.testOIDCCmd.FullCommand() {
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(c.TestOIDC(ctx, awsOIDCClientAdapter{client: client}))
}

// TestOIDC validates that the AWS OIDC integration can assume the configured
// role and reach AWS STS. The assumed role does not require permissions beyond
// the trust relationship with `sts:AssumeRoleWithWebIdentity`.
func (c *AWSCommand) TestOIDC(ctx context.Context, client awsOIDCClient) error {
	resp, err := client.IntegrationAWSOIDCClient().Ping(ctx, integrationv1.PingRequest_builder{
		Integration: c.testOIDCArgs.integration,
	}.Build())
	if err != nil {
		if trace.IsNotImplemented(err) {
			return trace.NotImplemented("the server does not support testing AWS OIDC integrations")
		}
		return trace.Wrap(err)
	}

	o := testOIDCOutput{
		Status:         "operational",
		Integration:    c.testOIDCArgs.integration,
		AccountID:      resp.GetAccountId(),
		AssumedRoleARN: resp.GetArn(),
		UserID:         resp.GetUserId(),
	}

	switch c.testOIDCArgs.format {
	case teleport.JSON:
		err := utils.WriteJSON(c.stdout, o)
		if err != nil {
			return trace.Wrap(err, "failed to marshal output")
		}
	case teleport.YAML:
		err := utils.WriteYAML(c.stdout, o)
		if err != nil {
			return trace.Wrap(err, "failed to marshal output")
		}
	case teleport.Text:
		o.output(c.stdout)
	default:
		return trace.BadParameter("unknown value for --format flag %q", c.testOIDCArgs.format)
	}

	return nil
}

type testOIDCOutput struct {
	Status         string `json:"status"`
	Integration    string `json:"integration_name"`
	AccountID      string `json:"account_id"`
	AssumedRoleARN string `json:"assumed_role_arn"`
	UserID         string `json:"user_id"`
}

func (t *testOIDCOutput) output(out io.Writer) error {
	if _, err := fmt.Fprint(out, bold("AWS OIDC integration is operational.")+"\n\n"); err != nil {
		return trace.Wrap(err)
	}
	if _, err := fmt.Fprintf(out, "Integration Name: %s\n", t.Integration); err != nil {
		return trace.Wrap(err)
	}
	if _, err := fmt.Fprintf(out, "Account ID: %s\n", t.AccountID); err != nil {
		return trace.Wrap(err)
	}
	if _, err := fmt.Fprintf(out, "Assumed Role ARN: %s\n", t.AssumedRoleARN); err != nil {
		return trace.Wrap(err)
	}
	if _, err := fmt.Fprintf(out, "User ID: %s\n", t.UserID); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AWSCommand implements AWS-related helper commands.
type AWSCommand struct {
	testOIDCCmd *kingpin.CmdClause

	testOIDCArgs awsOIDCTestArgs

	stdout io.Writer
}

type awsOIDCTestArgs struct {
	integration string
	format      string
}

type awsOIDCClient interface {
	IntegrationAWSOIDCClient() awsOIDCPinger
}

type awsOIDCPinger interface {
	Ping(context.Context, *integrationv1.PingRequest, ...grpc.CallOption) (*integrationv1.PingResponse, error)
}

type awsOIDCClientAdapter struct {
	client *authclient.Client
}

func (a awsOIDCClientAdapter) IntegrationAWSOIDCClient() awsOIDCPinger {
	return a.client.IntegrationAWSOIDCClient()
}

var _ interface {
	Initialize(*kingpin.Application, *tctlcfg.GlobalCLIFlags, *servicecfg.Config)
	TryRun(context.Context, string, commonclient.InitFunc) (bool, error)
} = (*AWSCommand)(nil)
