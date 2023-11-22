/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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

// Initialize allows ExternalCloudAuditCommand to plug itself into the CLI parser.
func (c *ExternalAuditStorageCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.config = config

	externalAuditStorage := app.Command("externalauditstorage", "Operate on external cloud audit configuration.").Alias("externalcloudaudit").Hidden()
	c.promote = externalAuditStorage.Command("promote", "Promotes existing draft external cloud audit to be used in cluster").Hidden()

	// This command should remain hidden it is only meant for development/test.
	c.generate = externalAuditStorage.Command("generate", "Generates an external cloud audit configuration with randomized resource names and saves it as the current draft").Hidden()
	c.generate.Flag("integration", "Name of an existing AWS OIDC integration").Required().StringVar(&c.integrationName)
	c.generate.Flag("region", "AWS region where infrastructure will be hosted").Required().StringVar(&c.region)
}

// TryRun attempts to run subcommands.
func (c *ExternalAuditStorageCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.promote.FullCommand():
		err = c.Promote(ctx, client)
	case c.generate.FullCommand():
		err = c.Generate(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// Promote calls PromoteToClusterExternalCloudAudit, which results in enabling
// external cloud audit in cluster based on existing draft.
func (c *ExternalAuditStorageCommand) Promote(ctx context.Context, clt auth.ClientI) error {
	return trace.Wrap(clt.ExternalCloudAuditClient().PromoteToClusterExternalCloudAudit(ctx))
}

// Generate creates an external cloud audit configuration with randomized
// resource names and saves it as the current draft.
func (c *ExternalAuditStorageCommand) Generate(ctx context.Context, clt auth.ClientI) error {
	_, err := clt.ExternalCloudAuditClient().GenerateDraftExternalCloudAudit(ctx, c.integrationName, c.region)
	return trace.Wrap(err)
}
