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
	"context"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web/ui"
)

func (h *Handler) updateUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return updateUser(r, clt)
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

	return getUsers(r.Context(), clt)
}

func (h *Handler) getUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := params.ByName("username")
	if username == "" {
		return nil, trace.BadParameter("missing username")
	}

	return getUser(r.Context(), username, clt)
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

	user, err := types.NewUser(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRoles(req.Roles)

	updateUserTraits(req, user)

	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: createdBy},
		Time: time.Now().UTC(),
	})

	created, err := m.CreateUser(r.Context(), user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(created)
}

// updateUserTraits receives a saveUserRequest and updates the user traits accordingly
// It only updates the traits that have a non-nil value in saveUserRequest
// This allows the partial update of the properties
func updateUserTraits(req *saveUserRequest, user types.User) {
	if req.Traits.Logins != nil {
		user.SetLogins(*req.Traits.Logins)
	}
	if req.Traits.DatabaseUsers != nil {
		user.SetDatabaseUsers(*req.Traits.DatabaseUsers)
	}
	if req.Traits.DatabaseNames != nil {
		user.SetDatabaseNames(*req.Traits.DatabaseNames)
	}
	if req.Traits.KubeUsers != nil {
		user.SetKubeUsers(*req.Traits.KubeUsers)
	}
	if req.Traits.KubeGroups != nil {
		user.SetKubeGroups(*req.Traits.KubeGroups)
	}
	if req.Traits.WindowsLogins != nil {
		user.SetWindowsLogins(*req.Traits.WindowsLogins)
	}
	if req.Traits.AWSRoleARNs != nil {
		user.SetAWSRoleARNs(*req.Traits.AWSRoleARNs)
	}
}

func updateUser(r *http.Request, m userAPIGetter) (*ui.User, error) {
	var req *saveUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove the MFA resp from the context before getting the user.
	// Otherwise, it will be consumed before the Update which actually
	// requires the MFA.
	// TODO(Joerger): Explicitly provide MFA response only where it is
	// needed instead of removing it like this.
	getUserCtx := mfa.ContextWithMFAResponse(r.Context(), nil)
	user, err := m.GetUser(getUserCtx, req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRoles(req.Roles)

	updateUserTraits(req, user)

	updated, err := m.UpdateUser(r.Context(), user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(updated)
}

func getUsers(ctx context.Context, m userAPIGetter) ([]ui.UserListEntry, error) {
	users, err := m.GetUsers(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var uiUsers []ui.UserListEntry
	for _, u := range users {
		// Do not display system users in the WebUI
		if types.IsSystemResource(u) {
			continue
		}
		uiuser, err := ui.NewUserListEntry(u)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		uiUsers = append(uiUsers, *uiuser)
	}

	return uiUsers, nil
}

func getUser(ctx context.Context, username string, m userAPIGetter) (*ui.User, error) {
	user, err := m.GetUser(ctx, username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiUser, err := ui.NewUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return uiUser, nil
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

type privilegeTokenRequest struct {
	// SecondFactorToken is the totp code.
	SecondFactorToken string `json:"secondFactorToken"`
	// WebauthnResponse is the response from authenticators.
	WebauthnResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse"`
}

// createPrivilegeTokenHandle creates and returns a privilege token.
func (h *Handler) createPrivilegeTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req privilegeTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.CreatePrivilegeTokenRequest{}

	switch {
	case req.SecondFactorToken != "":
		protoReq.ExistingMFAResponse = &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: req.SecondFactorToken},
		}}
	case req.WebauthnResponse != nil:
		protoReq.ExistingMFAResponse = &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(req.WebauthnResponse),
		}}
	default:
		// Can be empty, which means user did not have a second factor registered.
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := clt.CreatePrivilegeToken(r.Context(), protoReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return token.GetName(), nil
}

type userAPIGetter interface {
	// GetUser returns user by name
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user types.User) (types.User, error)
	// GetUsers returns a list of users
	GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error)
	// DeleteUser deletes a user by name.
	DeleteUser(ctx context.Context, user string) error
}

type userTraits struct {
	Logins        *[]string `json:"logins,omitempty"`
	DatabaseUsers *[]string `json:"databaseUsers,omitempty"`
	DatabaseNames *[]string `json:"databaseNames,omitempty"`
	KubeUsers     *[]string `json:"kubeUsers,omitempty"`
	KubeGroups    *[]string `json:"kubeGroups,omitempty"`
	WindowsLogins *[]string `json:"windowsLogins,omitempty"`
	AWSRoleARNs   *[]string `json:"awsRoleArns,omitempty"`
}

// saveUserRequest represents a create/update request for a user
// Name and Roles are always required
// The remaining fields are part of the Trait map
// They are optional and respect the following logic:
// - if the value is nil, we ignore it
// - if the value is an empty array we remove every element from the trait
// - otherwise, we replace the list for that trait
type saveUserRequest struct {
	Name   string     `json:"name"`
	Roles  []string   `json:"roles"`
	Traits userTraits `json:"traits"`
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
