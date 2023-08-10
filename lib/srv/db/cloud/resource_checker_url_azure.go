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

package cloud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/defaults"
)

func (c *urlChecker) checkAzure(ctx context.Context, database types.Database) error {
	// TODO check by fetching the resources from Azure and compare the URLs.
	if err := c.checkIsAzureEndpoint(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	c.log.Debugf("Azure database %q URL validated.", database.GetName())
	return nil
}

func (c *urlChecker) checkIsAzureEndpoint(ctx context.Context, database types.Database) error {
	switch database.GetProtocol() {
	case defaults.ProtocolRedis:
		return trace.Wrap(requireDatabaseIsEndpoint(ctx, database, azure.IsCacheForRedisEndpoint))

	case defaults.ProtocolMySQL, defaults.ProtocolPostgres:
		return trace.Wrap(requireDatabaseIsEndpoint(ctx, database, azure.IsDatabaseEndpoint))

	case defaults.ProtocolSQLServer:
		return trace.Wrap(requireDatabaseIsEndpoint(ctx, database, azure.IsMSSQLServerEndpoint))
	}
	c.log.Debugf("URL checker does not support Azure database type %q protocol %q.", database.GetType(), database.GetProtocol())
	return nil
}
