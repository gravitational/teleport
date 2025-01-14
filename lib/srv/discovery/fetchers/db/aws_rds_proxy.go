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

package db

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newRDSDBProxyFetcher returns a new AWS fetcher for RDS Proxy databases.
func newRDSDBProxyFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &rdsDBProxyPlugin{})
}

// rdsDBProxyPlugin retrieves RDS Proxies and their custom endpoints.
type rdsDBProxyPlugin struct{}

func (f *rdsDBProxyPlugin) ComponentShortName() string {
	return "rdsproxy"
}

// GetDatabases returns a list of database resources representing RDS
// Proxies and custom endpoints.
func (f *rdsDBProxyPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetRDSClient(awsCfg)
	// Get a list of all RDS Proxies. Each RDS Proxy has one "default"
	// endpoint.
	rdsProxies, err := getRDSProxies(ctx, clt, maxAWSPages)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get all RDS Proxy custom endpoints sorted by the name of the RDS Proxy
	// that owns the custom endpoints.
	customEndpointsByProxyName, err := getRDSProxyCustomEndpoints(ctx, clt, maxAWSPages)
	if err != nil {
		cfg.Logger.DebugContext(ctx, "Failed to get RDS Proxy endpoints", "error", err)
	}

	var databases types.Databases
	for _, dbProxy := range rdsProxies {
		if !aws.ToBool(dbProxy.RequireTLS) {
			cfg.Logger.DebugContext(ctx, "Skipping RDS Proxy that doesn't support TLS", "rds_proxy", aws.ToString(dbProxy.DBProxyName))
			continue
		}

		if !libcloudaws.IsRDSProxyAvailable(&dbProxy) {
			cfg.Logger.DebugContext(ctx, "Skipping unavailable RDS Proxy",
				"rds_proxy", aws.ToString(dbProxy.DBProxyName),
				"status", dbProxy.Status)
			continue
		}

		// rdstypes.DBProxy has no tags information. An extra SDK call is made to
		// fetch the tags. If failed, keep going without the tags.
		tags, err := listRDSResourceTags(ctx, clt, dbProxy.DBProxyArn)
		if err != nil {
			cfg.Logger.DebugContext(ctx, "Failed to get tags for RDS Proxy",
				"rds_proxy", aws.ToString(dbProxy.DBProxyName),
				"error", err,
			)
		}

		// Add a database from RDS Proxy (default endpoint).
		database, err := common.NewDatabaseFromRDSProxy(&dbProxy, tags)
		if err != nil {
			cfg.Logger.DebugContext(ctx, "Could not convert RDS Proxy to database resource",
				"rds_proxy", aws.ToString(dbProxy.DBProxyName),
				"error", err,
			)
		} else {
			databases = append(databases, database)
		}

		// Add custom endpoints.
		for _, customEndpoint := range customEndpointsByProxyName[aws.ToString(dbProxy.DBProxyName)] {
			if !libcloudaws.IsRDSProxyCustomEndpointAvailable(&customEndpoint) {
				cfg.Logger.DebugContext(ctx, "Skipping unavailable custom endpoint of RDS Proxy",
					"endpoint", aws.ToString(customEndpoint.DBProxyEndpointName),
					"rds_proxy", aws.ToString(customEndpoint.DBProxyName),
					"status", customEndpoint.Status,
				)
				continue
			}

			database, err = common.NewDatabaseFromRDSProxyCustomEndpoint(&dbProxy, &customEndpoint, tags)
			if err != nil {
				cfg.Logger.DebugContext(ctx, "Could not convert custom endpoint for RDS Proxy to database resource",
					"endpoint", aws.ToString(customEndpoint.DBProxyEndpointName),
					"rds_proxy", aws.ToString(customEndpoint.DBProxyName),
					"error", err,
				)
				continue
			}
			databases = append(databases, database)
		}
	}

	return databases, nil
}

// getRDSProxies fetches all RDS Proxies using the provided client, up to the
// specified max number of pages.
func getRDSProxies(ctx context.Context, clt RDSClient, maxPages int) ([]rdstypes.DBProxy, error) {
	pager := rds.NewDescribeDBProxiesPaginator(clt,
		&rds.DescribeDBProxiesInput{},
		func(dcpo *rds.DescribeDBProxiesPaginatorOptions) {
			dcpo.StopOnDuplicateToken = true
		},
	)

	var rdsProxies []rdstypes.DBProxy
	for i := 0; i < maxPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureErrorV2(err))
		}
		rdsProxies = append(rdsProxies, page.DBProxies...)
	}
	return rdsProxies, nil
}

// getRDSProxyCustomEndpoints fetches all RDS Proxy custom endpoints using the
// provided client.
func getRDSProxyCustomEndpoints(ctx context.Context, clt RDSClient, maxPages int) (map[string][]rdstypes.DBProxyEndpoint, error) {
	customEndpointsByProxyName := make(map[string][]rdstypes.DBProxyEndpoint)
	pager := rds.NewDescribeDBProxyEndpointsPaginator(clt,
		&rds.DescribeDBProxyEndpointsInput{},
		func(ddepo *rds.DescribeDBProxyEndpointsPaginatorOptions) {
			ddepo.StopOnDuplicateToken = true
		},
	)
	for i := 0; i < maxPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureErrorV2(err))
		}
		for _, customEndpoint := range page.DBProxyEndpoints {
			customEndpointsByProxyName[aws.ToString(customEndpoint.DBProxyName)] = append(customEndpointsByProxyName[aws.ToString(customEndpoint.DBProxyName)], customEndpoint)
		}
	}
	return customEndpointsByProxyName, nil
}

// listRDSResourceTags returns tags for provided RDS resource.
func listRDSResourceTags(ctx context.Context, clt RDSClient, resourceName *string) ([]rdstypes.Tag, error) {
	output, err := clt.ListTagsForResource(ctx, &rds.ListTagsForResourceInput{
		ResourceName: resourceName,
	})
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureErrorV2(err))
	}
	return output.TagList, nil
}
