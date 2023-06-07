// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/opensearchservice/opensearchserviceiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// openSearchFetcherConfig is the OpenSearch databases fetcher configuration.
type openSearchFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// openSearch is the OpenSearch API client.
	openSearch opensearchserviceiface.OpenSearchServiceAPI
	// Region is the AWS region to query databases in.
	Region string
	// AssumeRole is the AWS IAM role to assume before discovering databases.
	AssumeRole types.AssumeRole
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *openSearchFetcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.openSearch == nil {
		return trace.BadParameter("missing parameter openSearch")
	}
	if c.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	return nil
}

// openSearchFetcher retrieves OpenSearch databases.
type openSearchFetcher struct {
	awsFetcher

	cfg openSearchFetcherConfig
	log logrus.FieldLogger
}

// newOpenSearchFetcher returns a new OpenSearch databases fetcher instance.
func newOpenSearchFetcher(config openSearchFetcherConfig) (common.Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &openSearchFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:opensearch",
			"labels":        config.Labels,
			"region":        config.Region,
			"role":          config.AssumeRole,
		}),
	}, nil
}

// Get returns OpenSearch databases matching the watcher's selectors.
func (f *openSearchFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	domains, err := getOpenSearchDomains(ctx, f.cfg.openSearch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var eligibleDomains []*opensearchservice.DomainStatus
	for _, domain := range domains {
		if !services.IsOpenSearchDomainAvailable(domain) {
			f.log.Debugf("OpenSearch domain %q is unavailable. Skipping.", aws.StringValue(domain.DomainName))
			continue
		}

		eligibleDomains = append(eligibleDomains, domain)
	}

	if len(eligibleDomains) == 0 {
		return types.ResourcesWithLabels{}, nil
	}

	var databases types.Databases
	for _, domain := range eligibleDomains {
		tags, err := getOpenSearchResourceTags(ctx, f.cfg.openSearch, domain.ARN)

		if err != nil {
			if trace.IsAccessDenied(err) {
				f.log.WithError(err).Debug("No permissions to list resource tags")
			} else {
				f.log.WithError(err).Infof("Failed to list resource tags for OpenSearch domain %q.", aws.StringValue(domain.DomainName))
			}
		}

		dbs, err := services.NewDatabaseFromOpenSearchDomain(domain, tags)
		if err != nil {
			f.log.WithError(err).Infof("Could not convert OpenSearch domain %q configuration to database resource.", aws.StringValue(domain.DomainName))
		} else {
			databases = append(databases, dbs...)
		}
	}

	applyAssumeRoleToDatabases(databases, f.cfg.AssumeRole)
	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log).AsResources(), nil
}

// String returns the fetcher's string description.
func (f *openSearchFetcher) String() string {
	return fmt.Sprintf("openSearchFetcher(Region=%v, Labels=%v)",
		f.cfg.Region, f.cfg.Labels)
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
