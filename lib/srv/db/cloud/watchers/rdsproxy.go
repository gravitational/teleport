/*
Copyright 2022 Gravitational, Inc.

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

package watchers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// rdsDBProxyFetcher retrieves RDS proxies and proxy endpoints.
type rdsDBProxyFetcher struct {
	cfg rdsFetcherConfig
	log logrus.FieldLogger
}

// newRDSDBProxyFetcher returns a new RDS proxy fetcher instance.
func newRDSDBProxyFetcher(config rdsFetcherConfig) (Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &rdsDBProxyFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:rdsproxy",
			"labels":        config.Labels,
			"region":        config.Region,
		}),
	}, nil
}

// Get returns RDS proxies and prosy endpoints matching the watcher's
// selectors.
func (f *rdsDBProxyFetcher) Get(ctx context.Context) (types.Databases, error) {
	databases, err := f.getRDSProxyDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var result types.Databases
	for _, database := range databases {
		match, _, err := services.MatchLabels(f.cfg.Labels, database.GetAllLabels())
		if err != nil {
			f.log.Warnf("Failed to match %v against selector: %v.", database, err)
		} else if match {
			result = append(result, database)
		} else {
			f.log.Debugf("%v doesn't match selector.", database)
		}
	}
	return result, nil
}

// getRDSProxyDatabases returns a list of database resources representing RDS
// proxies and proxy endpoints.
func (f *rdsDBProxyFetcher) getRDSProxyDatabases(ctx context.Context) (types.Databases, error) {
	rdsProxies, err := getRDSProxies(ctx, f.cfg.RDS, maxPages)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rdsProxyEndpoints, err := getRDSProxyEndpoints(ctx, f.cfg.RDS, maxPages)
	if err != nil {
		f.log.Debugf("Failed to get RDS proxy endpoints: %v.", err)
	}

	databases := types.Databases{}
	for _, dbProxy := range rdsProxies {
		// rds.DBProxy has no port information. An extra SDK call is made to
		// find the port from its targets.
		port, err := getRDSProxyTargetPort(ctx, f.cfg.RDS, dbProxy.DBProxyName)
		if err != nil {
			f.log.Debugf("Failed to get port for RDS proxy %v: %v.", aws.StringValue(dbProxy.DBProxyName), err)
			continue
		}

		// Add a database from RDS proxy (default endpoint).
		database, err := services.NewDatabaseFromRDSProxy(dbProxy, port)
		if err != nil {
			f.log.Debugf("Could not convert RDS proxy %q to database resource: %v.",
				aws.StringValue(dbProxy.DBProxyName), err)
		} else {
			databases = append(databases, database)
		}

		// Add additional proxy endpoints from the same proxy.
		for key, dbProxyEndpoint := range rdsProxyEndpoints {
			if aws.StringValue(dbProxyEndpoint.DBProxyName) != aws.StringValue(dbProxy.DBProxyName) {
				continue
			}

			database, err = services.NewDatabaseFromRDSProxyEndpoint(dbProxy, dbProxyEndpoint, port)
			if err != nil {
				f.log.Debugf("Could not convert RDS proxy endpoint %q to database resource: %v.",
					aws.StringValue(dbProxyEndpoint.DBProxyEndpointName), err)
			} else {
				databases = append(databases, database)
			}

			delete(rdsProxyEndpoints, key)
		}
	}
	return databases, nil
}

// getRDSProxies fetches all RDS proxies using the provided client, up to the
// specified max number of pages.
func getRDSProxies(ctx context.Context, rdsClient rdsiface.RDSAPI, maxPages int) (rdsProxies []*rds.DBProxy, err error) {
	var pageNum int
	err = rdsClient.DescribeDBProxiesPagesWithContext(ctx,
		&rds.DescribeDBProxiesInput{},
		func(ddo *rds.DescribeDBProxiesOutput, lastPage bool) bool {
			pageNum++
			rdsProxies = append(rdsProxies, ddo.DBProxies...)
			return pageNum <= maxPages
		},
	)
	return rdsProxies, common.ConvertError(err)
}

// getRDSProxyEndpoints fetches all RDS proxy endpoints using the
// provided client, up to the specified max number of pages.
func getRDSProxyEndpoints(ctx context.Context, rdsClient rdsiface.RDSAPI, maxPages int) (rdsProxyEndpoints map[string]*rds.DBProxyEndpoint, err error) {
	rdsProxyEndpoints = make(map[string]*rds.DBProxyEndpoint)
	var pageNum int
	err = rdsClient.DescribeDBProxyEndpointsPagesWithContext(ctx,
		&rds.DescribeDBProxyEndpointsInput{},
		func(ddo *rds.DescribeDBProxyEndpointsOutput, lastPage bool) bool {
			pageNum++

			for _, dbProxyEndpoint := range ddo.DBProxyEndpoints {
				rdsProxyEndpoints[aws.StringValue(dbProxyEndpoint.DBProxyEndpointArn)] = dbProxyEndpoint
			}
			return pageNum <= maxPages
		})
	return rdsProxyEndpoints, common.ConvertError(err)
}

// getRDSProxyTargetPort gets the port number that the targets of the RDS proxy
// are using.
func getRDSProxyTargetPort(ctx context.Context, rdsClient rdsiface.RDSAPI, dbProxyName *string) (port int64, err error) {
	output, err := rdsClient.DescribeDBProxyTargetsWithContext(ctx, &rds.DescribeDBProxyTargetsInput{
		DBProxyName: dbProxyName,
	})
	if err != nil {
		return 0, common.ConvertError(err)
	}

	// The proxy may have multiple targets but they should have the same port.
	for _, target := range output.Targets {
		if target.Port != nil {
			return aws.Int64Value(target.Port), nil
		}
	}
	return 0, trace.NotFound("RDS proxy target port not found")
}
