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
	"os"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

// urlChecker validates the database has the correct URL.
type urlChecker struct {
	// awsConfigProvider provides [aws.Config] for AWS SDK service clients.
	awsConfigProvider awsconfig.Provider
	// awsClients is an SDK client provider.
	awsClients awsClientProvider

	logger      *slog.Logger
	warnOnError bool

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
		warnOnError:       getWarnOnError(),
	}
}

// getWarnOnError returns true if urlChecker should only log a warning instead
// of returning errors when check fails.
//
// DELETE IN 16.0.0 The environement variable is a temporary toggle to disable
// returning errors by urlChecker, in case Database Service doesn't have proper
// permissions and basic endpoint checks fail for unknown reasons. Remove after
// one or two releases when implementation is stable.
func getWarnOnError() bool {
	value := os.Getenv("TELEPORT_DATABASE_URL_CHECK_WARN_ON_ERROR")
	if value == "" {
		return false
	}

	boolValue, err := utils.ParseBool(value)
	if err != nil {
		slog.WarnContext(context.Background(), "Invalid bool value for TELEPORT_DATABASE_URL_CHECK_WARN_ON_ERROR", "value", value)
	}
	return boolValue
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
		types.DatabaseTypeRDS:                c.checkAWS(c.checkRDS, convIsEndpoint(apiawsutils.IsRDSEndpoint)),
		types.DatabaseTypeRDSProxy:           c.checkAWS(c.checkRDSProxy, convIsEndpoint(apiawsutils.IsRDSEndpoint)),
		types.DatabaseTypeRedshift:           c.checkAWS(c.checkRedshift, convIsEndpoint(apiawsutils.IsRedshiftEndpoint)),
		types.DatabaseTypeRedshiftServerless: c.checkAWS(c.checkRedshiftServerless, convIsEndpoint(apiawsutils.IsRedshiftServerlessEndpoint)),
		types.DatabaseTypeElastiCache:        c.checkAWS(c.checkElastiCache, convIsEndpoint(apiawsutils.IsElastiCacheEndpoint)),
		types.DatabaseTypeMemoryDB:           c.checkAWS(c.checkMemoryDB, convIsEndpoint(apiawsutils.IsMemoryDBEndpoint)),
		types.DatabaseTypeOpenSearch:         c.checkAWS(c.checkOpenSearch, c.checkOpenSearchEndpoint),
		types.DatabaseTypeDocumentDB:         c.checkAWS(c.checkDocumentDB, convIsEndpoint(apiawsutils.IsDocumentDBEndpoint)),
		types.DatabaseTypeAzure:              c.checkAzure,
	}

	if check := checkersByDatabaseType[database.GetType()]; check != nil {
		err := check(ctx, database)
		if err != nil && c.warnOnError {
			c.logger.WarnContext(ctx, "URL check failed for database", "database", database.GetName(), "error", err)
			return nil
		}
		return trace.Wrap(err)
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
