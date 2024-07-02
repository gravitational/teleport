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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockedRootClient := &mockClient{preferences: rootPreferencesMock}
	mockedLeafClient := &mockClient{preferences: leafPreferencesMock}

	response, err := Get(ctx, mockedRootClient, mockedLeafClient)
	require.NoError(t, err)
	require.Equal(t, rootPreferencesMock.GetUnifiedResourcePreferences(), response.GetUnifiedResourcePreferences())
	require.Equal(t, leafPreferencesMock.GetClusterPreferences(), response.GetClusterPreferences())
}

func TestUserPreferencesUpdateForRoot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockedClient := &mockClient{preferences: rootPreferencesMock}

	newPreferences := &api.UserPreferences{
		ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
			PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
				ResourceIds: []string{"foo", "bar"},
			},
		},
		UnifiedResourcePreferences: nil,
	}

	updatedPreferences, err := Update(ctx, mockedClient, nil, newPreferences)
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

func TestUserPreferencesUpdateForRootAndLeaf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

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

	updatedPreferences, err := Update(ctx, mockedRootClient, mockedLeafClient, newPreferences)
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
