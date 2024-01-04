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

package local_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

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

func TestUserPreferences_ClusterPreferences(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	defaultPref := local.DefaultUserPreferences()
	defaultPref.ClusterPreferences = &userpreferencesv1.ClusterUserPreferences{
		PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
			ResourceIds: []string{"123", "234"},
		},
	}

	username := "something"
	identity := newUserPreferencesService(t)

	err := identity.UpsertUserPreferences(ctx, username, defaultPref)
	require.NoError(t, err)

	res, err := identity.GetUserPreferences(ctx, username)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(defaultPref, res, protocmp.Transform()))

	// send empty preferences, cluster prefs should be overwritten
	reqPrefs := local.DefaultUserPreferences()
	err = identity.UpsertUserPreferences(ctx, username, reqPrefs)
	require.NoError(t, err)
	res, err = identity.GetUserPreferences(ctx, username)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(reqPrefs, res, protocmp.Transform()))
}

func TestUserPreferencesCRUD(t *testing.T) {
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
				Preferences: &userpreferencesv1.UserPreferences{
					Theme: userpreferencesv1.Theme_THEME_DARK,
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Assist:                     defaultPref.Assist,
				Onboard:                    defaultPref.Onboard,
				Theme:                      userpreferencesv1.Theme_THEME_DARK,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				ClusterPreferences:         defaultPref.ClusterPreferences,
			},
		},
		{
			name: "update the unified tab preference only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
						DefaultTab: userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Assist:  defaultPref.Assist,
				Onboard: defaultPref.Onboard,
				Theme:   defaultPref.Theme,
				UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
					DefaultTab:     userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
					ViewMode:       userpreferencesv1.ViewMode_VIEW_MODE_CARD,
					LabelsViewMode: userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
				},
				ClusterPreferences: defaultPref.ClusterPreferences,
			},
		},
		{
			name: "update the assist preferred logins only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					Assist: &userpreferencesv1.AssistUserPreferences{
						PreferredLogins: []string{"foo", "bar"},
					},
					Onboard: &userpreferencesv1.OnboardUserPreferences{
						PreferredResources: []userpreferencesv1.Resource{},
						MarketingParams:    &userpreferencesv1.MarketingParams{},
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Theme:                      defaultPref.Theme,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				Onboard:                    defaultPref.Onboard,
				Assist: &userpreferencesv1.AssistUserPreferences{
					PreferredLogins: []string{"foo", "bar"},
					ViewMode:        defaultPref.Assist.ViewMode,
				},
				ClusterPreferences: defaultPref.ClusterPreferences,
			},
		},
		{
			name: "update the assist view mode only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					Assist: &userpreferencesv1.AssistUserPreferences{
						ViewMode: userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE,
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Theme:                      defaultPref.Theme,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				Onboard:                    defaultPref.Onboard,
				Assist: &userpreferencesv1.AssistUserPreferences{
					PreferredLogins: defaultPref.Assist.PreferredLogins,
					ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE,
				},
				ClusterPreferences: defaultPref.ClusterPreferences,
			},
		},
		{
			name: "update the onboard preference only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					Onboard: &userpreferencesv1.OnboardUserPreferences{
						PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_DATABASES},
						MarketingParams: &userpreferencesv1.MarketingParams{
							Campaign: "c_1",
							Source:   "s_1",
							Medium:   "m_1",
							Intent:   "i_1",
						},
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Assist:                     defaultPref.Assist,
				Theme:                      defaultPref.Theme,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				Onboard: &userpreferencesv1.OnboardUserPreferences{
					PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_DATABASES},
					MarketingParams: &userpreferencesv1.MarketingParams{
						Campaign: "c_1",
						Source:   "s_1",
						Medium:   "m_1",
						Intent:   "i_1",
					},
				},
				ClusterPreferences: defaultPref.ClusterPreferences,
			},
		},
		{
			name: "update cluster preference only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
						PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
							ResourceIds: []string{"node1", "node2"},
						},
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Assist:                     defaultPref.Assist,
				Theme:                      defaultPref.Theme,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				Onboard:                    defaultPref.Onboard,
				ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
					PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
						ResourceIds: []string{"node1", "node2"},
					},
				},
			},
		},
		{
			name: "update all the settings at once",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					Theme: userpreferencesv1.Theme_THEME_LIGHT,
					UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
						DefaultTab:     userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
						ViewMode:       userpreferencesv1.ViewMode_VIEW_MODE_LIST,
						LabelsViewMode: userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
					},
					Assist: &userpreferencesv1.AssistUserPreferences{
						PreferredLogins: []string{"baz"},
						ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP,
					},
					Onboard: &userpreferencesv1.OnboardUserPreferences{
						PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_KUBERNETES},
						MarketingParams: &userpreferencesv1.MarketingParams{
							Campaign: "c_2",
							Source:   "s_2",
							Medium:   "m_2",
							Intent:   "i_2",
						},
					},
					ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
						PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
							ResourceIds: []string{"node1", "node2"},
						},
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Theme: userpreferencesv1.Theme_THEME_LIGHT,
				UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
					DefaultTab:     userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
					ViewMode:       userpreferencesv1.ViewMode_VIEW_MODE_LIST,
					LabelsViewMode: userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
				},
				Assist: &userpreferencesv1.AssistUserPreferences{
					PreferredLogins: []string{"baz"},
					ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_POPUP,
				},
				Onboard: &userpreferencesv1.OnboardUserPreferences{
					PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_KUBERNETES},
					MarketingParams: &userpreferencesv1.MarketingParams{
						Campaign: "c_2",
						Source:   "s_2",
						Medium:   "m_2",
						Intent:   "i_2",
					},
				},
				ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
					PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
						ResourceIds: []string{"node1", "node2"},
					},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			identity := newUserPreferencesService(t)

			res, err := identity.GetUserPreferences(ctx, username)
			require.NoError(t, err)
			// Clone the proto as the accessing fields for some reason modifies the state.
			require.Empty(t, cmp.Diff(defaultPref, proto.Clone(res), protocmp.Transform()))

			if test.req != nil {
				err := identity.UpsertUserPreferences(ctx, username, test.req.Preferences)
				require.NoError(t, err)
			}

			res, err = identity.GetUserPreferences(ctx, username)

			require.NoError(t, err)
			require.Empty(t, cmp.Diff(test.expected, res, protocmp.Transform()))
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
	prefs, err := identity.GetUserPreferences(ctx, "test")
	require.NoError(t, err)
	// The layout should be updated to the latest version (values should not be nil).
	require.NotNil(t, prefs.Onboard)
	// Non-existing values should be set to the default value.
	require.Equal(t, userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED, prefs.Assist.ViewMode)
	require.Equal(t, userpreferencesv1.Theme_THEME_LIGHT, prefs.Theme)
	// Existing values should be preserved.
	require.Equal(t, []string{"foo", "bar"}, prefs.Assist.PreferredLogins)
}
