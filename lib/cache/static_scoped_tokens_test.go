// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"testing/synctest"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

func newStaticScopedTokens() *joiningv1.StaticScopedTokens {
	return &joiningv1.StaticScopedTokens{
		Kind:  types.KindStaticScopedTokens,
		Scope: "/",
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameStaticScopedTokens,
		},
		Spec: &joiningv1.StaticScopedTokensSpec{
			Tokens: []*joiningv1.ScopedToken{
				{
					Kind:  types.KindScopedToken,
					Scope: "/",
					Metadata: &headerv1.Metadata{
						Name:    "tok1",
						Expires: timestamppb.New(time.Now().UTC().Add(time.Hour)),
					},
					Spec: &joiningv1.ScopedTokenSpec{
						Roles:         []string{types.RoleNode.String()},
						JoinMethod:    string(types.JoinMethodToken),
						UsageMode:     string(joining.TokenUsageModeUnlimited),
						AssignedScope: "/local",
					},
				},
			},
		},
	}
}

// TestStaticScopedTokens tests that CRUD operations on the StaticScopedTokens resource are
// replicated from the backend to the cache.
func TestStaticScopedTokens(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)
		testSingleton153(t, p, testSingletonFuncs153[*joiningv1.StaticScopedTokens]{
			newResource: newStaticScopedTokens,
			create: func(ctx context.Context, sst *joiningv1.StaticScopedTokens) (*joiningv1.StaticScopedTokens, error) {
				// Does not obey 153 semantics.
				return sst, p.clusterConfigS.SetStaticScopedTokens(ctx, sst)
			},
			update: func(ctx context.Context, sst *joiningv1.StaticScopedTokens) (*joiningv1.StaticScopedTokens, error) {
				// Does not obey 153 semantics.
				return sst, p.clusterConfigS.SetStaticScopedTokens(ctx, sst)
			},
			get:      p.clusterConfigS.GetStaticScopedTokens,
			cacheGet: p.cache.GetStaticScopedTokens,
			delete:   p.clusterConfigS.DeleteStaticScopedTokens,
		})
	})
}
