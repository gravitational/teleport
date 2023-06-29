/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/lib/httplib"
)

// AssistUserPreferencesResponse is the JSON response for the assist user preferences.
type AssistUserPreferencesResponse struct {
	PreferredLogins []string                         `json:"preferredLogins"`
	ViewMode        userpreferencesv1.AssistViewMode `json:"viewMode"`
}

// UserPreferencesResponse is the JSON response for the user preferences.
type UserPreferencesResponse struct {
	Assist AssistUserPreferencesResponse `json:"assist"`
	Theme  userpreferencesv1.Theme       `json:"theme"`
}

// getUserPreferences is a handler for GET /webapi/user/preferences
func (h *Handler) getUserPreferences(_ http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := authClient.GetUserPreferences(r.Context(), &userpreferencesv1.GetUserPreferencesRequest{
		Username: sctx.GetUser(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userPreferencesResponse(resp.Preferences), nil
}

// updateUserPreferences is a handler for PUT /webapi/user/preferences.
func (h *Handler) updateUserPreferences(_ http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext) (any, error) {
	req := UserPreferencesResponse{}

	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	preferences := &userpreferencesv1.UpsertUserPreferencesRequest{
		Username: sctx.GetUser(),
		Preferences: &userpreferencesv1.UserPreferences{
			Theme: req.Theme,
			Assist: &userpreferencesv1.AssistUserPreferences{
				PreferredLogins: req.Assist.PreferredLogins,
				ViewMode:        req.Assist.ViewMode,
			},
		},
	}

	if err := authClient.UpsertUserPreferences(r.Context(), preferences); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// userPreferencesResponse creates a JSON response for the user preferences.
func userPreferencesResponse(resp *userpreferencesv1.UserPreferences) *UserPreferencesResponse {
	jsonResp := &UserPreferencesResponse{
		Assist: assistUserPreferencesResponse(resp.Assist),
		Theme:  resp.Theme,
	}

	return jsonResp
}

// assistUserPreferencesResponse creates a JSON response for the assist user preferences.
func assistUserPreferencesResponse(resp *userpreferencesv1.AssistUserPreferences) AssistUserPreferencesResponse {
	jsonResp := AssistUserPreferencesResponse{
		PreferredLogins: make([]string, 0, len(resp.PreferredLogins)),
		ViewMode:        resp.ViewMode,
	}

	jsonResp.PreferredLogins = append(jsonResp.PreferredLogins, resp.PreferredLogins...)

	return jsonResp
}
