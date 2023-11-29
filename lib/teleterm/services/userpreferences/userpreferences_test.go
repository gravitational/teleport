// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package userpreferences

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

var preferencesMock = &userpreferencesv1.UserPreferences{
	Assist:  nil,
	Onboard: nil,
	Theme:   userpreferencesv1.Theme_THEME_LIGHT,
	ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
		PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
			ResourceIds: []string{"abc", "def"},
		},
	},
	UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
		DefaultTab: userpreferencesv1.DefaultTab_DEFAULT_TAB_ALL,
		ViewMode:   userpreferencesv1.ViewMode_VIEW_MODE_CARD,
	},
}

func TestUserPreferencesGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockedClient := &mockClient{preferences: preferencesMock}

	response, err := Get(ctx, mockedClient)
	require.NoError(t, err)
	require.Equal(t, preferencesMock.GetUnifiedResourcePreferences(), response.GetUnifiedResourcePreferences())
	require.Equal(t, preferencesMock.GetClusterPreferences(), response.GetClusterPreferences())
}

func TestUserPreferencesUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockedClient := &mockClient{preferences: preferencesMock}

	newPreferences := &api.UserPreferences{
		ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
			PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
				ResourceIds: []string{"foo", "bar"},
			},
		},
		UnifiedResourcePreferences: nil,
	}

	err := Update(ctx, mockedClient, newPreferences)
	require.NoError(t, err)
	// ClusterPreferences field has been updated with the new value.
	require.Equal(t, newPreferences.ClusterPreferences, mockedClient.upsertCalledWith.ClusterPreferences)
	// UnifiedResourcePreferences field has not changed because it was nil in the new value.
	require.Equal(t, preferencesMock.UnifiedResourcePreferences, mockedClient.upsertCalledWith.UnifiedResourcePreferences)
	// Other user preferences have not been touched.
	require.Equal(t, preferencesMock.Theme, mockedClient.upsertCalledWith.Theme)
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
