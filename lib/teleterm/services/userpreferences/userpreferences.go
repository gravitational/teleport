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

	"github.com/gravitational/trace"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

func Get(ctx context.Context, client Client) (*api.UserPreferences, error) {
	preferences, err := client.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &api.UserPreferences{
		ClusterPreferences:         preferences.GetPreferences().ClusterPreferences,
		UnifiedResourcePreferences: preferences.GetPreferences().UnifiedResourcePreferences,
	}, nil
}

// Update updates the preferences for a given user.
// Only the properties that are set (cluster_preferences, unified_resource_preferences) will be updated.
func Update(ctx context.Context, client Client, newPreferences *api.UserPreferences) error {
	// We have to fetch the full user preferences struct and modify only
	// the fields that change.
	// Calling `UpsertUserPreferences` with only the modified values would reset
	// the rest of the preferences.
	currentPreferencesResponse, err := client.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{})
	if err != nil {
		return trace.Wrap(err)
	}

	currentPreferences := currentPreferencesResponse.GetPreferences()

	if newPreferences.UnifiedResourcePreferences != nil {
		currentPreferences.UnifiedResourcePreferences = newPreferences.UnifiedResourcePreferences
	}
	if newPreferences.ClusterPreferences != nil {
		currentPreferences.ClusterPreferences = newPreferences.ClusterPreferences
	}

	err = client.UpsertUserPreferences(ctx, &userpreferencesv1.UpsertUserPreferencesRequest{
		Preferences: currentPreferences,
	})

	return trace.Wrap(err)
}

// Client represents auth.ClientI methods used by [Get] and [Update].
// During a normal operation, auth.ClientI is passed as this interface.
type Client interface {
	// See auth.ClientI.GetUserPreferences
	GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error)
	// See auth.ClientI.UpsertUserPreferences
	UpsertUserPreferences(ctx context.Context, req *userpreferencesv1.UpsertUserPreferencesRequest) error
}
