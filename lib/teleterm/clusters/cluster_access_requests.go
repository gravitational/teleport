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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

type AccessRequest struct {
	URI uri.ResourceURI
	types.AccessRequest
}

// Returns all access requests available to the user.
func (c *Cluster) GetAccessRequests(ctx context.Context, req types.AccessRequestFilter) ([]AccessRequest, error) {
	var (
		requests []types.AccessRequest
		err      error
	)
	err = addMetadataToRetryableError(ctx, func() error {
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
			ClusterName: resource.ClusterName,
			Name:        resource.Name,
			Kind:        resource.Kind,
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

	err = addMetadataToRetryableError(ctx, func() error {
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

		return authClient.DeleteAccessRequest(ctx, req.AccessRequestId)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *Cluster) AssumeRole(ctx context.Context, req *api.AssumeRoleRequest) error {
	var err error

	err = addMetadataToRetryableError(ctx, func() error {
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

		return c.clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, params)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.clusterClient.SaveProfile(c.dir, true)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
