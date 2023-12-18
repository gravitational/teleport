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
