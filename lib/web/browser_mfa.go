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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
)

// putBrowserMFA accepts a webauthn response from a Browser MFA attempt which is
// sent to CompleteBrowserMFAChallenge for verification. Once verified a tsh
// redirect URL with an encrypted webauthn response is returned.
func (h *Handler) putBrowserMFA(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	requestID := params.ByName("request_id")
	if requestID == "" {
		return "", trace.BadParameter("request is missing request ID")
	}

	var req client.MFAChallengeResponse
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	mfaResp, err := req.GetOptionalMFAResponseProtoReq()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if mfaResp == nil {
		return nil, trace.Errorf("mfa response is nil")
	}

	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.MFAServiceClient().CompleteBrowserMFAChallenge(r.Context(), &mfav2.CompleteBrowserMFAChallengeRequest{
		BrowserMfaResponse: &mfav1.BrowserMFAResponse{
			RequestId:        requestID,
			WebauthnResponse: mfaResp.GetWebauthn(),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.TshRedirectUrl, nil
}
