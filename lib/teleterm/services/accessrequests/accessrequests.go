// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package accessrequests

import (
	"context"

	"github.com/gravitational/trace"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

func PromoteAccessRequest(ctx context.Context, client Client, req *accesslistv1.AccessRequestPromoteRequest) (*types.AccessRequestV3, error) {
	response, err := client.AccessRequestPromote(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response.AccessRequest, nil
}

func GetSuggestedAccessLists(ctx context.Context, client Client, accessRequestID string) ([]*accesslist.AccessList, error) {
	response, err := client.GetSuggestedAccessLists(ctx, accessRequestID)
	return response, trace.Wrap(err)
}

// Client represents services.AccessLists methods used by [PromoteAccessRequest] and [GetSuggestedAccessLists].
// During a normal operation, authClient.AccessListClient is passed as this interface.
type Client interface {
	// See services.AccessListsSuggestionsGetter.AccessRequestPromote.
	AccessRequestPromote(ctx context.Context, req *accesslistv1.AccessRequestPromoteRequest) (*accesslistv1.AccessRequestPromoteResponse, error)
	// See services.AccessLists.AccessRequestPromote.
	GetSuggestedAccessLists(ctx context.Context, accessRequestID string) ([]*accesslist.AccessList, error)
}
