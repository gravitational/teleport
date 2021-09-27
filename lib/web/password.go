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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// changePasswordReq is a request to change user password
type changePasswordReq struct {
	// OldPassword is user current password
	OldPassword []byte `json:"old_password"`
	// NewPassword is user new password
	NewPassword []byte `json:"new_password"`
	// SecondFactorToken is user 2nd factor token
	SecondFactorToken string `json:"second_factor_token"`
	// U2FSignResponse is U2F response
	U2FSignResponse *u2f.AuthenticateChallengeResponse `json:"u2f_sign_response"`
	// WebauthnResponse is a Webauthn response
	WebauthnResponse *wanlib.CredentialAssertionResponse `json:"webauthn_response"`
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

	servicedReq := services.ChangePasswordReq{
		User:              ctx.GetUser(),
		OldPassword:       req.OldPassword,
		NewPassword:       req.NewPassword,
		SecondFactorToken: req.SecondFactorToken,
		U2FSignResponse:   req.U2FSignResponse,
		WebauthnResponse:  req.WebauthnResponse,
	}

	if err := clt.ChangePassword(servicedReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// u2fChangePasswordRequest is called to get U2F challedge for changing a user password
func (h *Handler) u2fChangePasswordRequest(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req *client.MFAChallengeRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chal, err := clt.CreateAuthenticateChallenge(r.Context(), &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
			Username: req.User,
			Password: []byte(req.Pass),
		}},
	})
	if err != nil && trace.IsAccessDenied(err) {
		// logout in case of access denied
		logoutErr := h.logout(w, ctx)
		if logoutErr != nil {
			return nil, trace.Wrap(logoutErr)
		}
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.MakeAuthenticateChallenge(chal), nil
}
