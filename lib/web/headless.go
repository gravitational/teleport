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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (h *Handler) getHeadless(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	headlessAuthenticationID := params.ByName("headless_authentication_id")
	if headlessAuthenticationID == "" {
		return nil, trace.NotFound("failed to find Headless session") // TODO this or just failed?
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headlessAuthn, err := authClient.GetHeadlessAuthentication(r.Context(), headlessAuthenticationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return headlessAuthn, nil
}

func (h *Handler) headlessLogin(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	headlessAuthenticationID := params.ByName("headless_authentication_id")
	if headlessAuthenticationID == "" {
		return nil, trace.NotFound("failed to find Headless session") // TODO this or just failed?
	}

	var req client.HeadlessRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{Webauthn: wanlib.CredentialAssertionResponseToProto(req.WebauthnAssertionResponse)},
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authClient.UpdateHeadlessAuthenticationState(r.Context(), headlessAuthenticationID, types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED, resp)
	if err != nil {
		return nil, trace.Wrap(err) // TODO replace with failed to authenticate always?
	}

	// TODO(jakule): webui expects JSON on POST ¯\_(ツ)_/¯
	w.Write([]byte("{\"status\": \"OK\"}"))

	return nil, nil
}
