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
	"fmt"
	"net"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud"
)

// urlChecker validates the database has the correct URL.
type urlChecker struct {
	clients cloud.Clients
	log     logrus.FieldLogger

	warnAWSOnce sync.Once

	// TODO(greedy52) consider caching describe call responses to avoid
	// repeated calls:
	// - metadata service
	// - multiple endpoints from the same cloud resource
}

func newURLChecker(cfg DiscoveryResourceCheckerConfig) *urlChecker {
	return &urlChecker{
		clients: cfg.Clients,
		log:     cfg.Log,
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
		types.DatabaseTypeRDS:                c.checkAWS(c.checkRDS, convIsEndpoint(apiawsutils.IsRDSEndpoint)),
		types.DatabaseTypeRDSProxy:           c.checkAWS(c.checkRDSProxy, convIsEndpoint(apiawsutils.IsRDSEndpoint)),
		types.DatabaseTypeRedshift:           c.checkAWS(c.checkRedshift, convIsEndpoint(apiawsutils.IsRedshiftEndpoint)),
		types.DatabaseTypeRedshiftServerless: c.checkAWS(c.checkRedshiftServerless, convIsEndpoint(apiawsutils.IsRedshiftServerlessEndpoint)),
		types.DatabaseTypeElastiCache:        c.checkAWS(c.checkElastiCache, convIsEndpoint(apiawsutils.IsElastiCacheEndpoint)),
		types.DatabaseTypeMemoryDB:           c.checkAWS(c.checkMemoryDB, convIsEndpoint(apiawsutils.IsMemoryDBEndpoint)),
		types.DatabaseTypeOpenSearch:         c.checkAWS(c.checkOpenSearch, c.checkOpenSearchEndpoint),
		types.DatabaseTypeAzure:              c.checkAzure,
	}

	if check := checkersByDatabaseType[database.GetType()]; check != nil {
		return trace.Wrap(check(ctx, database))
	}

	c.log.Debugf("URL checker does not support database type %q.", database.GetType())
	return nil
}

func requireDatabaseIsEndpoint(ctx context.Context, database types.Database, isEndpoint isEndpointFunc) error {
	return trace.Wrap(convIsEndpoint(isEndpoint)(ctx, database))
}

func requireDatabaseAddressPort(database types.Database, wantURLHost *string, wantURLPort *int64) error {
	wantURL := fmt.Sprintf("%v:%v", aws.StringValue(wantURLHost), aws.Int64Value(wantURLPort))
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

func requireContainsDatabaseURLAndType(in types.Databases, database types.Database, resource any) error {
	matchURLAndType := func(other types.Database) bool {
		return other.GetURI() == database.GetURI() && other.GetEndpointType() == database.GetEndpointType()
	}
	if slices.ContainsFunc(in, matchURLAndType) {
		return nil
	}
	return trace.BadParameter("cannot find %v in %#v", database.GetURI(), resource)
}
