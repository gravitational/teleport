/*
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

package local_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func newUserPreferencesService(t *testing.T) *local.UserPreferencesService {
	t.Helper()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	return local.NewUserPreferencesService(backend)
}

func TestUserPreferencesCRUD2(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	defaultPref := local.DefaultUserPreferences()
	username := "something"

	tests := []struct {
		name     string
		req      *userpreferencesv1.UpsertUserPreferencesRequest
		expected *userpreferencesv1.UserPreferences
	}{
		{
			name:     "no existing preferences returns the default preferences",
			req:      nil,
			expected: defaultPref,
		},
		{
			name: "update the theme preference only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Username: username,
				Preferences: &userpreferencesv1.UserPreferences{
					Theme: userpreferencesv1.Theme_THEME_DARK,
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Assist:  defaultPref.Assist,
				Onboard: defaultPref.Onboard,
				Theme:   userpreferencesv1.Theme_THEME_DARK,
			},
		},
		{
			name: "update the assist preferred logins only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Username: username,
				Preferences: &userpreferencesv1.UserPreferences{
					Assist: &userpreferencesv1.AssistUserPreferences{
						PreferredLogins: []string{"foo", "bar"},
					},
					Onboard: &userpreferencesv1.OnboardUserPreferences{
						PreferredResources: []userpreferencesv1.Resource{},
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Theme:   defaultPref.Theme,
				Onboard: defaultPref.Onboard,
				Assist: &userpreferencesv1.AssistUserPreferences{
					PreferredLogins: []string{"foo", "bar"},
					ViewMode:        defaultPref.Assist.ViewMode,
				},
			},
		},
		{
			name: "update the assist view mode only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Username: username,
				Preferences: &userpreferencesv1.UserPreferences{
					Assist: &userpreferencesv1.AssistUserPreferences{
						ViewMode: userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE,
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Theme:   defaultPref.Theme,
				Onboard: defaultPref.Onboard,
				Assist: &userpreferencesv1.AssistUserPreferences{
					PreferredLogins: defaultPref.Assist.PreferredLogins,
					ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE,
				},
			},
		},
		{
			name: "update the onboard preference only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Username: username,
				Preferences: &userpreferencesv1.UserPreferences{
					Onboard: &userpreferencesv1.OnboardUserPreferences{
						PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_DATABASES},
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Assist: defaultPref.Assist,
				Theme:  defaultPref.Theme,
				Onboard: &userpreferencesv1.OnboardUserPreferences{
					PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_DATABASES},
				},
			},
		},
		{
			name: "update all the settings at once",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Username: username,
				Preferences: &userpreferencesv1.UserPreferences{
					Theme: userpreferencesv1.Theme_THEME_LIGHT,
					Assist: &userpreferencesv1.AssistUserPreferences{
						PreferredLogins: []string{"baz"},
						ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP,
					},
					Onboard: &userpreferencesv1.OnboardUserPreferences{
						PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_KUBERNETES},
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Theme: userpreferencesv1.Theme_THEME_LIGHT,
				Assist: &userpreferencesv1.AssistUserPreferences{
					PreferredLogins: []string{"baz"},
					ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP,
				},
				Onboard: &userpreferencesv1.OnboardUserPreferences{
					PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_KUBERNETES},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			identity := newUserPreferencesService(t)

			res, err := identity.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{
				Username: username,
			})
			require.NoError(t, err)
			require.Equal(t, defaultPref, res.Preferences)

			if test.req != nil {
				err := identity.UpsertUserPreferences(ctx, test.req)
				require.NoError(t, err)
			}

			res, err = identity.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{
				Username: username,
			})

			require.NoError(t, err)
			require.Empty(t, cmp.Diff(test.expected, res.Preferences,
				cmpopts.IgnoreUnexported(
					userpreferencesv1.UserPreferences{},
					userpreferencesv1.AssistUserPreferences{},
					userpreferencesv1.OnboardUserPreferences{}),
			))
		})
	}
}

func TestLayoutUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	identity := newUserPreferencesService(t)

	outdatedPrefs := &userpreferencesv1.UserPreferences{
		Assist: &userpreferencesv1.AssistUserPreferences{
			PreferredLogins: []string{"foo", "bar"},
		},
	}
	val, err := json.Marshal(outdatedPrefs)
	require.NoError(t, err)

	// Insert the outdated preferences directly into the backend
	// to simulate a previous version of the preferences.
	_, err = identity.Put(ctx, backend.Item{
		Key:   backend.Key("user_preferences", "test"),
		Value: val,
	})
	require.NoError(t, err)

	// Get the preferences and ensure that the layout is updated.
	prefs, err := identity.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{
		Username: "test",
	})
	require.NoError(t, err)
	// The layout should be updated to the latest version (values should not be nil).
	require.NotNil(t, prefs.Preferences.Onboard)
	// Non-existing values should be set to the default value.
	require.Equal(t, userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED, prefs.Preferences.Assist.ViewMode)
	require.Equal(t, userpreferencesv1.Theme_THEME_LIGHT, prefs.Preferences.Theme)
	// Existing values should be preserved.
	require.Equal(t, []string{"foo", "bar"}, prefs.Preferences.Assist.PreferredLogins)
}
