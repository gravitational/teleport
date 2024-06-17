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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
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

	return getUsers(clt)
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

	return getUser(username, clt)
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

	// checkAndSetDefaults makes sure either TraitsPreset
	// or AllTraits field to be populated. Since empty
	// AllTraits is also used to delete all user traits,
	// we explicitly check if TraitsPreset is empty so
	// to prevent traits deletion.
	if req.TraitsPreset == nil {
		user.SetTraits(req.AllTraits)
	} else {
		updateUserTraitsPreset(req, user)
	}

	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: createdBy},
		Time: time.Now().UTC(),
	})

	if err := m.CreateUser(r.Context(), user); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(user)
}

// updateUserTraitsPreset receives a saveUserRequest and updates the user traits
// accordingly. It only updates the traits that have a non-nil value in
// saveUserRequest. This allows the partial update of the properties
func updateUserTraitsPreset(req *saveUserRequest, user types.User) {
	if req.TraitsPreset.Logins != nil {
		user.SetLogins(*req.TraitsPreset.Logins)
	}
	if req.TraitsPreset.DatabaseUsers != nil {
		user.SetDatabaseUsers(*req.TraitsPreset.DatabaseUsers)
	}
	if req.TraitsPreset.DatabaseNames != nil {
		user.SetDatabaseNames(*req.TraitsPreset.DatabaseNames)
	}
	if req.TraitsPreset.KubeUsers != nil {
		user.SetKubeUsers(*req.TraitsPreset.KubeUsers)
	}
	if req.TraitsPreset.KubeGroups != nil {
		user.SetKubeGroups(*req.TraitsPreset.KubeGroups)
	}
	if req.TraitsPreset.WindowsLogins != nil {
		user.SetWindowsLogins(*req.TraitsPreset.WindowsLogins)
	}
	if req.TraitsPreset.AWSRoleARNs != nil {
		user.SetAWSRoleARNs(*req.TraitsPreset.AWSRoleARNs)
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

	user, err := m.GetUser(req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRoles(req.Roles)

	// checkAndSetDefaults makes sure either TraitsPreset
	// or AllTraits field to be populated. Since empty
	// AllTraits is also used to delete all user traits,
	// we explicitly check if TraitsPreset is empty so
	// to prevent traits deletion.
	if req.TraitsPreset == nil {
		user.SetTraits(req.AllTraits)
	} else {
		updateUserTraitsPreset(req, user)
	}

	if err := m.UpdateUser(r.Context(), user); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(user)
}

func getUsers(m userAPIGetter) ([]ui.UserListEntry, error) {
	users, err := m.GetUsers(false)
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

func getUser(username string, m userAPIGetter) (*ui.User, error) {
	user, err := m.GetUser(username, false)
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
	GetUser(name string, withSecrets bool) (types.User, error)
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user types.User) error
	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user types.User) error
	// GetUsers returns a list of users
	GetUsers(withSecrets bool) ([]types.User, error)
	// DeleteUser deletes a user by name.
	DeleteUser(ctx context.Context, user string) error
}

// traitsPreset are user traits that are pre-defined in Teleport
type traitsPreset struct {
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
// - otherwise, we replace the list for that trait.
// Use TraitsPreset to selectively update traits.
// Use AllTraits to fully replace existing traits.
type saveUserRequest struct {
	// Name is username.
	Name string `json:"name"`
	// Roles is slice of user roles assigned to user.
	Roles []string `json:"roles"`
	// TraitsPreset holds traits that are pre-defined in Teleport.
	// Clients may use TraitsPreset to selectively update user traits.
	TraitsPreset *traitsPreset `json:"traits"`
	// AllTraits may hold all the user traits, including traits key defined
	// in TraitsPreset and/or new trait key values defined by Teleport admin.
	// AllTraits should be used to fully replace and update user traits.
	AllTraits map[string][]string `json:"allTraits"`
}

func (r *saveUserRequest) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing user name")
	}
	if len(r.Roles) == 0 {
		return trace.BadParameter("missing roles")
	}
	if len(r.AllTraits) != 0 && r.TraitsPreset != nil {
		return trace.BadParameter("either traits or allTraits must be provided")
	}
	return nil
}
