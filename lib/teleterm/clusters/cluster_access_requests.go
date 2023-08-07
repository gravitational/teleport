// Copyright 2022 Gravitational, Inc
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

package clusters

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

type ResourceDetails struct {
	Hostname     string
	FriendlyName string
}

type AccessRequest struct {
	URI uri.ResourceURI
	types.AccessRequest
	ResourceDetails map[string]ResourceDetails
}

// GetAccessRequest returns a specific access request by ID and includes resource details
func (c *Cluster) GetAccessRequest(ctx context.Context, req types.AccessRequestFilter) (*AccessRequest, error) {
	var (
		request         types.AccessRequest
		resourceDetails map[string]ResourceDetails
		proxyClient     *client.ProxyClient
		authClient      auth.ClientI
		err             error
	)

	err = AddMetadataToRetryableError(ctx, func() error {
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		requests, err := proxyClient.GetAccessRequests(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		// This has to happen inside this scope because we need access to the authClient
		// We can remove this once we make the change to keep around the proxy and auth clients
		if len(requests) < 1 {
			return trace.NotFound("Access request not found.")
		}
		request = requests[0]

		authClient, err = proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

		resourceDetails, err = getResourceDetails(ctx, request, authClient)

		return err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessRequest{
		URI:             c.URI.AppendAccessRequest(request.GetName()),
		AccessRequest:   request,
		ResourceDetails: resourceDetails,
	}, nil
}

// Returns all access requests available to the user.
func (c *Cluster) GetAccessRequests(ctx context.Context, req types.AccessRequestFilter) ([]AccessRequest, error) {
	var (
		requests []types.AccessRequest
		err      error
	)
	err = AddMetadataToRetryableError(ctx, func() error {
		requests, err = c.clusterClient.GetAccessRequests(ctx, req)
		return err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := []AccessRequest{}
	for _, request := range requests {
		results = append(results, AccessRequest{
			URI:           c.URI.AppendAccessRequest(request.GetName()),
			AccessRequest: request,
		})
	}

	return results, nil
}

// Creates an access request.
func (c *Cluster) CreateAccessRequest(ctx context.Context, req *api.CreateAccessRequestRequest) (*AccessRequest, error) {
	var (
		err     error
		request types.AccessRequest
	)

	resourceIDs := make([]types.ResourceID, 0, len(req.ResourceIds))
	for _, resource := range req.ResourceIds {
		resourceIDs = append(resourceIDs, types.ResourceID{
			ClusterName:     resource.ClusterName,
			Name:            resource.Name,
			Kind:            resource.Kind,
			SubResourceName: resource.SubResourceName,
		})
	}

	// Role-based and Resource-based AccessRequests are mutually exclusive.
	if len(req.ResourceIds) > 0 {
		request, err = services.NewAccessRequestWithResources(c.status.Username, req.Roles, resourceIDs)
	} else {
		request, err = services.NewAccessRequest(c.status.Username, req.Roles...)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request.SetRequestReason(req.Reason)
	request.SetSuggestedReviewers(req.SuggestedReviewers)

	err = AddMetadataToRetryableError(ctx, func() error {
		return c.clusterClient.CreateAccessRequest(ctx, request)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessRequest{
		URI:           c.URI.AppendAccessRequest(request.GetName()),
		AccessRequest: request,
	}, nil
}

func (c *Cluster) ReviewAccessRequest(ctx context.Context, req *api.ReviewAccessRequestRequest) (*AccessRequest, error) {
	var (
		err            error
		authClient     auth.ClientI
		proxyClient    *client.ProxyClient
		updatedRequest types.AccessRequest
	)

	var reviewState types.RequestState
	if err := reviewState.Parse(req.State); err != nil {
		return nil, trace.Wrap(err)
	}

	err = AddMetadataToRetryableError(ctx, func() error {
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

		reviewSubmission := types.AccessReviewSubmission{
			RequestID: req.AccessRequestId,
			Review: types.AccessReview{
				Roles:         req.Roles,
				ProposedState: reviewState,
				Reason:        req.Reason,
				Created:       time.Now(),
			},
		}

		updatedRequest, err = authClient.SubmitAccessReview(ctx, reviewSubmission)

		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessRequest{
		URI:           c.URI.AppendAccessRequest(updatedRequest.GetName()),
		AccessRequest: updatedRequest,
	}, nil
}

func (c *Cluster) DeleteAccessRequest(ctx context.Context, req *api.DeleteAccessRequestRequest) error {
	var (
		err         error
		authClient  auth.ClientI
		proxyClient *client.ProxyClient
	)

	err = AddMetadataToRetryableError(ctx, func() error {
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

		return authClient.DeleteAccessRequest(ctx, req.AccessRequestId)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *Cluster) AssumeRole(ctx context.Context, req *api.AssumeRoleRequest) error {
	var err error

	err = AddMetadataToRetryableError(ctx, func() error {
		params := client.ReissueParams{
			AccessRequests:     req.AccessRequestIds,
			DropAccessRequests: req.DropRequestIds,
			RouteToCluster:     c.clusterClient.SiteName,
		}

		// keep existing access requests that aren't included in the droprequests
		for _, reqID := range c.status.ActiveRequests.AccessRequests {
			if !slices.Contains(req.DropRequestIds, reqID) {
				params.AccessRequests = append(params.AccessRequests, reqID)
			}
		}
		// When assuming a role, we want to drop all cached certs otherwise
		// tsh will continue to use the old certs.
		return c.clusterClient.ReissueUserCerts(ctx, client.CertCacheDrop, params)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.clusterClient.SaveProfile(true)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func getResourceDetails(ctx context.Context, req types.AccessRequest, clt auth.ClientI) (map[string]ResourceDetails, error) {
	resourceIDsByCluster := services.GetResourceIDsByCluster(req)

	resourceDetails := make(map[string]ResourceDetails)
	for clusterName, resourceIDs := range resourceIDsByCluster {
		details, err := services.GetResourceDetails(ctx, clusterName, clt, resourceIDs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for id, d := range details {
			resourceDetails[id] = ResourceDetails{
				FriendlyName: d.FriendlyName,
			}
		}
	}

	return resourceDetails, nil
}
