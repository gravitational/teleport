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
	defaultPref.SetClusterPreferences(userpreferencesv1.ClusterUserPreferences_builder{
		PinnedResources: userpreferencesv1.PinnedResourcesUserPreferences_builder{
			ResourceIds: []string{"123", "234"},
		}.Build(),
	}.Build())

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
			req: userpreferencesv1.UpsertUserPreferencesRequest_builder{
				Preferences: userpreferencesv1.UserPreferences_builder{
					Theme: userpreferencesv1.Theme_THEME_DARK,
				}.Build(),
			}.Build(),
			expected: userpreferencesv1.UserPreferences_builder{
				Onboard:                    defaultPref.GetOnboard(),
				Theme:                      userpreferencesv1.Theme_THEME_DARK,
				UnifiedResourcePreferences: defaultPref.GetUnifiedResourcePreferences(),
				ClusterPreferences:         defaultPref.GetClusterPreferences(),
				SideNavDrawerMode:          defaultPref.GetSideNavDrawerMode(),
			}.Build(),
		},
		{
			name: "update the availability view only",
			req: userpreferencesv1.UpsertUserPreferencesRequest_builder{
				Preferences: userpreferencesv1.UserPreferences_builder{
					UnifiedResourcePreferences: userpreferencesv1.UnifiedResourcePreferences_builder{
						AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_ACCESSIBLE,
					}.Build(),
				}.Build(),
			}.Build(),
			expected: userpreferencesv1.UserPreferences_builder{
				Onboard: defaultPref.GetOnboard(),
				Theme:   defaultPref.GetTheme(),
				UnifiedResourcePreferences: userpreferencesv1.UnifiedResourcePreferences_builder{
					DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_ALL,
					ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_CARD,
					LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
					AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_ACCESSIBLE,
				}.Build(),
				ClusterPreferences: defaultPref.GetClusterPreferences(),
				SideNavDrawerMode:  defaultPref.GetSideNavDrawerMode(),
			}.Build(),
		},
		{
			name: "update the unified tab preference only",
			req: userpreferencesv1.UpsertUserPreferencesRequest_builder{
				Preferences: userpreferencesv1.UserPreferences_builder{
					UnifiedResourcePreferences: userpreferencesv1.UnifiedResourcePreferences_builder{
						DefaultTab: userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
					}.Build(),
				}.Build(),
			}.Build(),
			expected: userpreferencesv1.UserPreferences_builder{
				Onboard: defaultPref.GetOnboard(),
				Theme:   defaultPref.GetTheme(),
				UnifiedResourcePreferences: userpreferencesv1.UnifiedResourcePreferences_builder{
					DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
					ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_CARD,
					LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
					AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_NONE,
				}.Build(),
				ClusterPreferences: defaultPref.GetClusterPreferences(),
				SideNavDrawerMode:  defaultPref.GetSideNavDrawerMode(),
			}.Build(),
		},
		{
			name: "update the onboard preference only",
			req: userpreferencesv1.UpsertUserPreferencesRequest_builder{
				Preferences: userpreferencesv1.UserPreferences_builder{
					Onboard: userpreferencesv1.OnboardUserPreferences_builder{
						PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_DATABASES},
						MarketingParams: userpreferencesv1.MarketingParams_builder{
							Campaign: "c_1",
							Source:   "s_1",
							Medium:   "m_1",
							Intent:   "i_1",
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			expected: userpreferencesv1.UserPreferences_builder{
				Theme:                      defaultPref.GetTheme(),
				UnifiedResourcePreferences: defaultPref.GetUnifiedResourcePreferences(),
				SideNavDrawerMode:          defaultPref.GetSideNavDrawerMode(),
				Onboard: userpreferencesv1.OnboardUserPreferences_builder{
					PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_DATABASES},
					MarketingParams: userpreferencesv1.MarketingParams_builder{
						Campaign: "c_1",
						Source:   "s_1",
						Medium:   "m_1",
						Intent:   "i_1",
					}.Build(),
				}.Build(),
				ClusterPreferences: defaultPref.GetClusterPreferences(),
			}.Build(),
		},
		{
			name: "update cluster preference only",
			req: userpreferencesv1.UpsertUserPreferencesRequest_builder{
				Preferences: userpreferencesv1.UserPreferences_builder{
					ClusterPreferences: userpreferencesv1.ClusterUserPreferences_builder{
						PinnedResources: userpreferencesv1.PinnedResourcesUserPreferences_builder{
							ResourceIds: []string{"node1", "node2"},
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			expected: userpreferencesv1.UserPreferences_builder{
				Theme:                      defaultPref.GetTheme(),
				UnifiedResourcePreferences: defaultPref.GetUnifiedResourcePreferences(),
				Onboard:                    defaultPref.GetOnboard(),
				SideNavDrawerMode:          defaultPref.GetSideNavDrawerMode(),
				ClusterPreferences: userpreferencesv1.ClusterUserPreferences_builder{
					PinnedResources: userpreferencesv1.PinnedResourcesUserPreferences_builder{
						ResourceIds: []string{"node1", "node2"},
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			name: "update sidenav preference only",
			req: userpreferencesv1.UpsertUserPreferencesRequest_builder{
				Preferences: userpreferencesv1.UserPreferences_builder{
					SideNavDrawerMode: userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_STICKY,
				}.Build(),
			}.Build(),
			expected: userpreferencesv1.UserPreferences_builder{
				Theme:                      defaultPref.GetTheme(),
				UnifiedResourcePreferences: defaultPref.GetUnifiedResourcePreferences(),
				Onboard:                    defaultPref.GetOnboard(),
				ClusterPreferences:         defaultPref.GetClusterPreferences(),
				SideNavDrawerMode:          userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_STICKY,
			}.Build(),
		},
		{
			name: "update the discover resource guide preference only",
			req: userpreferencesv1.UpsertUserPreferencesRequest_builder{
				Preferences: userpreferencesv1.UserPreferences_builder{
					DiscoverResourcePreferences: userpreferencesv1.DiscoverResourcePreferences_builder{
						DiscoverGuide: userpreferencesv1.DiscoverGuide_builder{
							Pinned: []string{"guide-1", "guide-2"},
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			expected: userpreferencesv1.UserPreferences_builder{
				Onboard:                    defaultPref.GetOnboard(),
				Theme:                      defaultPref.GetTheme(),
				UnifiedResourcePreferences: defaultPref.GetUnifiedResourcePreferences(),
				ClusterPreferences:         defaultPref.GetClusterPreferences(),
				SideNavDrawerMode:          defaultPref.GetSideNavDrawerMode(),
				DiscoverResourcePreferences: userpreferencesv1.DiscoverResourcePreferences_builder{
					DiscoverGuide: userpreferencesv1.DiscoverGuide_builder{
						Pinned: []string{"guide-1", "guide-2"},
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			name: "update all the settings at once",
			req: userpreferencesv1.UpsertUserPreferencesRequest_builder{
				Preferences: userpreferencesv1.UserPreferences_builder{
					Theme: userpreferencesv1.Theme_THEME_LIGHT,
					UnifiedResourcePreferences: userpreferencesv1.UnifiedResourcePreferences_builder{
						DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
						ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_LIST,
						LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
						AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_NONE,
					}.Build(),
					SideNavDrawerMode: userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_STICKY,
					Onboard: userpreferencesv1.OnboardUserPreferences_builder{
						PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_KUBERNETES},
						MarketingParams: userpreferencesv1.MarketingParams_builder{
							Campaign: "c_2",
							Source:   "s_2",
							Medium:   "m_2",
							Intent:   "i_2",
						}.Build(),
					}.Build(),
					ClusterPreferences: userpreferencesv1.ClusterUserPreferences_builder{
						PinnedResources: userpreferencesv1.PinnedResourcesUserPreferences_builder{
							ResourceIds: []string{"node1", "node2"},
						}.Build(),
					}.Build(),
					DiscoverResourcePreferences: userpreferencesv1.DiscoverResourcePreferences_builder{
						DiscoverGuide: userpreferencesv1.DiscoverGuide_builder{
							Pinned: []string{"guide-3", "guide-4"},
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			expected: userpreferencesv1.UserPreferences_builder{
				Theme: userpreferencesv1.Theme_THEME_LIGHT,
				UnifiedResourcePreferences: userpreferencesv1.UnifiedResourcePreferences_builder{
					DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
					ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_LIST,
					LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
					AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_NONE,
				}.Build(),
				Onboard: userpreferencesv1.OnboardUserPreferences_builder{
					PreferredResources: []userpreferencesv1.Resource{userpreferencesv1.Resource_RESOURCE_KUBERNETES},
					MarketingParams: userpreferencesv1.MarketingParams_builder{
						Campaign: "c_2",
						Source:   "s_2",
						Medium:   "m_2",
						Intent:   "i_2",
					}.Build(),
				}.Build(),
				ClusterPreferences: userpreferencesv1.ClusterUserPreferences_builder{
					PinnedResources: userpreferencesv1.PinnedResourcesUserPreferences_builder{
						ResourceIds: []string{"node1", "node2"},
					}.Build(),
				}.Build(),
				DiscoverResourcePreferences: userpreferencesv1.DiscoverResourcePreferences_builder{
					DiscoverGuide: userpreferencesv1.DiscoverGuide_builder{
						Pinned: []string{"guide-3", "guide-4"},
					}.Build(),
				}.Build(),
				SideNavDrawerMode: userpreferencesv1.SideNavDrawerMode_SIDE_NAV_DRAWER_MODE_STICKY,
			}.Build(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			identity := newUserPreferencesService(t)

			res, err := identity.GetUserPreferences(ctx, username)
			require.NoError(t, err)
			// Clone the proto as the accessing fields for some reason modifies the state.
			require.Empty(t, cmp.Diff(defaultPref, proto.Clone(res), protocmp.Transform()))

			if test.req != nil {
				err := identity.UpsertUserPreferences(ctx, username, test.req.GetPreferences())
				require.NoError(t, err)
			}

			res, err = identity.GetUserPreferences(ctx, username)

			require.NoError(t, err)
			require.Empty(t, cmp.Diff(test.expected, res, protocmp.Transform()))
		})
	}
}
