// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) registerBeamHandlers() {
	p.h.GET("/webapi/sites/:site/beams", p.h.WithClusterAuth(p.listBeams))
}

func (p *Plugin) listBeams(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	if !p.Config.Modules.Features().GetEntitlement(entitlements.Beams).Enabled {
		return nil, trace.AccessDenied("not authorized to use beams")
	}

	querystring := r.URL.Query()

	sortField, err := parseBeamsSortField(querystring.Get("sort_field"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sortOrder, err := parseBeamsSortOrder(querystring.Get("sort_dir"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request := beamsv1.ListBeamsRequest_builder{
		PageToken: querystring.Get("page_token"),
		SortField: sortField,
		SortOrder: sortOrder,
		Filters: beamsv1.ListBeamsRequest_Filters_builder{
			Users: querystring["user"],
		}.Build(),
	}.Build()

	if querystring.Has("page_size") {
		pageSize, err := strconv.ParseInt(querystring.Get("page_size"), 10, 32)
		if err != nil {
			return nil, trace.BadParameter("invalid page size")
		}
		request.SetPageSize(int32(pageSize))
	}

	lt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err, "getting user client")
	}

	resp, err := lt.BeamServiceClient().ListBeams(r.Context(), request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	beams := make([]beam, 0, len(resp.GetBeams()))
	for _, b := range resp.GetBeams() {
		beams = append(beams, beam{
			Name:    b.GetMetadata().GetName(),
			Alias:   b.GetStatus().GetAlias(),
			User:    b.GetStatus().GetUser(),
			Expires: b.GetSpec().GetExpires().AsTime(),
			NodeId:  b.GetStatus().GetNodeId(),
			AppName: b.GetStatus().GetAppName(),
		})
	}

	return listBeamsResponse{
		Items:         beams,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

type listBeamsResponse struct {
	Items         []beam `json:"items"`
	NextPageToken string `json:"next_page_token"`
}

type beam struct {
	Name    string    `json:"name"`
	Alias   string    `json:"alias"`
	User    string    `json:"user"`
	Expires time.Time `json:"expires"`
	NodeId  string    `json:"node_id"`
	AppName string    `json:"app_name"`
}

func parseBeamsSortField(field string) (beamsv1.BeamSortField, error) {
	switch field {
	case "name":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_NAME, nil
	case "alias":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_ALIAS, nil
	case "user":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_USER, nil
	case "expires":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_EXPIRES, nil
	case "":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_UNSPECIFIED, nil
	default:
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_UNSPECIFIED, trace.BadParameter("unsupported sort field %q but expected name, alias, user or expires", field)
	}
}

func parseBeamsSortOrder(order string) (beamsv1.BeamSortOrder, error) {
	switch strings.ToLower(order) {
	case "asc":
		return beamsv1.BeamSortOrder_BEAM_SORT_ORDER_ASCENDING, nil
	case "desc":
		return beamsv1.BeamSortOrder_BEAM_SORT_ORDER_DESCENDING, nil
	case "":
		return beamsv1.BeamSortOrder_BEAM_SORT_ORDER_UNSPECIFIED, nil
	default:
		return beamsv1.BeamSortOrder_BEAM_SORT_ORDER_UNSPECIFIED, trace.BadParameter("unsupported sort order %q but expected asc or desc", order)
	}
}
