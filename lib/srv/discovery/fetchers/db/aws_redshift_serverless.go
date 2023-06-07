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

package db

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/aws/aws-sdk-go/service/redshiftserverless/redshiftserverlessiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// redshiftServerlessFetcherConfig is the Redshift Serverless databases fetcher
// configuration.
type redshiftServerlessFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// Region is the AWS region to query databases in.
	Region string
	// Client is the Redshift Serverless API client.
	Client redshiftserverlessiface.RedshiftServerlessAPI
	// AssumeRole is the AWS IAM role to assume before discovering databases.
	AssumeRole types.AssumeRole
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *redshiftServerlessFetcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	return nil
}

type redshiftServerlessWorkgroupWithTags struct {
	*redshiftserverless.Workgroup

	Tags []*redshiftserverless.Tag
}

// redshiftServerlessFetcher retrieves Redshift Serverless databases.
type redshiftServerlessFetcher struct {
	awsFetcher

	cfg redshiftServerlessFetcherConfig
	log logrus.FieldLogger
}

// newRedshiftServerlessFetcher returns a new Redshift Serverless databases
// fetcher instance.
func newRedshiftServerlessFetcher(config redshiftServerlessFetcherConfig) (common.Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &redshiftServerlessFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:rss<", // (r)ed(s)hift (s)erver(<)less
			"labels":        config.Labels,
			"region":        config.Region,
			"role":          config.AssumeRole,
		}),
	}, nil
}

// Get returns Redshift Serverless databases matching the watcher's selectors.
func (f *redshiftServerlessFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	databases, workgroups, err := f.getDatabasesFromWorkgroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(workgroups) > 0 {
		vpcEndpointDatabases, err := f.getDatabasesFromVPCEndpoints(ctx, workgroups)
		if err != nil {
			if trace.IsAccessDenied(err) {
				f.log.Debugf("No permission to get Redshift Serverless VPC endpoints: %v.", err)
			} else {
				f.log.Warnf("Failed to get Redshift Serverless VPC endpoints: %v.", err)
			}
		}

		databases = append(databases, vpcEndpointDatabases...)
	}
	applyAssumeRoleToDatabases(databases, f.cfg.AssumeRole)
	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log).AsResources(), nil
}

// String returns the fetcher's string description.
func (f *redshiftServerlessFetcher) String() string {
	return fmt.Sprintf("redshiftServerlessFetcher(Region=%v, Labels=%v)", f.cfg.Region, f.cfg.Labels)
}

func (f *redshiftServerlessFetcher) getDatabasesFromWorkgroups(ctx context.Context) (types.Databases, []*redshiftServerlessWorkgroupWithTags, error) {
	workgroups, err := f.getWorkgroups(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var databases types.Databases
	var workgroupsWithTags []*redshiftServerlessWorkgroupWithTags
	for _, workgroup := range workgroups {
		if !services.IsAWSResourceAvailable(workgroup, workgroup.Status) {
			f.log.Debugf("The current status of Redshift Serverless workgroup %v is %v. Skipping.", aws.StringValue(workgroup.WorkgroupName), aws.StringValue(workgroup.Status))
			continue
		}

		tags := f.getResourceTags(ctx, workgroup.WorkgroupArn)
		database, err := services.NewDatabaseFromRedshiftServerlessWorkgroup(workgroup, tags)
		if err != nil {
			f.log.WithError(err).Infof("Could not convert Redshift Serverless workgroup %q to database resource.", aws.StringValue(workgroup.WorkgroupName))
			continue
		}

		databases = append(databases, database)
		workgroupsWithTags = append(workgroupsWithTags, &redshiftServerlessWorkgroupWithTags{
			Workgroup: workgroup,
			Tags:      tags,
		})
	}
	return databases, workgroupsWithTags, nil
}

func (f *redshiftServerlessFetcher) getDatabasesFromVPCEndpoints(ctx context.Context, workgroups []*redshiftServerlessWorkgroupWithTags) (types.Databases, error) {
	endpoints, err := f.getVPCEndpoints(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, endpoint := range endpoints {
		workgroup, found := findWorkgroupWithName(workgroups, aws.StringValue(endpoint.WorkgroupName))
		if !found {
			f.log.Debugf("Could not find matching workgroup for Redshift Serverless endpoint %v. Skipping.", aws.StringValue(endpoint.EndpointName))
			continue
		}

		if !services.IsAWSResourceAvailable(endpoint, endpoint.EndpointStatus) {
			f.log.Debugf("The current status of Redshift Serverless endpoint %v is %v. Skipping.", aws.StringValue(endpoint.EndpointName), aws.StringValue(endpoint.EndpointStatus))
			continue
		}

		// VPC endpoints do not have resource tags attached to them. Use the
		// tags from the workgroups instead.
		database, err := services.NewDatabaseFromRedshiftServerlessVPCEndpoint(endpoint, workgroup.Workgroup, workgroup.Tags)
		if err != nil {
			f.log.WithError(err).Infof("Could not convert Redshift Serverless endpoint %q to database resource.", aws.StringValue(endpoint.EndpointName))
			continue
		}
		databases = append(databases, database)
	}
	return databases, nil
}

func (f *redshiftServerlessFetcher) getResourceTags(ctx context.Context, arn *string) []*redshiftserverless.Tag {
	output, err := f.cfg.Client.ListTagsForResourceWithContext(ctx, &redshiftserverless.ListTagsForResourceInput{
		ResourceArn: arn,
	})
	if err != nil {
		// Log errors here and return nil.
		if trace.IsAccessDenied(err) {
			f.log.WithError(err).Debugf("No Permission to get tags for %q.", aws.StringValue(arn))
		} else {
			f.log.WithError(err).Warnf("Failed to get tags for %q.", aws.StringValue(arn))
		}
		return nil
	}
	return output.Tags
}

func (f *redshiftServerlessFetcher) getWorkgroups(ctx context.Context) ([]*redshiftserverless.Workgroup, error) {
	var pages [][]*redshiftserverless.Workgroup
	err := f.cfg.Client.ListWorkgroupsPagesWithContext(ctx, nil, func(page *redshiftserverless.ListWorkgroupsOutput, lastPage bool) bool {
		pages = append(pages, page.Workgroups)
		return len(pages) <= maxAWSPages
	})
	return flatten(pages), libcloudaws.ConvertRequestFailureError(err)
}

func (f *redshiftServerlessFetcher) getVPCEndpoints(ctx context.Context) ([]*redshiftserverless.EndpointAccess, error) {
	var pages [][]*redshiftserverless.EndpointAccess
	err := f.cfg.Client.ListEndpointAccessPagesWithContext(ctx, nil, func(page *redshiftserverless.ListEndpointAccessOutput, lastPage bool) bool {
		pages = append(pages, page.Endpoints)
		return len(pages) <= maxAWSPages
	})
	return flatten(pages), libcloudaws.ConvertRequestFailureError(err)
}

func findWorkgroupWithName(workgroups []*redshiftServerlessWorkgroupWithTags, name string) (*redshiftServerlessWorkgroupWithTags, bool) {
	for _, workgroup := range workgroups {
		if aws.StringValue(workgroup.WorkgroupName) == name {
			return workgroup, true
		}
	}
	return nil, false
}
