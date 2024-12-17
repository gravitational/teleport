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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

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
				Onboard:                    defaultPref.Onboard,
				Theme:                      userpreferencesv1.Theme_THEME_DARK,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				ClusterPreferences:         defaultPref.ClusterPreferences,
				SideNavDrawerMode:          defaultPref.SideNavDrawerMode,
			},
		},
		{
			name: "update the availability view only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
						AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_ACCESSIBLE,
					},
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Onboard: defaultPref.Onboard,
				Theme:   defaultPref.Theme,
				UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
					DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_ALL,
					ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_CARD,
					LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
					AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_ACCESSIBLE,
				},
				ClusterPreferences: defaultPref.ClusterPreferences,
				SideNavDrawerMode:  defaultPref.SideNavDrawerMode,
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
				Onboard: defaultPref.Onboard,
				Theme:   defaultPref.Theme,
				UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
					DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
					ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_CARD,
					LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
					AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_NONE,
				},
				ClusterPreferences: defaultPref.ClusterPreferences,
				SideNavDrawerMode:  defaultPref.SideNavDrawerMode,
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
				Theme:                      defaultPref.Theme,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				SideNavDrawerMode:          defaultPref.SideNavDrawerMode,
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
				Theme:                      defaultPref.Theme,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				Onboard:                    defaultPref.Onboard,
				SideNavDrawerMode:          defaultPref.SideNavDrawerMode,
				ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
					PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
						ResourceIds: []string{"node1", "node2"},
					},
				},
			},
		},
		{
			name: "update sidenav preference only",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					SideNavDrawerMode: userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_STICKY,
				},
			},
			expected: &userpreferencesv1.UserPreferences{
				Theme:                      defaultPref.Theme,
				UnifiedResourcePreferences: defaultPref.UnifiedResourcePreferences,
				Onboard:                    defaultPref.Onboard,
				ClusterPreferences:         defaultPref.ClusterPreferences,
				SideNavDrawerMode:          userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_STICKY,
			},
		},
		{
			name: "update all the settings at once",
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: &userpreferencesv1.UserPreferences{
					Theme: userpreferencesv1.Theme_THEME_LIGHT,
					UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
						DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
						ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_LIST,
						LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
						AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_NONE,
					},
					SideNavDrawerMode: userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_STICKY,
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
					DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
					ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_LIST,
					LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
					AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_NONE,
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
				SideNavDrawerMode: userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_STICKY,
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
