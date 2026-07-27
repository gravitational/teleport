/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// gcpDatabaseGetter lists and converts databases for one GCP database service in a single project.
type gcpDatabaseGetter func(ctx context.Context, cfg *gcpFetcherConfig, projectID string) (types.Databases, error)

// gcpFetcherConfig is the configuration for a GCP database fetcher.
type gcpFetcherConfig struct {
	// Type is the type of DB matcher, for example "cloudsql".
	Type string
	// GCPClients are the GCP API clients.
	GCPClients gcp.Clients
	// ProjectID is the GCP project to search for databases.
	// May contain wildcard to be resolved with ListProjects API call.
	ProjectID string
	// Locations are the GCP location selectors to match cloud databases.
	// Empty matches all locations.
	Locations []string
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// DiscoveryConfigName is the name of the discovery config which originated the resource.
	// Might be empty when the fetcher is using static matchers:
	// ie teleport.yaml/discovery_service.<cloud>.<matcher>
	DiscoveryConfigName string
	// Logger is the slog.Logger.
	Logger *slog.Logger
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *gcpFetcherConfig) CheckAndSetDefaults() error {
	if c.Type == "" {
		return trace.BadParameter("missing parameter Type")
	}
	if c.GCPClients == nil {
		return trace.BadParameter("missing parameter GCPClients")
	}
	if c.ProjectID == "" {
		return trace.BadParameter("missing parameter ProjectID")
	}
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "watch:gcp")
	}
	c.Logger = c.Logger.With(
		"labels", c.Labels,
		"locations", c.Locations,
		"project_id", c.ProjectID,
		"type", c.Type,
	)
	return nil
}

// regionFilter returns the locations to request from the cloud API, or nil
// (no filter, match all regions) when the locations contain a wildcard.
func (c *gcpFetcherConfig) regionFilter() []string {
	if slices.Contains(c.Locations, types.Wildcard) {
		return nil
	}
	return c.Locations
}

type gcpFetcher struct {
	cfg    gcpFetcherConfig
	getter gcpDatabaseGetter
}

func newGCPFetcher(cfg gcpFetcherConfig, getter gcpDatabaseGetter) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &gcpFetcher{
		cfg:    cfg,
		getter: getter,
	}, nil
}

// Cloud returns the cloud the fetcher is operating.
func (f *gcpFetcher) Cloud() string {
	return types.CloudGCP
}

// ResourceType identifies the resource type the fetcher is returning.
func (f *gcpFetcher) ResourceType() string {
	return types.KindDatabase
}

// FetcherType returns the matcher type (`discovery_service.gcp.[].types`).
func (f *gcpFetcher) FetcherType() string {
	return f.cfg.Type
}

// IntegrationName returns the integration name. GCP database discovery only
// supports ambient credentials, so this is always empty.
func (f *gcpFetcher) IntegrationName() string {
	return ""
}

// GetDiscoveryConfigName is the name of the discovery config which originated
// the resource. Might be empty when using static matchers.
func (f *gcpFetcher) GetDiscoveryConfigName() string {
	return f.cfg.DiscoveryConfigName
}

// Get returns GCP databases matching the fetcher's selectors.
func (f *gcpFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	projectIDs, err := f.getProjectIDs(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases types.Databases
	for _, projectID := range projectIDs {
		projectDatabases, err := f.getter(ctx, &f.cfg, projectID)
		if trace.IsAccessDenied(err) {
			// TODO(Tener): Create user tasks to fix permission issues.
			f.cfg.Logger.WarnContext(ctx, "Access denied to list databases", "project_id", projectID)
			continue
		} else if err != nil {
			return nil, trace.Wrap(err, "fetching databases for project %q", projectID)
		}
		databases = append(databases, projectDatabases...)
	}

	databases = filterDatabasesByLabels(ctx, databases, f.cfg.Labels, f.cfg.Logger)

	for _, db := range databases {
		common.ApplyGCPDatabaseNameSuffix(db, f.cfg.Type)
	}
	return databases.AsResources(), nil
}

// getProjectIDs returns the project IDs this fetcher queries: its configured
// project, or every visible project when that project is a wildcard.
func (f *gcpFetcher) getProjectIDs(ctx context.Context) ([]string, error) {
	if f.cfg.ProjectID != types.Wildcard {
		return []string{f.cfg.ProjectID}, nil
	}

	client, err := f.cfg.GCPClients.GetProjectsClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	projects, err := client.ListProjects(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(projects) == 0 {
		// The API docs say:
		//
		//	"Lists Projects that the caller has the `resourcemanager.projects.get`
		//	permission on and satisfy the specified filter."
		//
		// An empty result usually means missing permissions rather than an empty account.
		//
		// https://docs.cloud.google.com/resource-manager/reference/rest/v1/projects/list
		f.cfg.Logger.WarnContext(ctx, "Wildcard project matcher found no visible projects, check the resourcemanager.projects.get permission.")
	}
	projectIDs := make([]string, 0, len(projects))
	for _, project := range projects {
		projectIDs = append(projectIDs, project.ID)
	}
	return projectIDs, nil
}

// String returns the fetcher's string description.
func (f *gcpFetcher) String() string {
	return fmt.Sprintf("gcpFetcher(Type=%v, ProjectID=%v, Locations=%v, Labels=%v)",
		f.cfg.Type, f.cfg.ProjectID, f.cfg.Locations, f.cfg.Labels)
}
