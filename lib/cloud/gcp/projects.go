/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package gcp

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/api/cloudresourcemanager/v1"
)

// Project is a GCP project.
type Project struct {
	// ID is the project ID.
	ID string
	// Name is the project name.
	Name string
}

// ProjectsClient is an interface to interact with GCP Projects API.
type ProjectsClient interface {
	// ListProjects lists the GCP projects that the authenticated user has access to.
	ListProjects(ctx context.Context) ([]Project, error)
}

// ProjectsClientConfig is the client configuration for ProjectsClient.
type ProjectsClientConfig struct {
	// Client is the GCP client for resourcemanager service.
	Client *cloudresourcemanager.Service
}

// CheckAndSetDefaults check and set defaults for ProjectsClientConfig.
func (c *ProjectsClientConfig) CheckAndSetDefaults(ctx context.Context) (err error) {
	if c.Client == nil {
		c.Client, err = cloudresourcemanager.NewService(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NewProjectsClient returns a ProjectsClient interface wrapping resourcemanager.ProjectsClient
// for interacting with GCP Projects API.
func NewProjectsClient(ctx context.Context) (ProjectsClient, error) {
	var cfg ProjectsClientConfig
	client, err := NewProjectsClientWithConfig(ctx, cfg)
	return client, trace.Wrap(err)
}

// NewProjectsClientWithConfig returns a ProjectsClient interface wrapping resourcemanager.ProjectsClient
// for interacting with GCP Projects API.
func NewProjectsClientWithConfig(ctx context.Context, cfg ProjectsClientConfig) (ProjectsClient, error) {
	if err := cfg.CheckAndSetDefaults(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &projectsClient{cfg}, nil
}

type projectsClient struct {
	ProjectsClientConfig
}

// ListProjects lists the GCP Projects that the authenticated user has access to.
func (g *projectsClient) ListProjects(ctx context.Context) ([]Project, error) {

	var pageToken string
	var projects []Project
	for {
		projectsCall, err := g.Client.Projects.List().PageToken(pageToken).Do()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, project := range projectsCall.Projects {
			projects = append(projects,
				Project{
					ID:   project.ProjectId,
					Name: project.Name,
				},
			)
		}
		if projectsCall.NextPageToken == "" {
			break
		}
		pageToken = projectsCall.NextPageToken
	}

	return projects, nil
}
