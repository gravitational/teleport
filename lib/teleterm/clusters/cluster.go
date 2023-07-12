/*
Copyright 2021 Gravitational, Inc.

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

package clusters

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// Cluster describes user settings and access to various resources.
type Cluster struct {
	// URI is the cluster URI
	URI uri.ResourceURI
	// Name is the cluster name
	Name string
	// ProfileName is the name of the tsh profile
	ProfileName string
	// RootClusterName is the root cluster name.
	RootClusterName string
	// Log is a component logger
	Log *logrus.Entry
	// dir is the directory where cluster certificates are stored
	dir string
	// Status is the cluster status
	status client.ProfileStatus
	// client is the cluster Teleport client
	clusterClient *client.TeleportClient
	// clock is a clock for time-related operations
	clock clockwork.Clock
}

type ClusterWithDetails struct {
	*Cluster
	// Auth server features
	Features *proto.Features
	// AuthClusterID is the unique cluster ID that is set once
	// during the first auth server startup.
	AuthClusterID string
	// SuggestedReviewers for the given user.
	SuggestedReviewers []string
	// RequestableRoles for the given user.
	RequestableRoles []string
	// ACL contains user access control list.
	ACL *api.ACL
}

// Connected indicates if connection to the cluster can be established
func (c *Cluster) Connected() bool {
	return c.status.Name != "" && !c.status.IsExpired(c.clock)
}

// GetWithDetails makes requests to the auth server to return details of the current
// Cluster that cannot be found on the disk only, including details about the user
// and enabled enterprise features. This method requires a valid cert.
func (c *Cluster) GetWithDetails(ctx context.Context) (*ClusterWithDetails, error) {
	var (
		pingResponse  proto.PingResponse
		caps          *types.AccessCapabilities
		authClusterID string
		acl           *api.ACL
	)

	err := addMetadataToRetryableError(ctx, func() error {
		proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err := proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

		pingResponse, err = authClient.Ping(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		caps, err = authClient.GetAccessCapabilities(ctx, types.AccessCapabilitiesRequest{
			RequestableRoles:   true,
			SuggestedReviewers: true,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		clusterName, err := authClient.GetClusterName()
		if err != nil {
			return trace.Wrap(err)
		}
		authClusterID = clusterName.GetClusterID()

		user, err := authClient.GetCurrentUser(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		roles, err := authClient.GetCurrentUserRoles(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		roleSet := services.NewRoleSet(roles...)
		userACL := services.NewUserACL(user, roleSet, *pingResponse.ServerFeatures, false)

		acl = &api.ACL{
			RecordedSessions: convertToAPIResourceAccess(userACL.RecordedSessions),
			ActiveSessions:   convertToAPIResourceAccess(userACL.ActiveSessions),
			AuthConnectors:   convertToAPIResourceAccess(userACL.AuthConnectors),
			Roles:            convertToAPIResourceAccess(userACL.Roles),
			Users:            convertToAPIResourceAccess(userACL.Users),
			TrustedClusters:  convertToAPIResourceAccess(userACL.TrustedClusters),
			Events:           convertToAPIResourceAccess(userACL.Events),
			Tokens:           convertToAPIResourceAccess(userACL.Tokens),
			Servers:          convertToAPIResourceAccess(userACL.Nodes),
			Apps:             convertToAPIResourceAccess(userACL.AppServers),
			Dbs:              convertToAPIResourceAccess(userACL.DBServers),
			Kubeservers:      convertToAPIResourceAccess(userACL.KubeServers),
			AccessRequests:   convertToAPIResourceAccess(userACL.AccessRequests),
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	withDetails := &ClusterWithDetails{
		Cluster:            c,
		SuggestedReviewers: caps.SuggestedReviewers,
		RequestableRoles:   caps.RequestableRoles,
		Features:           pingResponse.ServerFeatures,
		AuthClusterID:      authClusterID,
		ACL:                acl,
	}

	return withDetails, nil
}

func convertToAPIResourceAccess(access services.ResourceAccess) *api.ResourceAccess {
	return &api.ResourceAccess{
		List:   access.List,
		Read:   access.Read,
		Edit:   access.Edit,
		Create: access.Create,
		Delete: access.Delete,
		Use:    access.Use,
	}
}

// GetRoles returns currently logged-in user roles
func (c *Cluster) GetRoles(ctx context.Context) ([]*types.Role, error) {
	var roles []*types.Role
	err := addMetadataToRetryableError(ctx, func() error {
		proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		for _, name := range c.status.Roles {
			role, err := proxyClient.GetRole(ctx, name)
			if err != nil {
				return trace.Wrap(err)
			}
			roles = append(roles, &role)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return roles, nil
}

// GetRequestableRoles returns the requestable roles for the currently logged-in user
func (c *Cluster) GetRequestableRoles(ctx context.Context, req *api.GetRequestableRolesRequest) (*types.AccessCapabilities, error) {
	var (
		authClient  auth.ClientI
		proxyClient *client.ProxyClient
		err         error
		response    *types.AccessCapabilities
	)

	resourceIds := make([]types.ResourceID, 0, len(req.GetResourceIds()))
	for _, r := range req.GetResourceIds() {
		resourceIds = append(resourceIds, types.ResourceID{
			ClusterName:     r.ClusterName,
			Kind:            r.Kind,
			Name:            r.Name,
			SubResourceName: r.SubResourceName,
		})
	}

	err = addMetadataToRetryableError(ctx, func() error {
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err = proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

		response, err = authClient.GetAccessCapabilities(ctx, types.AccessCapabilitiesRequest{
			ResourceIDs:      resourceIds,
			RequestableRoles: true,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

// GetLoggedInUser returns currently logged-in user
func (c *Cluster) GetLoggedInUser() LoggedInUser {
	return LoggedInUser{
		Name:           c.status.Username,
		SSHLogins:      c.status.Logins,
		Roles:          c.status.Roles,
		ActiveRequests: c.status.ActiveRequests.AccessRequests,
	}
}

// GetProxyHost returns proxy address (host:port) of the cluster
func (c *Cluster) GetProxyHost() string {
	return c.status.ProxyURL.Host
}

// LoggedInUser is the currently logged-in user
type LoggedInUser struct {
	// Name is the user name
	Name string
	// SSHLogins is the user sshlogins
	SSHLogins []string
	// Roles is the user roles
	Roles []string
	// ActiveRequests is the user active requests
	ActiveRequests []string
}

// addMetadataToRetryableError is Connect's equivalent of client.RetryWithRelogin. By adding the
// metadata to the error, we're letting the Electron app know that the given error was caused by
// expired certs and letting the user log in again should resolve the error upon another attempt.
func addMetadataToRetryableError(ctx context.Context, fn func() error) error {
	err := fn()
	if err == nil {
		return nil
	}

	if client.IsErrorResolvableWithRelogin(err) {
		trailer := metadata.Pairs("is-resolvable-with-relogin", "1")
		grpc.SetTrailer(ctx, trailer)
	}

	return trace.Wrap(err)
}
