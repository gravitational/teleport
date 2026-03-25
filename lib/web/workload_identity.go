/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	tslices "github.com/gravitational/teleport/lib/utils/slices"
)

// listWorkloadIdentities returns a list of workload identities for a given
// cluster site.
func (h *Handler) listWorkloadIdentities(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request := &workloadidentityv1.ListWorkloadIdentitiesV2Request{
		PageSize:         20,
		PageToken:        r.URL.Query().Get("page_token"),
		SortField:        r.URL.Query().Get("sort_field"),
		FilterSearchTerm: r.URL.Query().Get("search"),
	}

	if r.URL.Query().Has("page_size") {
		pageSize, err := strconv.ParseInt(r.URL.Query().Get("page_size"), 10, 32)
		if err != nil {
			return nil, trace.BadParameter("invalid page size")
		}
		request.PageSize = int32(pageSize)
	}

	if r.URL.Query().Has("sort_dir") {
		sortDir := r.URL.Query().Get("sort_dir")
		request.SortDesc = strings.ToLower(sortDir) == "desc"
	}

	result, err := clt.WorkloadIdentityResourceServiceClient().ListWorkloadIdentitiesV2(r.Context(), request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiItems := tslices.Map(result.WorkloadIdentities, func(item *workloadidentityv1.WorkloadIdentity) WorkloadIdentity {
		uiItem := WorkloadIdentity{
			Name:       item.Metadata.Name,
			SpiffeID:   item.Spec.Spiffe.Id,
			SpiffeHint: item.Spec.Spiffe.Hint,
			Labels:     item.Metadata.Labels,
		}

		return uiItem
	})

	return ListWorkloadIdentitiesResponse{
		Items:         uiItems,
		NextPageToken: result.NextPageToken,
	}, nil
}

type ListWorkloadIdentitiesResponse struct {
	Items         []WorkloadIdentity `json:"items"`
	NextPageToken string             `json:"next_page_token,omitempty"`
}

type WorkloadIdentity struct {
	Name       string            `json:"name"`
	SpiffeID   string            `json:"spiffe_id"`
	SpiffeHint string            `json:"spiffe_hint"`
	Labels     map[string]string `json:"labels"`
}
