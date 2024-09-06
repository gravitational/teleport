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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/opensearchservice/opensearchserviceiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

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
	opensearchClient, err := cfg.AWSClients.GetAWSOpenSearchClient(ctx, cfg.Region,
		cloud.WithAssumeRole(cfg.AssumeRole.RoleARN, cfg.AssumeRole.ExternalID),
		cloud.WithCredentialsMaybeIntegration(cfg.Integration),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	domains, err := getOpenSearchDomains(ctx, opensearchClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var eligibleDomains []*opensearchservice.DomainStatus
	for _, domain := range domains {
		if !libcloudaws.IsOpenSearchDomainAvailable(domain) {
			cfg.Log.Debugf("OpenSearch domain %q is unavailable. Skipping.", aws.StringValue(domain.DomainName))
			continue
		}

		eligibleDomains = append(eligibleDomains, domain)
	}

	if len(eligibleDomains) == 0 {
		return nil, nil
	}

	var databases types.Databases
	for _, domain := range eligibleDomains {
		tags, err := getOpenSearchResourceTags(ctx, opensearchClient, domain.ARN)

		if err != nil {
			if trace.IsAccessDenied(err) {
				cfg.Log.WithError(err).Debug("No permissions to list resource tags")
			} else {
				cfg.Log.WithError(err).Infof("Failed to list resource tags for OpenSearch domain %q.", aws.StringValue(domain.DomainName))
			}
		}

		dbs, err := common.NewDatabasesFromOpenSearchDomain(domain, tags)
		if err != nil {
			cfg.Log.WithError(err).Infof("Could not convert OpenSearch domain %q configuration to database resource.", aws.StringValue(domain.DomainName))
		} else {
			databases = append(databases, dbs...)
		}
	}
	return databases, nil
}

// getOpenSearchDomains fetches all OpenSearch domains.
func getOpenSearchDomains(ctx context.Context, client opensearchserviceiface.OpenSearchServiceAPI) ([]*opensearchservice.DomainStatus, error) {
	names, err := client.ListDomainNamesWithContext(ctx, nil)
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	req := &opensearchservice.DescribeDomainsInput{DomainNames: []*string{}}
	for _, domain := range names.DomainNames {
		req.DomainNames = append(req.DomainNames, domain.DomainName)
	}

	domains, err := client.DescribeDomainsWithContext(ctx, req)
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}
	return domains.DomainStatusList, nil
}

// getOpenSearchResourceTags fetches resource tags for provided ARN.
func getOpenSearchResourceTags(ctx context.Context, client opensearchserviceiface.OpenSearchServiceAPI, resourceARN *string) ([]*opensearchservice.Tag, error) {
	output, err := client.ListTagsWithContext(ctx, &opensearchservice.ListTagsInput{ARN: resourceARN})
	if err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	return output.TagList, nil
}
