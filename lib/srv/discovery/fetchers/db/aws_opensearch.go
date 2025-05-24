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
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	opensearch "github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// OpenSearchClient is a subset of the AWS OpenSearch API.
type OpenSearchClient interface {
	DescribeDomains(context.Context, *opensearch.DescribeDomainsInput, ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error)
	ListDomainNames(context.Context, *opensearch.ListDomainNamesInput, ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error)
	ListTags(context.Context, *opensearch.ListTagsInput, ...func(*opensearch.Options)) (*opensearch.ListTagsOutput, error)
}

// newOpenSearchFetcher returns a new AWS fetcher for OpenSearch databases.
func newOpenSearchFetcher(cfg awsFetcherConfig) (common.Fetcher, error) {
	return newAWSFetcher(cfg, &openSearchPlugin{})
}

// openSearchPlugin retrieves OpenSearch databases.
type openSearchPlugin struct{}

func (f *openSearchPlugin) ComponentShortName() string {
	return "opensearch"
}

// GetDatabases returns OpenSearch databases.
func (f *openSearchPlugin) GetDatabases(ctx context.Context, cfg *awsFetcherConfig) (types.Databases, error) {
	awsCfg, err := cfg.AWSConfigProvider.GetConfig(ctx, cfg.Region,
		awsconfig.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		awsconfig.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := cfg.awsClients.GetOpenSearchClient(awsCfg)
	domains, err := getOpenSearchDomains(ctx, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var eligibleDomains []opensearchtypes.DomainStatus
	for _, domain := range domains {
		if !libcloudaws.IsOpenSearchDomainAvailable(&domain) {
			cfg.Logger.DebugContext(ctx, "Skipping unavailable OpenSearch domain", "domain", aws.ToString(domain.DomainName))
			continue
		}

		eligibleDomains = append(eligibleDomains, domain)
	}

	if len(eligibleDomains) == 0 {
		return nil, nil
	}

	var databases types.Databases
	for _, domain := range eligibleDomains {
		tags, err := getOpenSearchResourceTags(ctx, clt, domain.ARN)

		if err != nil {
			if trace.IsAccessDenied(err) {
				cfg.Logger.DebugContext(ctx, "No permissions to list resource tags", "error", err)
			} else {
				cfg.Logger.InfoContext(ctx, "Failed to list resource tags for OpenSearch domain",
					"error", err,
					"domain", aws.ToString(domain.DomainName),
				)
			}
		}

		dbs, err := common.NewDatabasesFromOpenSearchDomain(&domain, tags)
		if err != nil {
			cfg.Logger.InfoContext(ctx, "Could not convert OpenSearch domain configuration to database resource",
				"error", err,
				"domain", aws.ToString(domain.DomainName),
			)
		} else {
			databases = append(databases, dbs...)
		}
	}
	return databases, nil
}

// getOpenSearchDomains fetches all OpenSearch domains.
func getOpenSearchDomains(ctx context.Context, client OpenSearchClient) ([]opensearchtypes.DomainStatus, error) {
	names, err := client.ListDomainNames(ctx, &opensearch.ListDomainNamesInput{})
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	// API only allows 5 at a time.
	var all []opensearchtypes.DomainStatus
	for chunk := range slices.Chunk(names.DomainNames, 5) {
		req := &opensearch.DescribeDomainsInput{}

		for _, domain := range chunk {
			if dn := aws.ToString(domain.DomainName); dn != "" {
				req.DomainNames = append(req.DomainNames, dn)
			}
		}

		domains, err := client.DescribeDomains(ctx, req)
		if err != nil {
			return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
		}
		all = append(all, domains.DomainStatusList...)
	}
	return all, nil
}

// getOpenSearchResourceTags fetches resource tags for provided ARN.
func getOpenSearchResourceTags(ctx context.Context, client OpenSearchClient, resourceARN *string) ([]opensearchtypes.Tag, error) {
	output, err := client.ListTags(ctx, &opensearch.ListTagsInput{ARN: resourceARN})
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	return output.TagList, nil
}
