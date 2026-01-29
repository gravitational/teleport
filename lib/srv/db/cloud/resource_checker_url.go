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

package cloud

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

// urlChecker validates the database has the correct URL.
type urlChecker struct {
	// awsConfigProvider provides [aws.Config] for AWS SDK service clients.
	awsConfigProvider awsconfig.Provider
	// awsClients is an SDK client provider.
	awsClients awsClientProvider

	logger *slog.Logger

	warnAWSOnce sync.Once

	// TODO(greedy52) consider caching describe call responses to avoid
	// repeated calls:
	// - metadata service
	// - multiple endpoints from the same cloud resource
}

func newURLChecker(cfg DiscoveryResourceCheckerConfig) *urlChecker {
	return &urlChecker{
		awsConfigProvider: cfg.AWSConfigProvider,
		awsClients:        defaultAWSClients{},
		logger:            cfg.Logger,
	}
}

type checkDatabaseFunc func(context.Context, types.Database) error
type isEndpointFunc func(string) bool

func convIsEndpoint(isEndpoint isEndpointFunc) checkDatabaseFunc {
	return func(_ context.Context, database types.Database) error {
		if isEndpoint(database.GetURI()) {
			return nil
		}
		return trace.BadParameter("expect a %q endpoint for database %q but got %v", database.GetType(), database.GetName(), database.GetURI())
	}
}

// Check permforms url checks.
func (c *urlChecker) Check(ctx context.Context, database types.Database) error {
	checkersByDatabaseType := map[string]checkDatabaseFunc{
		types.DatabaseTypeRDS:                   c.checkAWS(c.checkRDS, convIsEndpoint(apiawsutils.IsRDSEndpoint)),
		types.DatabaseTypeRDSOracle:             c.checkAWS(c.checkRDS, convIsEndpoint(apiawsutils.IsRDSEndpoint)),
		types.DatabaseTypeRDSProxy:              c.checkAWS(c.checkRDSProxy, convIsEndpoint(apiawsutils.IsRDSEndpoint)),
		types.DatabaseTypeRedshift:              c.checkAWS(c.checkRedshift, convIsEndpoint(apiawsutils.IsRedshiftEndpoint)),
		types.DatabaseTypeRedshiftServerless:    c.checkAWS(c.checkRedshiftServerless, convIsEndpoint(apiawsutils.IsRedshiftServerlessEndpoint)),
		types.DatabaseTypeElastiCache:           c.checkAWS(c.checkElastiCache, convIsEndpoint(apiawsutils.IsElastiCacheEndpoint)),
		types.DatabaseTypeElastiCacheServerless: c.checkAWS(c.checkElastiCacheServerless, convIsEndpoint(apiawsutils.IsElastiCacheServerlessEndpoint)),
		types.DatabaseTypeMemoryDB:              c.checkAWS(c.checkMemoryDB, convIsEndpoint(apiawsutils.IsMemoryDBEndpoint)),
		types.DatabaseTypeOpenSearch:            c.checkAWS(c.checkOpenSearch, c.checkOpenSearchEndpoint),
		types.DatabaseTypeDocumentDB:            c.checkAWS(c.checkDocumentDB, convIsEndpoint(apiawsutils.IsDocumentDBEndpoint)),
		types.DatabaseTypeAzure:                 c.checkAzure,
	}

	if check := checkersByDatabaseType[database.GetType()]; check != nil {
		return trace.Wrap(check(ctx, database))
	}

	c.logger.DebugContext(ctx, "URL checker does not support database type", "database_type", database.GetType())
	return nil
}

func requireDatabaseIsEndpoint(ctx context.Context, database types.Database, isEndpoint isEndpointFunc) error {
	return trace.Wrap(convIsEndpoint(isEndpoint)(ctx, database))
}

func requireDatabaseAddressPort(database types.Database, wantURLHost *string, wantURLPort *int32) error {
	wantURL := fmt.Sprintf("%v:%v", aws.ToString(wantURLHost), aws.ToInt32(wantURLPort))
	if database.GetURI() != wantURL {
		return trace.BadParameter("expect database URL %q but got %q for database %q", wantURL, database.GetURI(), database.GetName())
	}
	return nil
}

func requireDatabaseHost(database types.Database, wantURLHost string) error {
	host, _, _ := net.SplitHostPort(database.GetURI())
	if host != wantURLHost {
		return trace.BadParameter("expect database URL %q:<port> but got %q for database %q", wantURLHost, database.GetURI(), database.GetName())
	}
	return nil
}

func requireContainsDatabaseURLAndEndpointType(in types.Databases, database types.Database, resource any) error {
	matchURLAndType := func(other types.Database) bool {
		return other.GetURI() == database.GetURI() && other.GetEndpointType() == database.GetEndpointType()
	}
	if slices.ContainsFunc(in, matchURLAndType) {
		return nil
	}
	return trace.BadParameter("cannot find %v in %#v", database.GetURI(), resource)
}
