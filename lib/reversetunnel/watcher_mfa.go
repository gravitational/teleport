/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package reversetunnel

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// ValidatedMFAChallengeLister is an interface that wraps the ListValidatedMFAChallenges method.
type ValidatedMFAChallengeLister interface {
	ListValidatedMFAChallenges(
		ctx context.Context,
		in *mfav1.ListValidatedMFAChallengesRequest,
		opts ...grpc.CallOption,
	) (*mfav1.ListValidatedMFAChallengesResponse, error)
}

// ValidatedMFAChallengeWatcherConfig represents the configuration for a ValidatedMFAChallengeWatcher.
type ValidatedMFAChallengeWatcherConfig struct {
	ValidatedMFAChallengeLister ValidatedMFAChallengeLister
	ClusterName                 string
	ResourceWatcherConfig       *services.ResourceWatcherConfig
}

// NewValidatedMFAChallengeWatcher returns a new ValidatedMFAChallengeWatcher.
func NewValidatedMFAChallengeWatcher(
	ctx context.Context,
	cfg ValidatedMFAChallengeWatcherConfig,
) (*services.GenericWatcher[*mfav1.ValidatedMFAChallenge, *mfav1.ValidatedMFAChallenge], error) {
	switch {
	case cfg.ValidatedMFAChallengeLister == nil:
		return nil, trace.BadParameter("cfg.ValidatedMFAChallengeLister must be set")

	case cfg.ClusterName == "":
		return nil, trace.BadParameter("cfg.ClusterName must be set")

	case cfg.ResourceWatcherConfig == nil:
		return nil, trace.BadParameter("cfg.ResourceWatcherConfig must be set")
	}

	paginatedGetFunc := func(ctx context.Context, limit int, startKey string) ([]*mfav1.ValidatedMFAChallenge, string, error) {
		resp, err := cfg.ValidatedMFAChallengeLister.ListValidatedMFAChallenges(
			ctx,
			&mfav1.ListValidatedMFAChallengesRequest{
				PageToken: startKey,
				PageSize:  int32(limit),
				Filter: &mfav1.ListValidatedMFAChallengesFilter{
					XTargetCluster: &mfav1.ListValidatedMFAChallengesFilter_TargetCluster{
						TargetCluster: cfg.ClusterName,
					},
				},
			},
		)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		return resp.GetValidatedChallenges(), resp.GetNextPageToken(), nil
	}

	filter := &types.ValidatedMFAChallengeFilter{
		TargetCluster: cfg.ClusterName,
	}

	w, err := services.NewGenericResourceWatcher(
		ctx,
		services.GenericWatcherConfig[*mfav1.ValidatedMFAChallenge, *mfav1.ValidatedMFAChallenge]{
			ResourceKind:          types.KindValidatedMFAChallenge,
			ResourceFilter:        filter.IntoMap(),
			ResourceWatcherConfig: *cfg.ResourceWatcherConfig,
			CloneFunc:             apiutils.CloneProtoMsg[*mfav1.ValidatedMFAChallenge],
			ReadOnlyFunc:          apiutils.CloneProtoMsg[*mfav1.ValidatedMFAChallenge],
			// This watcher's consumer waits on WaitInitialization before it starts reading ResourcesC. Keep one slot
			// buffered to avoid deadlocking initial broadcast when there are already resources present.
			ResourcesC:                          make(chan []*mfav1.ValidatedMFAChallenge, 1),
			RequireResourcesForInitialBroadcast: false,
			ResourceGetter: func(ctx context.Context) ([]*mfav1.ValidatedMFAChallenge, error) {
				return stream.Collect(clientutils.Resources(ctx, paginatedGetFunc))
			},
			ResourceKey: func(r *mfav1.ValidatedMFAChallenge) string {
				return backend.NewKey(
					r.GetSpec().GetTargetCluster(),
					r.GetMetadata().GetName(),
				).String()
			},
			DeleteKey: func(resource types.Resource) string {
				chal, err := types.ConvertResource[*mfav1.ValidatedMFAChallenge](resource)
				if err != nil {
					return resource.GetMetadata().Description + resource.GetName()
				}

				return backend.NewKey(
					chal.GetSpec().GetTargetCluster(),
					chal.GetMetadata().GetName(),
				).String()
			},
			ResourceDiffer: func(oldR, newR *mfav1.ValidatedMFAChallenge) bool {
				return !proto.Equal(oldR, newR)
			},
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return w, nil
}
