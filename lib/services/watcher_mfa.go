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

package services

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
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
	ResourceWatcherConfig       *ResourceWatcherConfig
}

// NewValidatedMFAChallengeWatcher returns a new ValidatedMFAChallengeWatcher.
func NewValidatedMFAChallengeWatcher(
	ctx context.Context,
	cfg ValidatedMFAChallengeWatcherConfig,
) (*GenericWatcher[*mfav1.ValidatedMFAChallenge, *mfav1.ValidatedMFAChallenge], error) {
	switch {
	case cfg.ValidatedMFAChallengeLister == nil:
		return nil, trace.BadParameter("cfg.ValidatedMFAChallengeGetter must be set")

	case cfg.ResourceWatcherConfig == nil:
		return nil, trace.BadParameter("cfg.ResourceWatcherConfig must be set")
	}

	cloneFunc := func(r *mfav1.ValidatedMFAChallenge) *mfav1.ValidatedMFAChallenge {
		return proto.Clone(r).(*mfav1.ValidatedMFAChallenge)
	}

	paginatedGetFunc := func(ctx context.Context, limit int, startKey string) ([]*mfav1.ValidatedMFAChallenge, string, error) {
		resp, err := cfg.ValidatedMFAChallengeLister.ListValidatedMFAChallenges(
			ctx,
			&mfav1.ListValidatedMFAChallengesRequest{
				PageToken: startKey,
				PageSize:  int32(limit),
			},
		)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		return resp.GetValidatedChallenges(), resp.GetNextPageToken(), nil
	}

	w, err := NewGenericResourceWatcher(
		ctx,
		GenericWatcherConfig[*mfav1.ValidatedMFAChallenge, *mfav1.ValidatedMFAChallenge]{
			ResourceKind:          types.KindValidatedMFAChallenge,
			ResourceWatcherConfig: *cfg.ResourceWatcherConfig,
			CloneFunc:             cloneFunc,
			ReadOnlyFunc:          cloneFunc,
			ResourceGetter:        pagerFn[*mfav1.ValidatedMFAChallenge](paginatedGetFunc).getAll,
			ResourceKey: func(r *mfav1.ValidatedMFAChallenge) string {
				// See lib/services/local/mfa.go#createValidatedMFAChallenge for how ValidatedMFAChallenge keys are
				// constructed. We need to construct the same key here to ensure the watcher can properly match updates
				// to existing resources.
				return backend.NewKey(
					r.GetSpec().GetUsername(),
					r.GetMetadata().GetName(),
				).String()
			},
			DeleteKey: func(_ types.Resource) string {
				return "NOOP: ValidatedMFAChallenges are never deleted, they expire instead"
			},
			ResourceDiffer: func(oldR, newR *mfav1.ValidatedMFAChallenge) bool {
				return proto.Equal(oldR, newR)
			},
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return w, nil
}
