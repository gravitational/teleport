/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

	"github.com/gravitational/teleport/lib/httplib"
)

// reportClientErrorRequest is the body of a POST to /webapi/log.
type reportClientErrorRequest struct {
	// Component identifies which part of the web UI is reporting the error
	Component string `json:"component"`
	// Message is a short, non-identifying description of what failed.
	Message string `json:"message"`
	// Metadata holds any additional caller-supplied context.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// reportClientErrorHandle logs client-side web UI errors so they show up alongside this proxy's other logs.
func (h *Handler) reportClientErrorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	var req reportClientErrorRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	h.logger.WarnContext(r.Context(), "Web UI client error",
		"source", "client",
		"component", req.Component,
		"message", req.Message,
		"user_agent", r.UserAgent(),
		"metadata", req.Metadata,
	)
	return nil, nil
}
