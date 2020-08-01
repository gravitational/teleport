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
	"net/http"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

// requestUser is a request used for user actions from the web UI:
//	- user creation
type requestUser struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

// responseCreateUser is used to send back data
// about created user and password setup token.
type responseCreateUser struct {
	User  ui.User      `json:"user"`
	Token ui.UserToken `json:"token"`
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

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check user doesn't exist already.
	_, err := ctx.clt.GetUser(req.Name, false)
	if !trace.IsNotFound(err) {
		if err != nil {
			return nil, trace.Wrap(err, "failed to check whether user %q exists: %v", req.Name, err)
		}
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

	if err := ctx.clt.CreateUser(r.Context(), newUser); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create sign up token.
	token, err := ctx.clt.CreateResetPasswordToken(r.Context(), auth.CreateResetPasswordTokenRequest{
		Name: req.Name,
		Type: auth.ResetPasswordTokenTypeInvite,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &responseCreateUser{
		User: ui.User{
			Name:    newUser.GetName(),
			Roles:   newUser.GetRoles(),
			Created: newUser.GetCreatedBy().Time,
		},
		Token: ui.UserToken{
			Name:    token.GetUser(),
			URL:     token.GetURL(),
			Created: token.GetCreated(),
			Expires: token.Expiry(),
		},
	}, nil
}

// checkAndSetDefaults checks validity of a user request.
func (r *requestUser) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing user name")
	}
	if len(r.Roles) == 0 {
		return trace.BadParameter("missing roles")
	}
	return nil
}
