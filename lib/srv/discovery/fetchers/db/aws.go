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
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// maxAWSPages is the maximum number of pages to iterate over when fetching aws
// databases.
const maxAWSPages = 10

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
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider
	// Type is the type of DB matcher, for example "rds", "redshift", etc.
	Type string
	// AssumeRole provides a role ARN and ExternalID to assume an AWS role
	// when fetching databases.
	AssumeRole types.AssumeRole
	// Labels is a selector to match cloud database tags.
	Labels types.Labels
	// Region is the AWS region selector to match cloud databases.
	Region string
	// Logger is the slog.Logger
	Logger *slog.Logger
	// Integration is the integration name to be used to fetch credentials.
	// When present, it will use this integration and discard any local credentials.
	Integration string
	// DiscoveryConfigName is the name of the discovery config which originated the resource.
	// Might be empty when the fetcher is using static matchers:
	// ie teleport.yaml/discovery_service.<cloud>.<matcher>
	DiscoveryConfigName string

	// awsClients provides AWS SDK v2 clients.
	awsClients AWSClientProvider
}

// CheckAndSetDefaults validates the config and sets defaults.
func (cfg *awsFetcherConfig) CheckAndSetDefaults(component string) error {
	if cfg.AWSClients == nil {
		return trace.BadParameter("missing parameter AWSClients")
	}
	if cfg.AWSConfigProvider == nil {
		return trace.BadParameter("missing AWSConfigProvider")
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
	if cfg.Logger == nil {
		credentialsSource := "environment"
		if cfg.Integration != "" {
			credentialsSource = fmt.Sprintf("integration:%s", cfg.Integration)
		}
		cfg.Logger = slog.With(
			teleport.ComponentKey, "watch:"+component,
			"labels", cfg.Labels,
			"region", cfg.Region,
			"role", cfg.AssumeRole,
			"credentials", credentialsSource,
		)
	}

	if cfg.awsClients == nil {
		cfg.awsClients = defaultAWSClients{}
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
	return filterDatabasesByLabels(ctx, databases, f.cfg.Labels, f.cfg.Logger), nil
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

// IntegrationName returns the integration name whose credentials are used to fetch the resources.
func (f *awsFetcher) IntegrationName() string {
	return f.cfg.Integration
}

// GetDiscoveryConfigName returns the discovery config name whose matchers are used to fetch the resources.
func (f *awsFetcher) GetDiscoveryConfigName() string {
	return f.cfg.DiscoveryConfigName
}

// ResourceType identifies the resource type the fetcher is returning.
func (f *awsFetcher) ResourceType() string {
	return types.KindDatabase
}

// FetcherType returns the type (`discovery_service.aws.[].types`) of the fetcher.
func (f *awsFetcher) FetcherType() string {
	return f.cfg.Type
}

// String returns the fetcher's string description.
func (f *awsFetcher) String() string {
	return fmt.Sprintf("awsFetcher(Type: %v, Region=%v, Labels=%v)",
		f.cfg.Type, f.cfg.Region, f.cfg.Labels)
}
