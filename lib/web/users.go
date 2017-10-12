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
}

// changePassword updates users password based on the old password
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req *changePasswordReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userID := ctx.GetUser()
	err = clt.CheckPassword(userID, req.OldPassword, req.SecondFactorToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = services.VerifyPassword(req.NewPassword)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = clt.UpsertPassword(userID, req.NewPassword)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}
