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
	"golang.org/x/sync/errgroup"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

func Get(ctx context.Context, rootClient Client, leafClient Client) (*api.UserPreferences, error) {
	group, groupCtx := errgroup.WithContext(ctx)
	var rootPreferencesResponse *userpreferencesv1.GetUserPreferencesResponse
	var leafPreferencesResponse *userpreferencesv1.GetUserPreferencesResponse

	group.Go(func() error {
		res, err := rootClient.GetUserPreferences(groupCtx, &userpreferencesv1.GetUserPreferencesRequest{})
		rootPreferencesResponse = res
		return trace.Wrap(err)
	})

	if leafClient != nil {
		group.Go(func() error {
			res, err := leafClient.GetUserPreferences(groupCtx, &userpreferencesv1.GetUserPreferencesRequest{})
			leafPreferencesResponse = res
			return trace.Wrap(err)
		})
	}

	if err := group.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	rootPreferences := rootPreferencesResponse.GetPreferences()
	clusterPreferences := rootPreferences.GetClusterPreferences()
	if leafPreferencesResponse != nil {
		clusterPreferences = leafPreferencesResponse.GetPreferences().GetClusterPreferences()
	}

	return &api.UserPreferences{
		UnifiedResourcePreferences: rootPreferences.UnifiedResourcePreferences,
		ClusterPreferences:         clusterPreferences,
	}, nil
}

// Update updates the preferences for a given user.
// Only the properties that are set (cluster_preferences, unified_resource_preferences) are updated.
// When updating the preferences for the root cluster, both unified_resource_preferences
// and cluster_preferences are updated in it.
// When updating the preferences for the leaf cluster, only cluster_preferences are updated
// in the leaf, unified_resource_preferences are always updated in the root.
func Update(ctx context.Context, rootClient Client, leafClient Client, newPreferences *api.UserPreferences) (*api.UserPreferences, error) {
	// We have to fetch the full user preferences struct and modify only
	// the fields that change.
	// Calling `UpsertUserPreferences` with only the modified values would reset
	// the rest of the preferences.
	getGroup, getGroupCtx := errgroup.WithContext(ctx)
	var rootPreferencesResponse *userpreferencesv1.GetUserPreferencesResponse
	var leafPreferencesResponse *userpreferencesv1.GetUserPreferencesResponse

	getGroup.Go(func() error {
		res, err := rootClient.GetUserPreferences(getGroupCtx, &userpreferencesv1.GetUserPreferencesRequest{})
		rootPreferencesResponse = res
		return trace.Wrap(err)
	})

	if leafClient != nil {
		getGroup.Go(func() error {
			res, err := leafClient.GetUserPreferences(getGroupCtx, &userpreferencesv1.GetUserPreferencesRequest{})
			leafPreferencesResponse = res
			return trace.Wrap(err)
		})
	}

	if err := getGroup.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	rootPreferences := rootPreferencesResponse.GetPreferences()
	var leafPreferences *userpreferencesv1.UserPreferences
	if leafPreferencesResponse != nil {
		leafPreferences = leafPreferencesResponse.GetPreferences()
	}

	// We do not use errgroup.WithContext since we don't want to cancel
	// the other request when one of them fails.
	//
	// We can run update requests concurrently because the preferences for the root
	// cluster and the leaf cluster aren't dependent on each other.
	// The preferences for the unified view are always set for the root cluster,
	// while pinned resources can be set for either the root or the leaf.
	// So if, for example, setting unified view preferences fails,
	// we can still update pinned resources for leaf.
	upsertGroup := errgroup.Group{}

	hasUnifiedResourcePreferencesForRoot := newPreferences.UnifiedResourcePreferences != nil
	hasClusterPreferencesForRoot := newPreferences.ClusterPreferences != nil && leafPreferences == nil

	if hasUnifiedResourcePreferencesForRoot || hasClusterPreferencesForRoot {
		if hasUnifiedResourcePreferencesForRoot {
			rootPreferences.UnifiedResourcePreferences = updateUnifiedResourcePreferences(rootPreferences.UnifiedResourcePreferences, newPreferences.UnifiedResourcePreferences)
		}
		if hasClusterPreferencesForRoot {
			rootPreferences.ClusterPreferences = updateClusterPreferences(rootPreferences.ClusterPreferences, newPreferences.ClusterPreferences)
		}

		upsertGroup.Go(func() error {
			err := rootClient.UpsertUserPreferences(ctx, &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: rootPreferences,
			})
			return trace.Wrap(err)
		})
	}

	hasClusterPreferencesForLeaf := newPreferences.ClusterPreferences != nil && leafPreferences != nil
	if hasClusterPreferencesForLeaf {
		leafPreferences.ClusterPreferences = updateClusterPreferences(leafPreferences.ClusterPreferences, newPreferences.ClusterPreferences)

		upsertGroup.Go(func() error {
			err := leafClient.UpsertUserPreferences(ctx, &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: leafPreferences,
			})
			return trace.Wrap(err)
		})
	}

	if err := upsertGroup.Wait(); err != nil {
		return nil, trace.Wrap(err)
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
func updateUnifiedResourcePreferences(oldPreferences *userpreferencesv1.UnifiedResourcePreferences, newPreferences *userpreferencesv1.UnifiedResourcePreferences) *userpreferencesv1.UnifiedResourcePreferences {
	updated := oldPreferences

	updated.DefaultTab = newPreferences.DefaultTab
	updated.ViewMode = newPreferences.ViewMode
	updated.LabelsViewMode = newPreferences.LabelsViewMode
	updated.AvailableResourceMode = newPreferences.AvailableResourceMode

	return updated
}

// updateClusterPreferences updates pinned resources in ClusterUserPreferences.
// The fields are updated one by one (instead of passing the entire struct as new preferences)
// to prevent potential new fields from being overwritten.
func updateClusterPreferences(oldPreferences *userpreferencesv1.ClusterUserPreferences, newPreferences *userpreferencesv1.ClusterUserPreferences) *userpreferencesv1.ClusterUserPreferences {
	updated := oldPreferences

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
