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
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestTargetDialer_CheckAndSetDefaults(t *testing.T) {
	tests := []struct {
		name    string
		target  *TargetDialer
		wantErr string
	}{
		{
			name: "valid target",
			target: &TargetDialer{
				Resource: func() types.ResourceWithLabels { return &fakeResource{kind: types.KindDatabase} },
				Resolver: func(ctx context.Context) ([]string, error) { return nil, nil },
			},
		},
		{
			name: "missing resource getter",
			target: &TargetDialer{
				Resolver: func(ctx context.Context) ([]string, error) { return nil, nil },
			},
			wantErr: "missing target resource getter",
		},
		{
			name: "missing endpoint resolver",
			target: &TargetDialer{
				Resource: func() types.ResourceWithLabels { return &fakeResource{kind: types.KindDatabase} },
			},
			wantErr: "missing target endpoint resolver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.CheckAndSetDefaults()
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

func TestTargetDialer_dialEndpoints(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const healthyAddr = "healthy.com:123"
	const unhealthyAddr = "unhealthy.com:123"
	tests := []struct {
		desc            string
		resolver        EndpointsResolverFunc
		wantErrContains string
	}{
		{
			desc: "resolver error",
			resolver: func(ctx context.Context) ([]string, error) {
				return nil, trace.Errorf("resolver error")
			},
			wantErrContains: "resolver error",
		},
		{
			desc: "resolved zero addrs",
			resolver: func(ctx context.Context) ([]string, error) {
				return nil, nil
			},
			wantErrContains: "resolved zero target endpoints",
		},
		{
			desc: "resolved one healthy addr",
			resolver: func(ctx context.Context) ([]string, error) {
				return []string{healthyAddr}, nil
			},
		},
		{
			desc: "resolved one unhealthy addr",
			resolver: func(ctx context.Context) ([]string, error) {
				return []string{unhealthyAddr}, nil
			},
			wantErrContains: "unhealthy addr",
		},
		{
			desc: "resolved multiple healthy addrs",
			resolver: func(ctx context.Context) ([]string, error) {
				return []string{healthyAddr, healthyAddr, healthyAddr}, nil
			},
		},
		{
			desc: "resolved a mix of healthy and unhealthy addrs",
			resolver: func(ctx context.Context) ([]string, error) {
				return []string{healthyAddr, unhealthyAddr, healthyAddr}, nil
			},
			wantErrContains: "unhealthy addr",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d := &TargetDialer{
				Resolver: test.resolver,
				dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
					if addr == healthyAddr {
						return fakeConn{}, nil
					}
					return nil, trace.Errorf("unhealthy addr")
				},
			}
			err := d.dialEndpoints(ctx)
			if test.wantErrContains != "" {
				require.ErrorContains(t, err, test.wantErrContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

type fakeConn struct {
	net.Conn
}

func (fakeConn) Close() error { return nil }

type fakeResource struct {
	kind string
	types.ResourceWithLabels
}

func (r *fakeResource) GetKind() string {
	return r.kind
}
