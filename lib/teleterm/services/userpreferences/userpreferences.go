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
			rootPreferences.UnifiedResourcePreferences = updateUnifiedResourcePreferences(rootPreferences.UnifiedResourcePreferences, newPreferences.UnifiedResourcePreferences)
		}
		if hasClusterPreferencesForRoot {
			rootPreferences.ClusterPreferences = updateClusterPreferences(rootPreferences.ClusterPreferences, newPreferences.ClusterPreferences)
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
		leafPreferences.ClusterPreferences = updateClusterPreferences(leafPreferences.ClusterPreferences, newPreferences.ClusterPreferences)

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

// updateUnifiedResourcePreferences updates DefaultTab, ViewMode,
// and LabelsViewMode fields in UnifiedResourcePreferences.
// The fields are updated one by one (instead of passing the entire struct as new preferences)
// to prevent potential new fields from being overwritten.
// Supports oldPreferences being nil.
func updateUnifiedResourcePreferences(oldPreferences *userpreferencesv1.UnifiedResourcePreferences, newPreferences *userpreferencesv1.UnifiedResourcePreferences) *userpreferencesv1.UnifiedResourcePreferences {
	updated := oldPreferences
	// TODO(gzdunek): DELETE IN 16.0.0.
	// We won't have to support old preferences being nil.
	if oldPreferences == nil {
		updated = &userpreferencesv1.UnifiedResourcePreferences{}
	}

	updated.DefaultTab = newPreferences.DefaultTab
	updated.ViewMode = newPreferences.ViewMode
	updated.LabelsViewMode = newPreferences.LabelsViewMode

	return updated
}

// updateClusterPreferences updates pinned resources in ClusterUserPreferences.
// The fields are updated one by one (instead of passing the entire struct as new preferences)
// to prevent potential new fields from being overwritten.
// Supports oldPreferences being nil.
func updateClusterPreferences(oldPreferences *userpreferencesv1.ClusterUserPreferences, newPreferences *userpreferencesv1.ClusterUserPreferences) *userpreferencesv1.ClusterUserPreferences {
	updated := oldPreferences
	// TODO(gzdunek): DELETE IN 16.0.0.
	// We won't have to support old preferences being nil.
	if oldPreferences == nil {
		updated = &userpreferencesv1.ClusterUserPreferences{}
	}
	if updated.PinnedResources == nil {
		updated.PinnedResources = &userpreferencesv1.PinnedResourcesUserPreferences{}
	}

	updated.PinnedResources.ResourceIds = newPreferences.PinnedResources.ResourceIds

	return updated
}

// Client represents auth.ClientI methods used by [Get] and [Update].
// During a normal operation, auth.ClientI is passed as this interface.
type Client interface {
	// See auth.ClientI.GetUserPreferences
	GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error)
	// See auth.ClientI.UpsertUserPreferences
	UpsertUserPreferences(ctx context.Context, req *userpreferencesv1.UpsertUserPreferencesRequest) error
}
