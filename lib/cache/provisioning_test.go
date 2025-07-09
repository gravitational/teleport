// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package cache

import (
	"context"
	"testing"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

const testDownstreamID = services.DownstreamID("weyland-yutani")

func newProvisioningPrincipalState(id string) *provisioningv1.PrincipalState {
	return &provisioningv1.PrincipalState{
		Kind:    types.KindProvisioningPrincipalState,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &provisioningv1.PrincipalStateSpec{
			DownstreamId:  string(testDownstreamID),
			PrincipalType: provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
			PrincipalId:   id,
		},
		Status: &provisioningv1.PrincipalStateStatus{
			ProvisioningState: provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED,
		},
	}
}

// TestProvisioningPrincipalState asserts that a ProvisioningPrincipalState can be cached
func TestProvisioningPrincipalState(t *testing.T) {
	t.Parallel()

	fixturePack := newTestPack(t, ForAuth)
	t.Cleanup(fixturePack.Close)

	testResources153(t, fixturePack, testFuncs153[*provisioningv1.PrincipalState]{
		newResource: func(s string) (*provisioningv1.PrincipalState, error) {
			return newProvisioningPrincipalState(s), nil
		},
		create: func(ctx context.Context, item *provisioningv1.PrincipalState) error {
			_, err := fixturePack.provisioningStates.CreateProvisioningState(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *provisioningv1.PrincipalState) error {
			_, err := fixturePack.provisioningStates.UpdateProvisioningState(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*provisioningv1.PrincipalState, error) {
			var result []*provisioningv1.PrincipalState
			var pageToken pagination.PageRequestToken
			for {
				page, nextPage, err := fixturePack.provisioningStates.ListProvisioningStatesForAllDownstreams(ctx, 0, &pageToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				result = append(result, page...)

				if nextPage == pagination.EndOfList {
					break
				}

				pageToken.Update(nextPage)
			}
			return result, nil
		},
		delete: func(ctx context.Context, id string) error {
			return trace.Wrap(fixturePack.provisioningStates.DeleteProvisioningState(
				ctx, testDownstreamID, services.ProvisioningStateID(id)))
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(fixturePack.provisioningStates.DeleteAllProvisioningStates(ctx))
		},
		cacheList: func(ctx context.Context) ([]*provisioningv1.PrincipalState, error) {
			var result []*provisioningv1.PrincipalState
			var pageToken pagination.PageRequestToken
			for {
				page, nextPage, err := fixturePack.cache.ListProvisioningStatesForAllDownstreams(ctx, 0, &pageToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				result = append(result, page...)

				if nextPage == pagination.EndOfList {
					break
				}

				pageToken.Update(nextPage)
			}
			return result, nil
		},
		cacheGet: func(ctx context.Context, id string) (*provisioningv1.PrincipalState, error) {
			r, err := fixturePack.provisioningStates.GetProvisioningState(
				ctx, testDownstreamID, services.ProvisioningStateID(id))
			return r, trace.Wrap(err)
		},
	})
}
