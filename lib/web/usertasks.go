/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

// userTaskStateUpdate updates the state of a User Task.
func (h *Handler) userTaskStateUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	userTaskName := p.ByName("name")
	if userTaskName == "" {
		return nil, trace.BadParameter("a user task name is required")
	}

	var req *ui.UpdateUserTaskStateRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userTask, err := clt.UserTasksServiceClient().GetUserTask(r.Context(), userTaskName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userTask.Spec.State = req.State

	newUserTask, err := clt.UserTasksServiceClient().UpsertUserTask(r.Context(), userTask)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeUserTask(newUserTask), nil
}

// userTaskGet returns a User Task based on its name
func (h *Handler) userTaskGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	userTaskName := p.ByName("name")
	if userTaskName == "" {
		return nil, trace.BadParameter("a user task name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ut, err := clt.UserTasksServiceClient().GetUserTask(r.Context(), userTaskName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDetailedUserTask(ut), nil
}

// userTaskListByIntegration returns a page of User Tasks.
// It requires a query param to filter by integration
//
// The following query params are optional:
// - limit: max number of items
// - startKey: used to iterate over pages
//
// It returns a list of user tasks with the base attributes (common among all user tasks).
// To get a detailed UserTask use the single resource endpoint, ie, usertask/<resource's name>.
func (h *Handler) userTaskListByIntegration(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := r.URL.Query()
	limit, err := QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	startKey := values.Get("startKey")

	integrationName := values.Get("integration")
	if integrationName == "" {
		return nil, trace.BadParameter("integration query param is required")
	}

	taskStateFilter := values.Get("state")

	filters := &usertasksv1.ListUserTasksFilters{
		Integration: integrationName,
		TaskState:   taskStateFilter,
	}
	userTasks, nextKey, err := clt.UserTasksServiceClient().ListUserTasks(r.Context(), int64(limit), startKey, filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	items := make([]ui.UserTask, 0, len(userTasks))
	for _, userTask := range userTasks {
		items = append(items, ui.MakeUserTask(userTask))
	}

	return ui.UserTasksListResponse{
		Items:   items,
		NextKey: nextKey,
	}, nil
}
