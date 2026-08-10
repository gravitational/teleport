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

package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/httplib"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/web"
)

// createPreUserEventHandle sends a user event to the UserEvent service
// this handler is for on-boarding user events pre-session
func (h *Handler) createPreUserEventHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req usagereporter.CreatePreUserEventRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	client := h.cfg.ProxyClient

	typedEvent, err := usagereporter.ConvertPreUserEventRequestToUsageEvent(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &proto.SubmitUsageEventRequest{
		Event: typedEvent,
	}

	err = client.SubmitUsageEvent(r.Context(), event)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// createUserEventHandle sends a user event to the UserEvent service
// this handler is for user events with a session
func (h *Handler) createUserEventHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (interface{}, error) {
	var req usagereporter.CreateUserEventRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	typedEvent, err := usagereporter.ConvertUserEventRequestToUsageEvent(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &proto.SubmitUsageEventRequest{
		Event: typedEvent,
	}

	err = client.SubmitUsageEvent(r.Context(), event)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}
