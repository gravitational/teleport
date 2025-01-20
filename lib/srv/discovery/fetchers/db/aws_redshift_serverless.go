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
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	rss "github.com/aws/aws-sdk-go-v2/service/redshiftserverless"
	rsstypes "github.com/aws/aws-sdk-go-v2/service/redshiftserverless/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// RSSClient is a subset of the AWS Redshift Serverless API.
type RSSClient interface {
	rss.ListEndpointAccessAPIClient
	rss.ListWorkgroupsAPIClient

	ListTagsForResource(context.Context, *rss.ListTagsForResourceInput, ...func(*rss.Options)) (*rss.ListTagsForResourceOutput, error)
}

// newRedshiftServerlessFetcher returns a new AWS fetcher for Redshift
// Serverless databases.
func newRedshiftServerlessFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &redshiftServerlessPlugin{})
}

type workgroupWithTags struct {
	*rsstypes.Workgroup

	Tags []rsstypes.Tag
}

// redshiftServerlessPlugin retrieves Redshift Serverless databases.
type redshiftServerlessPlugin struct{}

func (f *redshiftServerlessPlugin) ComponentShortName() string {
	// (r)ed(s)hift (s)erver(<)less
	return "rss<"
}

// GetDatabases returns Redshift Serverless databases matching the watcher's selectors.
func (f *redshiftServerlessPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetRedshiftServerlessClient(awsCfg)
	databases, workgroups, err := getDatabasesFromWorkgroups(ctx, clt, cfg.Logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(workgroups) > 0 {
		vpcEndpointDatabases, err := getDatabasesFromVPCEndpoints(ctx, workgroups, clt, cfg.Logger)
		if err != nil {
			if trace.IsAccessDenied(err) {
				cfg.Logger.DebugContext(ctx, "No permission to get Redshift Serverless VPC endpoints", "error", err)
			} else {
				cfg.Logger.WarnContext(ctx, "Failed to get Redshift Serverless VPC endpoints", "error", err)
			}
		}

		databases = append(databases, vpcEndpointDatabases...)
	}
	return databases, nil
}

func getDatabasesFromWorkgroups(ctx context.Context, client RSSClient, logger *slog.Logger) (types.Databases, []*workgroupWithTags, error) {
	workgroups, err := getRSSWorkgroups(ctx, client)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var databases types.Databases
	var workgroupsWithTags []*workgroupWithTags
	for _, workgroup := range workgroups {
		if !isWorkgroupAvailable(logger, &workgroup) {
			logger.DebugContext(ctx, "Skipping unavailable Redshift Serverless workgroup",
				"status", workgroup.Status,
				"workgroup", aws.ToString(workgroup.WorkgroupName),
			)
			continue
		}

		tags := getRSSResourceTags(ctx, workgroup.WorkgroupArn, client, logger)
		database, err := common.NewDatabaseFromRedshiftServerlessWorkgroup(&workgroup, tags)
		if err != nil {
			logger.InfoContext(ctx, "Could not convert Redshift Serverless workgroup to database resource",
				"workgroup", aws.ToString(workgroup.WorkgroupName),
				"error", err,
			)
			continue
		}

		databases = append(databases, database)
		workgroupsWithTags = append(workgroupsWithTags, &workgroupWithTags{
			Workgroup: &workgroup,
			Tags:      tags,
		})
	}
	return databases, workgroupsWithTags, nil
}

func getDatabasesFromVPCEndpoints(ctx context.Context, workgroups []*workgroupWithTags, client RSSClient, logger *slog.Logger) (types.Databases, error) {
	endpoints, err := getRSSVPCEndpoints(ctx, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, endpoint := range endpoints {
		workgroup, found := findWorkgroupWithName(workgroups, aws.ToString(endpoint.WorkgroupName))
		if !found {
			logger.DebugContext(ctx, "Could not find matching workgroup for Redshift Serverless endpoint", "endpoint", aws.ToString(endpoint.EndpointName))
			continue
		}

		if !libcloudaws.IsResourceAvailable(endpoint, endpoint.EndpointStatus) {
			logger.DebugContext(ctx, "Skipping unavailable Redshift Serverless endpoint",
				"endpoint", aws.ToString(endpoint.EndpointName),
				"status", aws.ToString(endpoint.EndpointStatus),
			)
			continue
		}

		// VPC endpoints do not have resource tags attached to them. Use the
		// tags from the workgroups instead.
		database, err := common.NewDatabaseFromRedshiftServerlessVPCEndpoint(&endpoint, workgroup.Workgroup, workgroup.Tags)
		if err != nil {
			logger.InfoContext(ctx, "Could not convert Redshift Serverless endpoint to database resource",
				"endpoint", aws.ToString(endpoint.EndpointName),
				"error", err,
			)
			continue
		}
		databases = append(databases, database)
	}
	return databases, nil
}

func getRSSResourceTags(ctx context.Context, arn *string, client RSSClient, logger *slog.Logger) []rsstypes.Tag {
	output, err := client.ListTagsForResource(ctx, &rss.ListTagsForResourceInput{
		ResourceArn: arn,
	})
	if err != nil {
		// Log errors here and return nil.
		if trace.IsAccessDenied(err) {
			logger.DebugContext(ctx, "No Permission to get Redshift Serverless tags",
				"arn", aws.ToString(arn),
				"error", err,
			)
		} else {
			logger.WarnContext(ctx, "Failed to get Redshift Serverless tags",
				"arn", aws.ToString(arn),
				"error", err,
			)
		}
		return nil
	}
	return output.Tags
}

func getRSSWorkgroups(ctx context.Context, clt RSSClient) ([]rsstypes.Workgroup, error) {
	var out []rsstypes.Workgroup
	pager := rss.NewListWorkgroupsPaginator(clt,
		&rss.ListWorkgroupsInput{},
		func(o *rss.ListWorkgroupsPaginatorOptions) {
			o.StopOnDuplicateToken = true
		},
	)
	for i := 0; i < maxAWSPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, libcloudaws.ConvertRequestFailureErrorV2(err)
		}
		out = append(out, page.Workgroups...)
	}
	return out, nil
}

func getRSSVPCEndpoints(ctx context.Context, clt RSSClient) ([]rsstypes.EndpointAccess, error) {
	var out []rsstypes.EndpointAccess
	pager := rss.NewListEndpointAccessPaginator(clt,
		&rss.ListEndpointAccessInput{},
		func(o *rss.ListEndpointAccessPaginatorOptions) {
			o.StopOnDuplicateToken = true
		},
	)
	for i := 0; i < maxAWSPages && pager.HasMorePages(); i++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, libcloudaws.ConvertRequestFailureErrorV2(err)
		}
		out = append(out, page.Endpoints...)
	}
	return out, nil
}

func findWorkgroupWithName(workgroups []*workgroupWithTags, name string) (*workgroupWithTags, bool) {
	for _, workgroup := range workgroups {
		if aws.ToString(workgroup.WorkgroupName) == name {
			return workgroup, true
		}
	}
	return nil, false
}

func isWorkgroupAvailable(logger *slog.Logger, wg *rsstypes.Workgroup) bool {
	switch wg.Status {
	case
		rsstypes.WorkgroupStatusAvailable,
		rsstypes.WorkgroupStatusModifying:
		return true
	case
		rsstypes.WorkgroupStatusCreating,
		rsstypes.WorkgroupStatusDeleting:
		return false
	default:
		logger.WarnContext(context.Background(), "Assuming Redshift Serverless workgroup with an unknown status is available",
			"status", wg.Status,
			"workgroup", aws.ToString(wg.NamespaceName),
		)
		return true
	}
}
