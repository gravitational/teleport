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
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

// requestUser is used to unmarshal JSON requests.
// Requests are made from the web UI for:
//	- user creation
type saveUserRequest struct {
	IsNew bool     `json:"isNew"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

// saveUser allows a UI user to create a new user or update an existing user.
//
// PUT /webapi/sites/:site/namespaces/:namespace/users
//
// Request:
// {
//		"isNew": true/false
//		"username": "foo",
//		"roles": ["role1", "role2"]
// }
//
// Response: { user_data }
func (h *Handler) saveUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, errSite := ctx.GetUserClient(site)
	if errSite != nil {
		return nil, trace.Wrap(errSite)
	}

	var req *saveUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var user services.User
	var err error

	if req.IsNew {
		// Create and insert new user.
		user, err = services.NewUser(req.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		user.SetRoles(req.Roles)
		user.SetCreatedBy(services.CreatedBy{
			User: services.UserRef{Name: ctx.user},
			Time: h.clock.Now().UTC(),
		})

		if err := clt.CreateUser(r.Context(), user); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Update existing user.
		user, err = clt.GetUser(req.Name, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		user.SetRoles(req.Roles)

		if err := ctx.clt.UpdateUser(r.Context(), user); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &ui.User{
		Name:    user.GetName(),
		Roles:   user.GetRoles(),
		Created: user.GetCreatedBy().Time,
	}, nil
}

// getUsers allows a UI user to retrieve a list of all locally saved users.
//
// GET /webapi/sites/:site/namespaces/:namespace/users
//
// Response: [ {user1}, {user2}... ]
func (h *Handler) getUsers(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localUsers, err := clt.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Iterate each user into UI compatible model.
	var users []ui.User
	for _, u := range localUsers {
		user := ui.User{
			Name:    u.GetName(),
			Roles:   u.GetRoles(),
			Created: u.GetCreatedBy().Time,
		}
		users = append(users, user)
	}

	return users, nil
}

// createResetPasswordToken allows a UI user to reset a user's password.
// This handler is also required for after creating new users.
//
// POST /webapi/sites/:site/namespaces/:namespace/users/password/token
//
// Request:
// {
//		"name": "foo"
//		"ttl": duration,
//		"type": "invite" || "password"
// }
//
// Response: { token_data }
func (h *Handler) createResetPasswordToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req auth.CreateResetPasswordTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := clt.CreateResetPasswordToken(r.Context(),
		auth.CreateResetPasswordTokenRequest{
			Name: req.Name,
			Type: req.Type,
		})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.ResetPasswordToken{
		URL:     token.GetURL(),
		Expiry:  token.Expiry(),
		TokenID: token.GetMetadata().Name,
		User:    token.GetUser(),
		Expires: token.Expiry().Sub(h.clock.Now().UTC()).Round(time.Second).String(),
	}, nil
}

// deleteUser allows a UI user to delete a existing user.
//
// DELETE /webapi/sites/:site/namespaces/:namespace/users/:username
//
// Response:
// {
//		"message": "ok"
// }
func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := p.ByName("username")
	if username == "" {
		return nil, trace.BadParameter("missing user name")
	}

	if err := clt.DeleteUser(r.Context(), username); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

// checkAndSetDefaults checks validity of a user request.
func (r *saveUserRequest) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing user name")
	}
	if len(r.Roles) == 0 {
		return trace.BadParameter("missing roles")
	}
	return nil
}
