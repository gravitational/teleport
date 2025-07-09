/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package endpoints

import (
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGetResolver(t *testing.T) {
	tests := []struct {
		desc    string
		builder ResolverBuilder
		wantErr string
	}{
		{
			desc:    "valid",
			builder: fakeResolverBuilder{}.builderFunc(),
		},
		{
			desc:    "builder error",
			builder: fakeResolverBuilder{builderErr: trace.Errorf("failed to build resolver")}.builderFunc(),
			wantErr: "failed to build resolver",
		},
		{
			desc:    "builder not registered",
			builder: nil,
			wantErr: "is not registered",
		},
	}

	ctx := context.Background()
	for i, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			db := &types.DatabaseV3{}
			db.SetName("dummy")
			db.Spec.Protocol = fmt.Sprintf("fake-%v", i)
			if test.builder != nil {
				RegisterResolver(test.builder, db.Spec.Protocol)
				t.Cleanup(func() {
					resolverBuildersMu.Lock()
					defer resolverBuildersMu.Unlock()
					delete(resolverBuilders, db.Spec.Protocol)
				})
			}

			resolver, err := GetResolver(ctx, db, ResolverBuilderConfig{})
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, resolver)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resolver)
		})
	}
}

type fakeResolverBuilder struct {
	builderErr error
}

func (f fakeResolverBuilder) builderFunc() ResolverBuilder {
	return func(context.Context, types.Database, ResolverBuilderConfig) (Resolver, error) {
		if f.builderErr != nil {
			return nil, trace.Wrap(f.builderErr)
		}
		return fakeResolver{}, nil
	}
}

type fakeResolver struct{}

func (f fakeResolver) Resolve(context.Context) ([]string, error) {
	return nil, nil
}
