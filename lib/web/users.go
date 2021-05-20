/*
Copyright 2021 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

func (h *Handler) updateUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return updateUser(r, clt, ctx.GetUser())
}

func (h *Handler) createUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createUser(r, clt, ctx.GetUser())
}

func (h *Handler) getUsersHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getUsers(clt)
}

func (h *Handler) deleteUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := deleteUser(r, params, clt, ctx.GetUser()); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

func createUser(r *http.Request, m userAPIGetter, createdBy string) (*ui.User, error) {
	var req *saveUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := services.NewUser(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRoles(req.Roles)
	user.SetTraits(map[string][]string{
		teleport.TraitLogins: req.Logins,
	})

	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: createdBy},
		Time: time.Now().UTC(),
	})

	if err := m.CreateUser(r.Context(), user); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(user)
}

func updateUser(r *http.Request, m userAPIGetter, createdBy string) (*ui.User, error) {
	var req *saveUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := m.GetUser(req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles(req.Roles)

	if err := m.UpdateUser(r.Context(), user); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(user)
}

func getUsers(m userAPIGetter) ([]ui.User, error) {
	users, err := m.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var uiUsers []ui.User
	for _, u := range users {
		uiuser, err := ui.NewUser(u)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		uiUsers = append(uiUsers, *uiuser)
	}

	return uiUsers, nil
}

func deleteUser(r *http.Request, params httprouter.Params, m userAPIGetter, user string) error {
	username := params.ByName("username")
	if username == "" {
		return trace.BadParameter("missing user name")
	}

	if username == user {
		return trace.BadParameter("cannot delete own user account")
	}

	if err := m.DeleteUser(r.Context(), username); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type userAPIGetter interface {
	// GetUser returns user by name
	GetUser(name string, withSecrets bool) (services.User, error)
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user services.User) error
	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user services.User) error
	// GetUsers returns a list of users
	GetUsers(withSecrets bool) ([]services.User, error)
	// DeleteUser deletes a user by name.
	DeleteUser(ctx context.Context, user string) error
}

type saveUserRequest struct {
	Name   string   `json:"name"`
	Roles  []string `json:"roles"`
	Logins []string `json:"logins,omitempty"`
}

func (r *saveUserRequest) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing user name")
	}
	if len(r.Roles) == 0 {
		return trace.BadParameter("missing roles")
	}
	return nil
}
