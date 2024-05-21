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

package clusters

import (
	"context"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
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
func (c *Cluster) GetAccessRequest(ctx context.Context, rootAuthClient authclient.ClientI, req types.AccessRequestFilter) (*AccessRequest, error) {
	var (
		request         types.AccessRequest
		resourceDetails map[string]ResourceDetails
		err             error
	)

	err = AddMetadataToRetryableError(ctx, func() error {
		requests, err := rootAuthClient.GetAccessRequests(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		// This has to happen inside this scope because we need access to the authClient
		// We can remove this once we make the change to keep around the proxy and auth clients
		if len(requests) < 1 {
			return trace.NotFound("Access request not found.")
		}
		request = requests[0]

		resourceDetails, err = getResourceDetails(ctx, request, rootAuthClient)

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
func (c *Cluster) GetAccessRequests(ctx context.Context, rootAuthClient authclient.ClientI, req types.AccessRequestFilter) ([]AccessRequest, error) {
	var (
		requests []types.AccessRequest
		err      error
	)
	err = AddMetadataToRetryableError(ctx, func() error {
		requests, err = rootAuthClient.GetAccessRequests(ctx, req)
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
func (c *Cluster) CreateAccessRequest(ctx context.Context, rootAuthClient authclient.ClientI, req *api.CreateAccessRequestRequest) (*AccessRequest, error) {
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
	request.SetDryRun(req.DryRun)

	if req.MaxDuration != nil {
		request.SetMaxDuration(req.MaxDuration.AsTime())
	}
	if req.RequestTtl != nil {
		request.SetExpiry(req.RequestTtl.AsTime())
	}
	if req.GetAssumeStartTime() != nil {
		request.SetAssumeStartTime(req.AssumeStartTime.AsTime())
	}

	var reqOut types.AccessRequest
	err = AddMetadataToRetryableError(ctx, func() error {
		reqOut, err = rootAuthClient.CreateAccessRequestV2(ctx, request)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessRequest{
		URI:           c.URI.AppendAccessRequest(request.GetName()),
		AccessRequest: reqOut,
	}, nil
}

func (c *Cluster) ReviewAccessRequest(ctx context.Context, rootAuthClient authclient.ClientI, req *api.ReviewAccessRequestRequest) (*AccessRequest, error) {
	var (
		err            error
		updatedRequest types.AccessRequest
	)

	var reviewState types.RequestState
	if err := reviewState.Parse(req.State); err != nil {
		return nil, trace.Wrap(err)
	}

	err = AddMetadataToRetryableError(ctx, func() error {
		var assumeStartTimePtr *time.Time
		if req.AssumeStartTime != nil {
			assumeStartTime := req.AssumeStartTime.AsTime()
			assumeStartTimePtr = &assumeStartTime
		}

		reviewSubmission := types.AccessReviewSubmission{
			RequestID: req.AccessRequestId,
			Review: types.AccessReview{
				Roles:           req.Roles,
				ProposedState:   reviewState,
				Reason:          req.Reason,
				Created:         time.Now(),
				AssumeStartTime: assumeStartTimePtr,
			},
		}

		updatedRequest, err = rootAuthClient.SubmitAccessReview(ctx, reviewSubmission)

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

func (c *Cluster) DeleteAccessRequest(ctx context.Context, rootAuthClient authclient.ClientI, req *api.DeleteAccessRequestRequest) error {
	err := AddMetadataToRetryableError(ctx, func() error {
		return rootAuthClient.DeleteAccessRequest(ctx, req.AccessRequestId)
	})
	return trace.Wrap(err)
}

func (c *Cluster) AssumeRole(ctx context.Context, rootProxyClient *client.ProxyClient, req *api.AssumeRoleRequest) error {
	err := AddMetadataToRetryableError(ctx, func() error {
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
		return rootProxyClient.ReissueUserCerts(ctx, client.CertCacheDrop, params)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.clusterClient.SaveProfile(true)
	return trace.Wrap(err)
}

func getResourceDetails(ctx context.Context, req types.AccessRequest, rootAuthClient authclient.ClientI) (map[string]ResourceDetails, error) {
	resourceIDsByCluster := accessrequest.GetResourceIDsByCluster(req)

	resourceDetails := make(map[string]ResourceDetails)
	for clusterName, resourceIDs := range resourceIDsByCluster {
		details, err := accessrequest.GetResourceDetails(ctx, clusterName, rootAuthClient, resourceIDs)
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
