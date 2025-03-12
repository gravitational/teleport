/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// ExternalAuditStorageCommand implements "tctl externalauditstorage" group of commands.
type ExternalAuditStorageCommand struct {
	config *servicecfg.Config

	integrationName string
	region          string

	// promote implements the "tctl externalauditstorage promote" subcommand.
	promote *kingpin.CmdClause
	// generate implements the "tctl externalauditstorage generate" subcommand.
	generate *kingpin.CmdClause
}

// Initialize allows ExternalAuditStorageCommand to plug itself into the CLI parser.
func (c *ExternalAuditStorageCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	externalAuditStorage := app.Command("externalauditstorage", "Operate on External Audit Storage configuration.").Hidden()
	c.promote = externalAuditStorage.Command("promote", "Promotes existing draft External Audit Storage configuration to cluster").Hidden()

	// This command should remain hidden it is only meant for development/test.
	c.generate = externalAuditStorage.Command("generate", "Generates an External Audit Storage configuration with randomized resource names and saves it as the current draft").Hidden()
	c.generate.Flag("integration", "Name of an existing AWS OIDC integration").Required().StringVar(&c.integrationName)
	c.generate.Flag("region", "AWS region where infrastructure will be hosted").Required().StringVar(&c.region)
}

// TryRun attempts to run subcommands.
func (c *ExternalAuditStorageCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.promote.FullCommand():
		commandFunc = c.Promote
	case c.generate.FullCommand():
		commandFunc = c.Generate
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)
	return true, trace.Wrap(err)
}

// Promote calls PromoteToClusterExternalAuditStorage, which results in enabling
// External Audit Storage in the cluster based on existing draft.
func (c *ExternalAuditStorageCommand) Promote(ctx context.Context, clt *authclient.Client) error {
	return trace.Wrap(clt.ExternalAuditStorageClient().PromoteToClusterExternalAuditStorage(ctx))
}

// Generate creates an External Audit Storage configuration with randomized
// resource names and saves it as the current draft.
func (c *ExternalAuditStorageCommand) Generate(ctx context.Context, clt *authclient.Client) error {
	_, err := clt.ExternalAuditStorageClient().GenerateDraftExternalAuditStorage(ctx, c.integrationName, c.region)
	return trace.Wrap(err)
}
