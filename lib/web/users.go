/*
Copyright 2020 Gravitational, Inc.

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
	"context"
	"net/http"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

// requestUser is used to pull out data from JSON requests.
type requestUser struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

// responseUserPasswordToken is used to send back data
// about created user and password setup token.
type responseUserPasswordToken struct {
	User  *services.UserV2               `json:"user"`
	Token *services.ResetPasswordTokenV3 `json:"token"`
}

// createUser allows UI users to create new users.
//
// POST /webapi/sites/:site/namespaces/:namespace/users
//
// Request:
// {
//		"username": "foo",
//		"roles": ["role1", "role2"]
// }
//
// Response:
// {
//		"user": {...},
//		"token": {...}
// }
func (h *Handler) createUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	// Pull out request data.
	var req *requestUser
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkUserParameters(*req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check user doesn't exist already.
	if _, err := ctx.clt.GetUser(req.Name, false); err == nil {
		return nil, trace.BadParameter("user %q already registered", req.Name)
	}

	// Create and insert new user.
	newUser, err := services.NewUser(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	newUser.SetRoles(req.Roles)
	newUser.SetCreatedBy(services.CreatedBy{
		User: services.UserRef{Name: ctx.user},
		Time: h.clock.Now().UTC(),
	})

	if err := ctx.clt.CreateUser(context.TODO(), newUser); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create sign up token.
	resetPassToken, err := ctx.clt.CreateResetPasswordToken(context.TODO(), auth.CreateResetPasswordTokenRequest{
		Name: req.Name,
		Type: auth.ResetPasswordTokenTypeInvite,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, ok := newUser.(*services.UserV2)
	if !ok {
		return nil, trace.BadParameter("unsupported user type: %T", user)
	}

	token, ok := resetPassToken.(*services.ResetPasswordTokenV3)
	if !ok {
		return nil, trace.BadParameter("unsupported reset password token type: %T", token)

	}

	return &responseUserPasswordToken{
		User:  user,
		Token: token,
	}, nil
}

// checkUserParameters checks validity of all parameters of a user request.
func checkUserParameters(user requestUser) error {
	if user.Name == "" {
		return trace.BadParameter("missing parameter user name")
	}
	if len(user.Roles) == 0 {
		return trace.BadParameter("missing parameter roles")
	}
	return nil
}
