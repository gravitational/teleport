/*
 *
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
 *
 */

package local

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/lib/backend"
)

// UserPreferencesService is responsible for managing a user's preferences.
type UserPreferencesService struct {
	backend.Backend
}

func DefaultUserPreferences() *userpreferencesv1.UserPreferences {
	return &userpreferencesv1.UserPreferences{
		Assist: &userpreferencesv1.AssistUserPreferences{
			PreferredLogins: nil,
			ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED,
		},
		Theme: userpreferencesv1.Theme_THEME_LIGHT,
		Onboard: &userpreferencesv1.OnboardUserPreferences{
			PreferredResources: nil,
		},
	}
}

// NewUserPreferencesService returns a new instance of the UserPreferencesService.
func NewUserPreferencesService(backend backend.Backend) *UserPreferencesService {
	return &UserPreferencesService{
		Backend: backend,
	}
}

// GetUserPreferences returns the user preferences for the given user.
func (u *UserPreferencesService) GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error) {
	preferences, err := u.getUserPreferences(ctx, req.Username)
	if err != nil {
		if trace.IsNotFound(err) {
			return &userpreferencesv1.GetUserPreferencesResponse{Preferences: DefaultUserPreferences()}, nil
		}

		return nil, trace.Wrap(err)
	}

	return &userpreferencesv1.GetUserPreferencesResponse{Preferences: preferences}, nil
}

// UpsertUserPreferences creates or updates user preferences for a given username.
func (u *UserPreferencesService) UpsertUserPreferences(ctx context.Context, req *userpreferencesv1.UpsertUserPreferencesRequest) error {
	if req.Username == "" {
		return trace.BadParameter("missing username")
	}
	if err := validatePreferences(req.Preferences); err != nil {
		return trace.Wrap(err)
	}

	preferences, err := u.getUserPreferences(ctx, req.Username)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		preferences = DefaultUserPreferences()
	}

	mergePreferences(preferences, req.Preferences)

	item, err := createBackendItem(req.Username, preferences)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = u.Put(ctx, item)

	return trace.Wrap(err)
}

// getUserPreferences returns the user preferences for the given username.
func (u *UserPreferencesService) getUserPreferences(ctx context.Context, username string) (*userpreferencesv1.UserPreferences, error) {
	existing, err := u.Get(ctx, backendKey(username))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var p userpreferencesv1.UserPreferences
	if err := json.Unmarshal(existing.Value, &p); err != nil {
		return nil, trace.Wrap(err)
	}

	return &p, nil
}

// backendKey returns the backend key for the user preferences for the given username.
func backendKey(username string) []byte {
	return backend.Key(userPreferencesPrefix, username)
}

// validatePreferences validates the given preferences.
func validatePreferences(preferences *userpreferencesv1.UserPreferences) error {
	if preferences == nil {
		return trace.BadParameter("missing preferences")
	}

	return nil
}

// createBackendItem creates a backend.Item for the given username and user preferences.
func createBackendItem(username string, preferences *userpreferencesv1.UserPreferences) (backend.Item, error) {
	settingsKey := backend.Key(userPreferencesPrefix, username)

	payload, err := json.Marshal(preferences)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   settingsKey,
		Value: payload,
	}

	return item, nil
}

// mergePreferences merges the values from src into dest.
func mergePreferences(dest, src *userpreferencesv1.UserPreferences) {
	if src.Theme != userpreferencesv1.Theme_THEME_UNSPECIFIED {
		dest.Theme = src.Theme
	}

	if src.Assist != nil {
		mergeAssistUserPreferences(dest.Assist, src.Assist)
	}

	if src.Onboard != nil {
		mergeOnboardUserPreferences(dest.Onboard, src.Onboard)
	}

}

// mergeAssistUserPreferences merges src preferences into the given dest assist user preferences.
func mergeAssistUserPreferences(dest, src *userpreferencesv1.AssistUserPreferences) {
	if src.PreferredLogins != nil {
		dest.PreferredLogins = src.PreferredLogins
	}

	if src.ViewMode != userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_UNSPECIFIED {
		dest.ViewMode = src.ViewMode
	}
}

// mergeOnboardUserPreferences merges src preferences into the given dest onboard user preferences.
func mergeOnboardUserPreferences(dest, src *userpreferencesv1.OnboardUserPreferences) {
	if src.PreferredResources != nil {
		dest.PreferredResources = src.PreferredResources
	}
}
