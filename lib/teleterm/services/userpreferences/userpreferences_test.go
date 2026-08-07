// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package userpreferences

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

var rootPreferencesMock = &userpreferencesv1.UserPreferences{
	Onboard: nil,
	Theme:   userpreferencesv1.Theme_THEME_LIGHT,
	ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
		PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
			ResourceIds: []string{"abc", "def"},
		},
	},
	UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
		DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_ALL,
		ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_CARD,
		LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
		AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_NONE,
	},
}

var leafPreferencesMock = &userpreferencesv1.UserPreferences{
	Onboard: nil,
	ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
		PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
			ResourceIds: []string{"ghi", "jkl"},
		},
	},
}

func TestUserPreferencesGet(t *testing.T) {
	mockedRootClient := &mockClient{preferences: rootPreferencesMock}
	mockedLeafClient := &mockClient{preferences: leafPreferencesMock}

	response, err := Get(t.Context(), mockedRootClient, mockedLeafClient)
	require.NoError(t, err)
	require.Equal(t, rootPreferencesMock.GetUnifiedResourcePreferences(), response.GetUnifiedResourcePreferences())
	require.Equal(t, leafPreferencesMock.GetClusterPreferences(), response.GetClusterPreferences())
}

func TestUserPreferencesUpdateForRoot(t *testing.T) {
	mockedClient := &mockClient{preferences: rootPreferencesMock}

	newPreferences := &api.UserPreferences{
		ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
			PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
				ResourceIds: []string{"foo", "bar"},
			},
		},
		UnifiedResourcePreferences: nil,
	}

	updatedPreferences, err := Update(t.Context(), mockedClient, nil, newPreferences)
	require.NoError(t, err)
	// ClusterPreferences field has been updated with the new value.
	require.Equal(t, newPreferences.ClusterPreferences, mockedClient.upsertCalledWith.ClusterPreferences)
	require.Equal(t, newPreferences.ClusterPreferences, updatedPreferences.ClusterPreferences)
	// UnifiedResourcePreferences field has not changed because it was nil in the new value.
	require.Equal(t, rootPreferencesMock.UnifiedResourcePreferences, mockedClient.upsertCalledWith.UnifiedResourcePreferences)
	require.Equal(t, rootPreferencesMock.UnifiedResourcePreferences, updatedPreferences.UnifiedResourcePreferences)
	// Other user preferences have not been touched.
	require.Equal(t, rootPreferencesMock.Theme, mockedClient.upsertCalledWith.Theme)
}

func TestUserPreferencesUpdateForRootWithNoExistingPinnedResources(t *testing.T) {
	rootPreferences := &userpreferencesv1.UserPreferences{
		Theme: userpreferencesv1.Theme_THEME_LIGHT,
		ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
			PinnedResources: nil,
		},
		UnifiedResourcePreferences: rootPreferencesMock.UnifiedResourcePreferences,
	}
	mockedClient := &mockClient{preferences: rootPreferences}

	newPreferences := &api.UserPreferences{
		ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
			PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
				ResourceIds: []string{"foo", "bar"},
			},
		},
	}

	updatedPreferences, err := Update(t.Context(), mockedClient, nil, newPreferences)
	require.NoError(t, err)

	require.Equal(t, newPreferences.ClusterPreferences, mockedClient.upsertCalledWith.ClusterPreferences)
	require.Equal(t, newPreferences.ClusterPreferences, updatedPreferences.ClusterPreferences)
	require.Equal(t, rootPreferencesMock.UnifiedResourcePreferences, mockedClient.upsertCalledWith.UnifiedResourcePreferences)
	require.Equal(t, userpreferencesv1.Theme_THEME_LIGHT, mockedClient.upsertCalledWith.Theme)
}

func TestUserPreferencesUpdateForRootAndLeaf(t *testing.T) {
	mockedRootClient := &mockClient{preferences: rootPreferencesMock}
	mockedLeafClient := &mockClient{preferences: leafPreferencesMock}

	newPreferences := &api.UserPreferences{
		ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
			PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
				ResourceIds: []string{"foo", "bar"},
			},
		},
		UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
			DefaultTab:            userpreferencesv1.DefaultTab_DEFAULT_TAB_PINNED,
			ViewMode:              userpreferencesv1.ViewMode_VIEW_MODE_LIST,
			LabelsViewMode:        userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_EXPANDED,
			AvailableResourceMode: userpreferencesv1.AvailableResourceMode_AVAILABLE_RESOURCE_MODE_REQUESTABLE,
		},
	}

	updatedPreferences, err := Update(t.Context(), mockedRootClient, mockedLeafClient, newPreferences)
	require.NoError(t, err)
	// ClusterPreferences field has been updated with the leaf cluster value.
	require.Equal(t, updatedPreferences.ClusterPreferences, mockedLeafClient.upsertCalledWith.ClusterPreferences)
	require.Equal(t, newPreferences.ClusterPreferences, updatedPreferences.ClusterPreferences)
	// UnifiedResourcePreferences field has been updated with the root cluster value.
	require.Equal(t, updatedPreferences.UnifiedResourcePreferences, mockedRootClient.upsertCalledWith.UnifiedResourcePreferences)
	require.Equal(t, newPreferences.UnifiedResourcePreferences, updatedPreferences.UnifiedResourcePreferences)
	// Other user preferences have not been touched.
	require.Equal(t, rootPreferencesMock.Theme, mockedRootClient.upsertCalledWith.Theme)
}

func TestNilUserPreferencesUpdate(t *testing.T) {
	tests := []struct {
		name                       string
		leafPreferences            *userpreferencesv1.UserPreferences
		expectedClusterPreferences *userpreferencesv1.ClusterUserPreferences
	}{
		{
			name:                       "root",
			expectedClusterPreferences: rootPreferencesMock.ClusterPreferences,
		},
		{
			name:                       "leaf",
			leafPreferences:            leafPreferencesMock,
			expectedClusterPreferences: leafPreferencesMock.ClusterPreferences,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			mockedRootClient := &mockClient{preferences: rootPreferencesMock}
			var leafClient Client
			var mockedLeafClient *mockClient
			if tt.leafPreferences != nil {
				mockedLeafClient = &mockClient{preferences: tt.leafPreferences}
				leafClient = mockedLeafClient
			}

			updatedPreferences, err := Update(ctx, mockedRootClient, leafClient, nil)
			require.NoError(t, err)

			require.Nil(t, mockedRootClient.upsertCalledWith)
			if mockedLeafClient != nil {
				require.Nil(t, mockedLeafClient.upsertCalledWith)
			}
			require.Equal(t, rootPreferencesMock.UnifiedResourcePreferences, updatedPreferences.UnifiedResourcePreferences)
			require.Equal(t, tt.expectedClusterPreferences, updatedPreferences.ClusterPreferences)
		})
	}
}

type mockClient struct {
	preferences      *userpreferencesv1.UserPreferences
	upsertCalledWith *userpreferencesv1.UserPreferences
}

func (m *mockClient) GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error) {
	return &userpreferencesv1.GetUserPreferencesResponse{
		Preferences: m.preferences,
	}, nil
}

func (m *mockClient) UpsertUserPreferences(ctx context.Context, req *userpreferencesv1.UpsertUserPreferencesRequest) error {
	m.upsertCalledWith = req.Preferences
	return nil
}
