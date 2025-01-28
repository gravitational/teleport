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

package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	container "cloud.google.com/go/container/apiv1"
	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/gravitational/trace"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
)

const (
	// kubernetesEngineScope is the GCP Kubernetes Engine Scope for OAuth2.
	// https://developers.google.com/identity/protocols/oauth2/scopes#container
	kubernetesEngineScope = "https://www.googleapis.com/auth/cloud-platform"
)

// GKEClient is an interface to interact with GCP Clusters.
type GKEClient interface {
	// ListClusters lists the GCP GKE clusters that belong to the projectID and are
	// located in location.
	// location supports wildcard "*".
	ListClusters(ctx context.Context, projectID string, location string) ([]GKECluster, error)
	// GetClusterRestConfig returns the Kubernetes client config to connect to the
	// specified cluster. The access token is based on the default credentials configured
	// for the current GCP Service Account and must include the following permissions:
	// - container.clusters.get
	// - container.clusters.impersonate
	// - container.clusters.list
	// - container.pods.get
	// - container.selfSubjectAccessReviews.create
	// - container.selfSubjectRulesReviews.create
	// It also returns the token expiration time from which the token is no longer valid.
	GetClusterRestConfig(ctx context.Context, cfg ClusterDetails) (*rest.Config, time.Time, error)
}

// GKEClientConfig is the client configuration for GKEClient.
type GKEClientConfig struct {
	// ClusterClient is the GCP client for container service.
	ClusterClient gcpGKEClient
	// TokenSource is the OAuth2 token generator for Google auth.
	// The scope must include the kubernetesEngineScope.
	TokenSource oauth2.TokenSource
}

