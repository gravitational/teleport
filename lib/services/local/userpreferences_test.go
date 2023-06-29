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
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
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

func TestUserPreferencesCRUD(t *testing.T) {
	t.Parallel()

	identity := newUserPreferencesService(t)
	ctx := context.Background()

	const username = "foo"

	t.Run("no existing preferences returns the default preferences", func(t *testing.T) {
		req := &userpreferencesv1.GetUserPreferencesRequest{
			Username: username,
		}

		res, err := identity.GetUserPreferences(ctx, req)

		require.NoError(t, err)
		require.Equal(t, local.DefaultUserPreferences, res.Preferences)
	})

	t.Run("update the theme preference only", func(t *testing.T) {
		req := &userpreferencesv1.UpsertUserPreferencesRequest{
			Username: username,
			Preferences: &userpreferencesv1.UserPreferences{
				Theme: userpreferencesv1.Theme_THEME_DARK,
			},
		}

		err := identity.UpsertUserPreferences(ctx, req)
		require.NoError(t, err)

		res, err := identity.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{
			Username: username,
		})

		require.NoError(t, err)
		require.Equal(t, userpreferencesv1.Theme_THEME_DARK, res.Preferences.Theme)

		// expect the assist settings to have stayed the same
		require.Len(t, res.Preferences.Assist.PreferredLogins, 0)
		require.Equal(t, userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED, res.Preferences.Assist.ViewMode)
	})

	t.Run("update the assist preferred logins only", func(t *testing.T) {
		req := &userpreferencesv1.UpsertUserPreferencesRequest{
			Username: username,
			Preferences: &userpreferencesv1.UserPreferences{
				Assist: &userpreferencesv1.AssistUserPreferences{
					PreferredLogins: []string{"foo", "bar"},
				},
			},
		}

		err := identity.UpsertUserPreferences(ctx, req)
		require.NoError(t, err)

		res, err := identity.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{
			Username: username,
		})

		require.NoError(t, err)

		require.Equal(t, []string{"foo", "bar"}, res.Preferences.Assist.PreferredLogins)

		// expect the view mode to have stayed the same
		require.Equal(t, userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED, res.Preferences.Assist.ViewMode)

		// expect the theme to have stayed the same
		require.Equal(t, userpreferencesv1.Theme_THEME_DARK, res.Preferences.Theme)
	})

	t.Run("update the assist view mode only", func(t *testing.T) {
		req := &userpreferencesv1.UpsertUserPreferencesRequest{
			Username: username,
			Preferences: &userpreferencesv1.UserPreferences{
				Assist: &userpreferencesv1.AssistUserPreferences{
					ViewMode: userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE,
				},
			},
		}

		err := identity.UpsertUserPreferences(ctx, req)
		require.NoError(t, err)

		res, err := identity.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{
			Username: username,
		})
		require.NoError(t, err)

		require.Equal(t, userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE, res.Preferences.Assist.ViewMode)

		// expect the assist view mode to have stayed the same
		require.Equal(t, []string{"foo", "bar"}, res.Preferences.Assist.PreferredLogins)

		// expect the theme to have stayed the same
		require.Equal(t, userpreferencesv1.Theme_THEME_DARK, res.Preferences.Theme)
	})

	t.Run("update all the settings at once", func(t *testing.T) {
		req := &userpreferencesv1.UpsertUserPreferencesRequest{
			Username: username,
			Preferences: &userpreferencesv1.UserPreferences{
				Theme: userpreferencesv1.Theme_THEME_LIGHT,
				Assist: &userpreferencesv1.AssistUserPreferences{
					PreferredLogins: []string{"baz"},
					ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP,
				},
			},
		}

		err := identity.UpsertUserPreferences(ctx, req)
		require.NoError(t, err)

		res, err := identity.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{
			Username: username,
		})

		require.NoError(t, err)

		require.Equal(t, userpreferencesv1.Theme_THEME_LIGHT, res.Preferences.Theme)
		require.Equal(t, []string{"baz"}, res.Preferences.Assist.PreferredLogins)
		require.Equal(t, userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP, res.Preferences.Assist.ViewMode)
	})
}
