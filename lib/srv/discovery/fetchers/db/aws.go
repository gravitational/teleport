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

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// awsFetcherPlugin defines an interface that provides database type specific
// functions for use by the common AWS database fetcher.
type awsFetcherPlugin interface {
	// GetDatabases fetches databases from AWS API and converts the results to
	// Teleport types.Databases.
	GetDatabases(context.Context, *awsFetcherConfig) (types.Databases, error)
	// ComponentShortName provides the plugin's short component name for
	// logging purposes.
	ComponentShortName() string
}

// awsFetcherConfig is the AWS database fetcher configuration.
type awsFetcherConfig struct {
	// AWSClients are the AWS API clients.
	AWSClients cloud.AWSClients
	// Type is the type of DB matcher, for example "rds", "redshift", etc.
	Type string
	// AssumeRole provides a role ARN and ExternalID to assume an AWS role
	// when fetching databases.
	AssumeRole types.AssumeRole
	// Labels is a selector to match cloud database tags.
	Labels types.Labels
	// Region is the AWS region selector to match cloud databases.
	Region string
	// Log is a field logger to provide structured logging for each matcher,
	// based on its config settings by default.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates the config and sets defaults.
func (cfg *awsFetcherConfig) CheckAndSetDefaults(component string) error {
	if cfg.AWSClients == nil {
		return trace.BadParameter("missing parameter AWSClients")
	}
	if cfg.Type == "" {
		return trace.BadParameter("missing parameter Type")
	}
	if len(cfg.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if cfg.Region == "" {
		return trace.BadParameter("missing parameter Region")
	}
	if cfg.Log == nil {
		cfg.Log = logrus.WithFields(logrus.Fields{
			trace.Component: "watch:" + component,
			"labels":        cfg.Labels,
			"region":        cfg.Region,
			"role":          cfg.AssumeRole,
		})
	}
	return nil
}

// newAWSFetcher returns a AWS database fetcher for the provided selectors
// and AWS database-type specific fetcher plugin.
func newAWSFetcher(cfg awsFetcherConfig, plugin awsFetcherPlugin) (*awsFetcher, error) {
	if err := cfg.CheckAndSetDefaults(plugin.ComponentShortName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &awsFetcher{cfg: cfg, plugin: plugin}, nil
}

// awsFetcher is the common base for AWS database fetchers.
type awsFetcher struct {
	// cfg is the awsFetcher configuration.
	cfg awsFetcherConfig
	// plugin does AWS database type specific API calls fetch databases.
	plugin awsFetcherPlugin
}

// awsFetcher implements common.Fetcher.
var _ common.Fetcher = (*awsFetcher)(nil)

// Get returns AWS databases matching the fetcher's selectors.
func (f *awsFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	databases, err := f.getDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f.rewriteDatabases(databases)
	return databases.AsResources(), nil
}

func (f *awsFetcher) getDatabases(ctx context.Context) (types.Databases, error) {
	databases, err := f.plugin.GetDatabases(ctx, &f.cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return filterDatabasesByLabels(databases, f.cfg.Labels, f.cfg.Log), nil
}

// rewriteDatabases rewrites the discovered databases.
func (f *awsFetcher) rewriteDatabases(databases types.Databases) {
	for _, db := range databases {
		f.applyAssumeRole(db)
		common.ApplyAWSDatabaseNameSuffix(db, f.cfg.Type)
	}
}

// applyAssumeRole sets the database AWS AssumeRole metadata to match the
// fetcher's setting.
func (f *awsFetcher) applyAssumeRole(db types.Database) {
	db.SetAWSAssumeRole(f.cfg.AssumeRole.RoleARN)
	db.SetAWSExternalID(f.cfg.AssumeRole.ExternalID)
}

// Cloud returns the cloud the fetcher is operating.
func (f *awsFetcher) Cloud() string {
	return types.CloudAWS
}

// ResourceType identifies the resource type the fetcher is returning.
func (f *awsFetcher) ResourceType() string {
	return types.KindDatabase
}

// String returns the fetcher's string description.
func (f *awsFetcher) String() string {
	return fmt.Sprintf("awsFetcher(Type: %v, Region=%v, Labels=%v)",
		f.cfg.Type, f.cfg.Region, f.cfg.Labels)
}

// maxAWSPages is the maximum number of pages to iterate over when fetching aws
// databases.
const maxAWSPages = 10