// CheckAndSetDefaults check and set defaults for GKEClientConfig.
func (c *GKEClientConfig) CheckAndSetDefaults(ctx context.Context) (err error) {
	if c.TokenSource == nil {
		if c.TokenSource, err = google.DefaultTokenSource(ctx, kubernetesEngineScope); err != nil {
			return trace.Wrap(err)
		}
	}
	if c.ClusterClient == nil {
		if c.ClusterClient, err = container.NewClusterManagerClient(ctx); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// gcpGKEClient is a subset of container.ClusterManagerClient methods used
// in this package.
type gcpGKEClient interface {
	ListClusters(ctx context.Context, req *containerpb.ListClustersRequest, opts ...gax.CallOption) (*containerpb.ListClustersResponse, error)
	GetCluster(ctx context.Context, req *containerpb.GetClusterRequest, opts ...gax.CallOption) (*containerpb.Cluster, error)
}

// make sure container.ClusterManagerClient satisfies GCPGKEClient interface.
var _ gcpGKEClient = &container.ClusterManagerClient{}

// GKECluster represents a GKE cluster and contains the information necessary
// for Teleport Discovery to decide whether or not to import the cluster.
type GKECluster struct {
	// Name is the cluster name.
	Name string
	// Description is the cluster description field in GCP.
	Description string
	// Location is the cluster location.
	Location string
	// ProjectID is the GCP project ID to which the cluster belongs.
	ProjectID string
	// Status is the cluster current status.
	Status containerpb.Cluster_Status
	// Labels are the cluster labels in GCP.
	Labels map[string]string
}

// ClusterDetails is the cluster identification properties.
type ClusterDetails struct {
	// ProjectID is the GCP project ID to which the cluster belongs.
	ProjectID string
	// Locations are the cluster locations.
	Location string
	// Name is the cluster name.
	Name string
}

// CheckAndSetDefaults check and set defaults for ClusterDetails.
func (c *ClusterDetails) CheckAndSetDefaults() error {
	if len(c.ProjectID) == 0 {
		return trace.BadParameter("ProjectID must be set")
	}
	if len(c.Location) == 0 {
		return trace.BadParameter("Location must be set")
	}
	if c.Location == types.Wildcard {
		return trace.BadParameter("Location does not support wildcards")
	}
	if len(c.Name) == 0 {
		return trace.BadParameter("Name must be set")
	}
	return nil
}

// toGCPEndpointName generates a GCP endpoint identifier with the following
// format: projects/*/locations/*/clusters/*.
func (c *ClusterDetails) toGCPEndpointName() string {
	return fmt.Sprintf("projects/%s/locations/%s/clusters/%s", c.ProjectID, c.Location, c.Name)
}

// NewGKEClient returns a GKEClient interface wrapping container.ClusterManagerClient and
// oauth2.TokenSource for interacting with GCP Kubernetes Service.
func NewGKEClient(ctx context.Context) (GKEClient, error) {
	var cfg GKEClientConfig
	client, err := NewGKEClientWithConfig(ctx, cfg)
	return client, trace.Wrap(err)
}

// NewGKEClientWithConfig returns a GKEClient interface wrapping
// container.ClusterManagerClient and oauth2.TokenSource for interacting with GCP
// Kubernetes Service.
func NewGKEClientWithConfig(ctx context.Context, cfg GKEClientConfig) (GKEClient, error) {
	if err := cfg.CheckAndSetDefaults(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &gkeClient{cfg}, nil
}

// gkeClient implements the GKEClient interface by wrapping container.ClusterManagerClient
// and oauth2.TokenSource for interacting with GCP Kubernetes Service.
type gkeClient struct {
	GKEClientConfig
}

// ListClusters lists the GCP GKE clusters that belong to the projectID and are
// located in location.
// location supports wildcard "*".
func (g *gkeClient) ListClusters(ctx context.Context, projectID string, location string) ([]GKECluster, error) {
	if len(projectID) == 0 {
		return nil, trace.BadParameter("projectID must be set")
	}
	if len(location) == 0 {
		return nil, trace.BadParameter("location must be set")
	}

	res, err := g.GKEClientConfig.ClusterClient.ListClusters(
		ctx,
		&containerpb.ListClustersRequest{
			Parent: fmt.Sprintf("projects/%s/locations/%s", projectID, convertLocationToGCP(location)),
		},
	)
	if err != nil {
		return nil, trace.Wrap(convertAPIError(err))
	}
	var clusters []GKECluster
	for _, cluster := range res.Clusters {
		clusters = append(clusters, GKECluster{
			Name:        cluster.Name,
			Description: cluster.Description,
			ProjectID:   projectID,
			Labels:      cluster.ResourceLabels,
			Status:      cluster.Status,
			Location:    cluster.Location,
		})
	}

	return clusters, nil
}

// GetClusterRestConfig returns the Kubernetes client config to connect to the
// specified cluster. The access token is based on the default credentials configured
// for the current GCP Service Account and must include the following permissions:
// - container.clusters.get
// - container.clusters.impersonate
// - container.clusters.list
// - container.pods.get
// - container.selfSubjectAccessReviews.create
// - container.selfSubjectRulesReviews.create
// It also returns the token expiration time from which the token is no longer valid.
func (g *gkeClient) GetClusterRestConfig(ctx context.Context, cfg ClusterDetails) (*rest.Config, time.Time, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	// Get cluster from cloud to extract the CA certificate.
	res, err := g.GKEClientConfig.ClusterClient.GetCluster(
		ctx,
		&containerpb.GetClusterRequest{
			Name: cfg.toGCPEndpointName(),
		},
	)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(convertAPIError(err))
	}

	// Generate a SA Authentication Token.
	token, err := g.GKEClientConfig.TokenSource.Token()
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	restCfg, err := getTLSConfig(res, token.AccessToken)

	return restCfg, token.Expiry, trace.Wrap(err)
}

// convertLocationToGCP checks if the location is a Teleport wildcard `*` and
// replaces it with the GCP wildcard, otherwise returns the location.
func convertLocationToGCP(location string) string {
	if location == types.Wildcard {
		// gcp location wildcard is a "-"
		location = "-"
	}
	return location
}

// getTLSConfig creates a rest.Config for the given cluster with the specified
// Bearer token for authentication.
func getTLSConfig(cluster *containerpb.Cluster, tok string) (*rest.Config, error) {
	if cluster.MasterAuth == nil {
		return nil, trace.BadParameter("cluster.MasterAuth was not set and is required")
	}
	ca, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &rest.Config{
		Host:        fmt.Sprintf("https://%s", cluster.Endpoint),
		BearerToken: tok,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}, nil
}
