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

func Get(ctx context.Context, rootClient Client, leafClient Client) (*api.UserPreferences, error) {
	rootPreferencesResponse, err := rootClient.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rootPreferences := rootPreferencesResponse.GetPreferences()
	clusterPreferences := rootPreferences.GetClusterPreferences()

	if leafClient != nil {
		preferences, err := leafClient.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusterPreferences = preferences.GetPreferences().GetClusterPreferences()
	}

	return &api.UserPreferences{
		UnifiedResourcePreferences: rootPreferences.UnifiedResourcePreferences,
		ClusterPreferences:         clusterPreferences,
	}, nil
}

// Update updates the preferences for a given user.
// Only the properties that are set (cluster_preferences, unified_resource_preferences) will be updated.
func Update(ctx context.Context, rootClient Client, leafClient Client, newPreferences *api.UserPreferences) (*api.UserPreferences, error) {
	// We have to fetch the full user preferences struct and modify only
	// the fields that change.
	// Calling `UpsertUserPreferences` with only the modified values would reset
	// the rest of the preferences.
	rootPreferencesResponse, err := rootClient.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rootPreferences := rootPreferencesResponse.GetPreferences()

	var leafPreferences *userpreferencesv1.UserPreferences
	if leafClient != nil {
		response, err := leafClient.GetUserPreferences(ctx, &userpreferencesv1.GetUserPreferencesRequest{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		leafPreferences = response.GetPreferences()
	}

	hasUnifiedResourcePreferencesForRoot := newPreferences.UnifiedResourcePreferences != nil
	hasClusterPreferencesForRoot := newPreferences.ClusterPreferences != nil && leafPreferences == nil

	if hasUnifiedResourcePreferencesForRoot || hasClusterPreferencesForRoot {
		if hasUnifiedResourcePreferencesForRoot {
			rootPreferences.UnifiedResourcePreferences = newPreferences.UnifiedResourcePreferences
		}
		if hasClusterPreferencesForRoot {
			rootPreferences.ClusterPreferences = newPreferences.ClusterPreferences
		}

		err := rootClient.UpsertUserPreferences(ctx, &userpreferencesv1.UpsertUserPreferencesRequest{
			Preferences: rootPreferences,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	hasClusterPreferencesForLeaf := newPreferences.ClusterPreferences != nil && leafPreferences != nil
	if hasClusterPreferencesForLeaf {
		leafPreferences.ClusterPreferences = newPreferences.ClusterPreferences

		err := leafClient.UpsertUserPreferences(ctx, &userpreferencesv1.UpsertUserPreferencesRequest{
			Preferences: leafPreferences,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	updatedPreferences := &api.UserPreferences{
		ClusterPreferences:         rootPreferences.ClusterPreferences,
		UnifiedResourcePreferences: rootPreferences.UnifiedResourcePreferences,
	}
	if leafPreferences != nil {
		updatedPreferences.ClusterPreferences = leafPreferences.ClusterPreferences
	}

	return updatedPreferences, nil
}

// Client represents auth.ClientI methods used by [Get] and [Update].
// During a normal operation, auth.ClientI is passed as this interface.
type Client interface {
	// See auth.ClientI.GetUserPreferences
	GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error)
	// See auth.ClientI.UpsertUserPreferences
	UpsertUserPreferences(ctx context.Context, req *userpreferencesv1.UpsertUserPreferencesRequest) error
}
