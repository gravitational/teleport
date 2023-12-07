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

package integration

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
)

// Client is a wrapper around *client.Client with some additional methods helpful for testing.
type Client struct {
	*client.Client
}

// SubmitAccessRequestReview is a simpler version of SubmitAccessReview.
func (api *Client) SubmitAccessRequestReview(ctx context.Context, reqID string, review types.AccessReview) error {
	_, err := api.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: reqID,
		Review:    review,
	})
	return trace.Wrap(err)
}

// ApproveAccessRequest sets an access request state to APPROVED.
func (api *Client) ApproveAccessRequest(ctx context.Context, reqID, reason string) error {
	update := types.AccessRequestUpdate{
		RequestID: reqID,
		State:     types.RequestState_APPROVED,
		Reason:    reason,
	}
	return api.SetAccessRequestState(ctx, update)
}

// ApproveAccessRequest sets an access request state to DENIED.
func (api *Client) DenyAccessRequest(ctx context.Context, reqID, reason string) error {
	update := types.AccessRequestUpdate{
		RequestID: reqID,
		State:     types.RequestState_DENIED,
		Reason:    reason,
	}
	return api.SetAccessRequestState(ctx, update)
}

// GetAccessRequest loads an access request.
func (api *Client) GetAccessRequest(ctx context.Context, reqID string) (types.AccessRequest, error) {
	requests, err := api.GetAccessRequests(ctx, types.AccessRequestFilter{ID: reqID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(requests) == 0 {
		return nil, trace.NotFound("request %q is not found", reqID)
	}
	return requests[0], nil
}

// PollAccessRequestPluginData waits until plugin data for a give request became available.
func (api *Client) PollAccessRequestPluginData(ctx context.Context, plugin, reqID string) (map[string]string, error) {
	filter := types.PluginDataFilter{
		Kind:     types.KindAccessRequest,
		Resource: reqID,
		Plugin:   plugin,
	}
	for {
		pluginDatas, err := api.GetPluginData(ctx, filter)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(pluginDatas) > 0 {
			pluginData := pluginDatas[0]
			entry := pluginData.Entries()[plugin]
			if entry != nil {
				return entry.Data, nil
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// SearchAccessRequestEvents searches for recent access request events in audit log.
func (api *Client) SearchAccessRequestEvents(ctx context.Context, reqID string) ([]*events.AccessRequestCreate, error) {
	auditEvents, _, err := api.SearchEvents(
		ctx,
		time.Now().UTC().AddDate(0, -1, 0),
		time.Now().UTC(),
		"default",
		[]string{"access_request.update"},
		100,
		types.EventOrderAscending,
		"",
	)
	result := make([]*events.AccessRequestCreate, 0, len(auditEvents))
	for _, event := range auditEvents {
		if event, ok := event.(*events.AccessRequestCreate); ok && event.RequestID == reqID {
			result = append(result, event)
		}
	}
	return result, trace.Wrap(err)
}
