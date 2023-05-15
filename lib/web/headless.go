/*

 Copyright 2023 Gravitational, Inc.

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
	"github.com/gravitational/teleport/api/types"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
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
		h.log.Debug("failed to get headless session: %v", err)

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
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	var action types.HeadlessAuthenticationState
	var resp = &proto.MFAAuthenticateResponse{}

	switch req.Action {
	case "accept":
		action = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
		resp = &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wanlib.CredentialAssertionResponseToProto(req.WebauthnAssertionResponse),
			},
		}
	case "denied":
		action = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED
	default:
		return nil, trace.BadParameter("unknown action %s", req.Action)
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authClient.UpdateHeadlessAuthenticationState(r.Context(), headlessAuthenticationID,
		action, resp)
	if err != nil {
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
