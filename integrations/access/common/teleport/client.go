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

package teleport

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

// Client aggregates the parts of Teleport API client interface
// (as implemented by github.com/gravitational/teleport/api/client.Client)
// that are used by the access plugins.
type Client interface {
	plugindata.Client
	types.Events
	Ping(context.Context) (proto.PingResponse, error)
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
	SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
}
