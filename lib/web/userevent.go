/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

	return nil, nil
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

	return nil, nil
}
