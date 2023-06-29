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
	"github.com/sirupsen/logrus"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/lib/backend"
)

// UserPreferencesService is responsible for managing a user's preferences.
type UserPreferencesService struct {
	backend.Backend
	log logrus.FieldLogger
}

// DefaultUserPreferences is the default user preferences.
var DefaultUserPreferences = &userpreferencesv1.UserPreferences{
	Assist: &userpreferencesv1.AssistUserPreferences{
		PreferredLogins: []string{},
		ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED,
	},
	Theme: userpreferencesv1.Theme_THEME_LIGHT,
}

// NewUserPreferencesService returns a new instance of the UserPreferencesService.
func NewUserPreferencesService(backend backend.Backend) *UserPreferencesService {
	return &UserPreferencesService{
		Backend: backend,
		log:     logrus.WithField(trace.Component, "userpreferences"),
	}
}

// GetUserPreferences returns the user preferences for the given user.
func (u *UserPreferencesService) GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error) {
	preferences, err := u.getUserPreferences(ctx, req.Username)
	if err != nil {
		if trace.IsNotFound(err) {
			return &userpreferencesv1.GetUserPreferencesResponse{Preferences: DefaultUserPreferences}, nil
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

	// track whether we need to create or update the user preferences record
	create := false

	preferences, err := u.getUserPreferences(ctx, req.Username)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		create = true
		preferences = DefaultUserPreferences
	}

	preferences = mergePreferences(preferences, req.Preferences)

	item, err := createBackendItem(req.Username, preferences)
	if err != nil {
		return trace.Wrap(err)
	}

	if create {
		_, err = u.Create(ctx, item)
	} else {
		_, err = u.Update(ctx, item)
	}

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

// mergePreferences merges the given preferences.
func mergePreferences(a, b *userpreferencesv1.UserPreferences) *userpreferencesv1.UserPreferences {
	if b.Theme != userpreferencesv1.Theme_THEME_UNSPECIFIED {
		a.Theme = b.Theme
	}

	if b.Assist != nil {
		a.Assist = mergeAssistUserPreferences(a.Assist, b.Assist)
	}

	return a
}

// mergeAssistUserPreferences merges the given assist user preferences.
func mergeAssistUserPreferences(a, b *userpreferencesv1.AssistUserPreferences) *userpreferencesv1.AssistUserPreferences {
	if b.PreferredLogins != nil {
		a.PreferredLogins = b.PreferredLogins
	}

	if b.ViewMode != userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_UNSPECIFIED {
		a.ViewMode = b.ViewMode
	}

	return a
}
