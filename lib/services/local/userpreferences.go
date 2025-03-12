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

package local

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/reflect/protoreflect"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/lib/backend"
)

// UserPreferencesService is responsible for managing a user's preferences.
type UserPreferencesService struct {
	backend.Backend
}

func DefaultUserPreferences() *userpreferencesv1.UserPreferences {
	return &userpreferencesv1.UserPreferences{
		Theme: userpreferencesv1.Theme_THEME_UNSPECIFIED,
		UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
			DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_ALL,
			ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_CARD,
			LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
			AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_NONE,
		},
		Onboard: &userpreferencesv1.OnboardUserPreferences{
			PreferredResources: []userpreferencesv1.Resource{},
			MarketingParams:    &userpreferencesv1.MarketingParams{},
		},
		ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
			PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{},
		},
		SideNavDrawerMode: userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_COLLAPSED,
	}
}

// NewUserPreferencesService returns a new instance of the UserPreferencesService.
func NewUserPreferencesService(backend backend.Backend) *UserPreferencesService {
	return &UserPreferencesService{
		Backend: backend,
	}
}

// GetUserPreferences returns the user preferences for the given user.
func (u *UserPreferencesService) GetUserPreferences(ctx context.Context, username string) (*userpreferencesv1.UserPreferences, error) {
	preferences, err := u.getUserPreferences(ctx, username)
	if err != nil {
		if trace.IsNotFound(err) {
			return DefaultUserPreferences(), nil
		}

		return nil, trace.Wrap(err)
	}

	return preferences, nil
}

// UpsertUserPreferences creates or updates user preferences for a given username.
func (u *UserPreferencesService) UpsertUserPreferences(ctx context.Context, username string, prefs *userpreferencesv1.UserPreferences) error {
	if username == "" {
		return trace.BadParameter("missing username")
	}
	if err := validatePreferences(prefs); err != nil {
		return trace.Wrap(err)
	}

	item, err := createBackendItem(username, prefs)
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

	// Apply the default values to the existing preferences.
	// This allows updating the preferences schema without returning empty values
	// for new fields in the existing preferences.
	df := DefaultUserPreferences()
	if err := overwriteValues(df, &p); err != nil {
		return nil, trace.Wrap(err)
	}

	return df, nil
}

// backendKey returns the backend key for the user preferences for the given username.
func backendKey(username string) backend.Key {
	return backend.NewKey(userPreferencesPrefix, username)
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
	settingsKey := backend.NewKey(userPreferencesPrefix, username)

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

// overwriteValues overwrites the values in dst with the values in src.
// This function uses proto.Ranges internally to iterate over the fields in src.
// Because of this, only non-nil/empty fields in src will overwrite the values in dst.
func overwriteValues(dst, src protoreflect.ProtoMessage) error {
	d := dst.ProtoReflect()
	s := src.ProtoReflect()

	dName := d.Descriptor().FullName().Name()
	sName := s.Descriptor().FullName().Name()
	// If the names don't match, then the types don't match, so we can't overwrite.
	if dName != sName {
		return trace.BadParameter("dst and src must be the same type")
	}

	overwriteValuesRecursive(d, s)

	return nil
}

// overwriteValuesRecursive recursively overwrites the values in dst with the values in src.
// It's a helper function for overwriteValues.
func overwriteValuesRecursive(dst, src protoreflect.Message) {
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.Message() != nil:
			overwriteValuesRecursive(dst.Mutable(fd).Message(), src.Get(fd).Message())
		default:
			dst.Set(fd, src.Get(fd))
		}

		return true
	})
}
