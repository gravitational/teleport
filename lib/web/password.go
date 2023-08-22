/*
Copyright 2015 Gravitational, Inc.

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
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
)

// changePasswordReq is a request to change user password
type changePasswordReq struct {
	// OldPassword is user current password
	OldPassword []byte `json:"old_password"`
	// NewPassword is user new password
	NewPassword []byte `json:"new_password"`
	// SecondFactorToken is user 2nd factor token
	SecondFactorToken string `json:"second_factor_token"`
	// WebauthnAssertionResponse is a Webauthn response
	WebauthnAssertionResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse"`
}

// changePassword updates users password based on the old password.
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req *changePasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.ChangePasswordRequest{
		User:              ctx.GetUser(),
		OldPassword:       req.OldPassword,
		NewPassword:       req.NewPassword,
		SecondFactorToken: req.SecondFactorToken,
		Webauthn: wantypes.CredentialAssertionResponseToProto(
			req.WebauthnAssertionResponse,
		),
	}

	if err := clt.ChangePassword(r.Context(), protoReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// createAuthenticateChallengeWithPassword verifies given password for the authenticated user
// and on success returns MFA challenges for the users registered devices.
func (h *Handler) createAuthenticateChallengeWithPassword(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req client.MFAChallengeRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chal, err := clt.CreateAuthenticateChallenge(r.Context(), &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
			Username: ctx.GetUser(),
			Password: []byte(req.Pass),
		}},
	})
	if err != nil && trace.IsAccessDenied(err) {
		// logout in case of access denied
		logoutErr := h.logout(r.Context(), w, ctx)
		if logoutErr != nil {
			return nil, trace.Wrap(logoutErr)
		}
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeAuthenticateChallenge(chal), nil
}
