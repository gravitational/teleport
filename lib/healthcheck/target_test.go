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

package healthcheck

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type fakeResource struct {
	kind string
	types.ResourceWithLabels
}

func (r *fakeResource) GetKind() string {
	return r.kind
}

func TestTarget_check(t *testing.T) {
	tests := []struct {
		name    string
		target  *Target
		wantErr string
	}{
		{
			name: "valid target",
			target: &Target{
				Resource:             &fakeResource{kind: types.KindDatabase},
				GetUpdatedResourceFn: func() types.ResourceWithLabels { return &fakeResource{kind: types.KindDatabase} },
				ResolverFn:           func(ctx context.Context) ([]string, error) { return []string{"127.0.0.1"}, nil },
			},
		},
		{
			name: "missing resource",
			target: &Target{
				GetUpdatedResourceFn: func() types.ResourceWithLabels { return &fakeResource{kind: types.KindDatabase} },
				ResolverFn:           func(ctx context.Context) ([]string, error) { return []string{"127.0.0.1"}, nil },
			},
			wantErr: "missing target resource",
		},
		{
			name: "missing resource updater",
			target: &Target{
				Resource:   &fakeResource{kind: "node"},
				ResolverFn: func(ctx context.Context) ([]string, error) { return nil, nil },
			},
			wantErr: "missing target resource update getter",
		},
		{
			name: "missing resolver",
			target: &Target{
				Resource:             &fakeResource{kind: types.KindDatabase},
				GetUpdatedResourceFn: func() types.ResourceWithLabels { return nil },
			},
			wantErr: "missing target endpoint resolver",
		},
		{
			name: "unsupported kind",
			target: &Target{
				Resource:             &fakeResource{kind: types.KindNode},
				GetUpdatedResourceFn: func() types.ResourceWithLabels { return nil },
				ResolverFn:           func(ctx context.Context) ([]string, error) { return nil, nil },
			},
			wantErr: `health check target resource kind "node" is not supported`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.check()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
