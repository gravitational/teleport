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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
)

const headlessAuthID = "headless_authentication_id"

func (h *Handler) getHeadless(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	headlessAuthenticationID, err := getHeadlessAuthID(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headlessAuthn, err := authClient.GetHeadlessAuthentication(r.Context(), headlessAuthenticationID)
	if err != nil {
		// Log the error, but return something more user-friendly.
		// Context exceeded or invalid request states are more confusing than helpful.
		h.logger.DebugContext(r.Context(), "failed to get headless session", "error", err)

		return nil, trace.BadParameter("requested invalid headless session")
	}

	return headlessAuthn, nil
}

func (h *Handler) putHeadlessState(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	headlessAuthenticationID, err := getHeadlessAuthID(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req client.HeadlessRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.MFAResponse == nil && req.WebauthnAssertionResponse != nil {
		req.MFAResponse = &client.MFAChallengeResponse{
			WebauthnResponse: req.WebauthnAssertionResponse,
		}
	}

	mfaResp, err := req.MFAResponse.GetOptionalMFAResponseProtoReq()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var action types.HeadlessAuthenticationState
	switch req.Action {
	case "accept":
		action = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
	case "denied":
		action = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED
	default:
		return nil, trace.BadParameter("unknown action %s", req.Action)
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err = authClient.UpdateHeadlessAuthenticationState(r.Context(), headlessAuthenticationID, action, mfaResp); err != nil {
		return nil, trace.Wrap(err)
	}

	// WebUI expects a JSON response.
	return OK(), nil
}

func getHeadlessAuthID(params httprouter.Params) (string, error) {
	headlessAuthenticationID := params.ByName(headlessAuthID)
	if headlessAuthenticationID == "" {
		return "", trace.BadParameter("request is missing headless authentication ID")
	}
	return headlessAuthenticationID, nil
}
