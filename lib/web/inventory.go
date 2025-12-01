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
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	inventoryv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/inventory/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

func splitQuery(value string) []string {
	values := map[string]struct{}{}
	for v := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			values[trimmed] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(values))
}

// listUnifiedInstancesResponse is the response for listing unified instances
type listUnifiedInstancesResponse struct {
	// Instances is the list of unified instances (both instances and bot instances)
	Instances []ui.UnifiedInstance `json:"instances"`
	// StartKey is the next page token
	StartKey string `json:"startKey"`
}

// clusterUnifiedInstancesGet returns a paginated list of unified instances
func (h *Handler) clusterUnifiedInstancesGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := r.URL.Query()

	limit, err := QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	startKey := values.Get("startKey")

	// Default values
	sort := inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_NAME
	order := inventoryv1.SortOrder_SORT_ORDER_ASCENDING
	if sortParam := values.Get("sort"); sortParam != "" {
		parts := strings.SplitN(sortParam, ":", 2)
		fieldName := strings.ToLower(parts[0])
		switch fieldName {
		case "name", "hostname":
			sort = inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_NAME
		case "type":
			sort = inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_TYPE
		case "version":
			sort = inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION
		}
		if len(parts) == 2 {
			direction := strings.ToLower(parts[1])
			if direction == "desc" || direction == "descending" {
				order = inventoryv1.SortOrder_SORT_ORDER_DESCENDING
			}
		}
	}

	filter := &inventoryv1.ListUnifiedInstancesFilter{
		Search:              values.Get("search"),
		PredicateExpression: values.Get("query"),
		Services:            splitQuery(values.Get("services")),
		Upgraders:           splitQuery(values.Get("upgraders")),
		UpdaterGroups:       splitQuery(values.Get("updaterGroups")),
	}

	var hasInstance, hasBotInstance bool
	for t := range strings.SplitSeq(values.Get("types"), ",") {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "instance":
			hasInstance = true
		case "bot_instance":
			hasBotInstance = true
		}

		if hasInstance && hasBotInstance {
			break
		}
	}
	if hasInstance {
		filter.InstanceTypes = append(filter.InstanceTypes, inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE)
	}
	if hasBotInstance {
		filter.InstanceTypes = append(filter.InstanceTypes, inventoryv1.InstanceType_INSTANCE_TYPE_BOT_INSTANCE)
	}

	resp, err := clt.ListUnifiedInstances(r.Context(), &inventoryv1.ListUnifiedInstancesRequest{
		PageSize:  limit,
		PageToken: startKey,
		Sort:      sort,
		Order:     order,
		Filter:    filter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiInstances := make([]ui.UnifiedInstance, 0, len(resp.Items))
	for _, item := range resp.Items {
		uiInstances = append(uiInstances, ui.MakeUnifiedInstance(item))
	}

	return &listUnifiedInstancesResponse{
		Instances: uiInstances,
		StartKey:  resp.NextPageToken,
	}, nil
}
