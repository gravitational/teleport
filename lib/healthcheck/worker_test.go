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
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

func Test_newUnstartedWorker(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "example-pg",
		Labels: map[string]string{
			"target": "true",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      listener.Addr().String(),
	})
	require.NoError(t, err)

	tests := []struct {
		desc       string
		cfg        workerConfig
		wantHealth types.TargetHealth
		wantErr    string
	}{
		{
			desc: "disabled",
			cfg: workerConfig{
				HealthCheckCfg: nil,
				Target: Target{
					GetResource: func() types.ResourceWithLabels { return db },
					ResolverFn: func(ctx context.Context) ([]string, error) {
						return []string{db.GetURI()}, nil
					},
				},
			},
			wantHealth: types.TargetHealth{
				Address:          "",
				Protocol:         "",
				Status:           string(types.TargetHealthStatusUnknown),
				TransitionReason: string(types.TargetHealthTransitionReasonDisabled),
				Message:          "No health check config matches this resource",
			},
		},
		{
			desc: "enabled",
			cfg: workerConfig{
				HealthCheckCfg: &healthCheckConfig{
					interval:           time.Minute,
					timeout:            time.Minute,
					healthyThreshold:   10,
					unhealthyThreshold: 10,
				},
				Target: Target{
					GetResource: func() types.ResourceWithLabels { return db },
					ResolverFn: func(ctx context.Context) ([]string, error) {
						return []string{db.GetURI()}, nil
					},
				},
			},
			wantHealth: types.TargetHealth{
				Address:          "",
				Protocol:         "",
				Status:           string(types.TargetHealthStatusUnknown),
				TransitionReason: string(types.TargetHealthTransitionReasonInit),
				Message:          "Health checker initialized",
			},
		},
		{
			desc: "invalid target",
			cfg: workerConfig{
				HealthCheckCfg: &healthCheckConfig{
					interval:           time.Minute,
					timeout:            time.Minute,
					healthyThreshold:   10,
					unhealthyThreshold: 10,
				},
				Target: Target{
					ResolverFn: func(ctx context.Context) ([]string, error) {
						return []string{db.GetURI()}, nil
					},
				},
			},
			wantErr: "missing target resource getter",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			w, err := newUnstartedWorker(ctx, test.cfg)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, w.Close()) })
			require.Empty(t, cmp.Diff(test.wantHealth, *w.GetTargetHealth(),
				cmpopts.IgnoreFields(types.TargetHealth{}, "TransitionTimestamp")),
			)
		})
	}
}

func Test_dialEndpoints(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const healthyAddr = "healthy.com:123"
	const unhealthyAddr = "unhealthy.com:123"
	tests := []struct {
		desc            string
		resolverFn      EndpointsResolverFunc
		wantErrContains string
	}{
		{
			desc: "resolver error",
			resolverFn: func(ctx context.Context) ([]string, error) {
				return nil, trace.Errorf("resolver error")
			},
			wantErrContains: "resolver error",
		},
		{
			desc: "resolved zero addrs",
			resolverFn: func(ctx context.Context) ([]string, error) {
				return nil, nil
			},
			wantErrContains: "resolved zero target endpoints",
		},
		{
			desc: "resolved one healthy addr",
			resolverFn: func(ctx context.Context) ([]string, error) {
				return []string{healthyAddr}, nil
			},
		},
		{
			desc: "resolved one unhealthy addr",
			resolverFn: func(ctx context.Context) ([]string, error) {
				return []string{unhealthyAddr}, nil
			},
			wantErrContains: "unhealthy addr",
		},
		{
			desc: "resolved multiple healthy addrs",
			resolverFn: func(ctx context.Context) ([]string, error) {
				return []string{healthyAddr, healthyAddr, healthyAddr}, nil
			},
		},
		{
			desc: "resolved a mix of healthy and unhealthy addrs",
			resolverFn: func(ctx context.Context) ([]string, error) {
				return []string{healthyAddr, unhealthyAddr, healthyAddr}, nil
			},
			wantErrContains: "unhealthy addr",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			w := &worker{
				healthCheckCfg: &healthCheckConfig{},
				log:            slog.Default(),
				target: Target{
					ResolverFn: test.resolverFn,
					dialFn: func(ctx context.Context, network, addr string) (net.Conn, error) {
						if addr == healthyAddr {
							return fakeConn{}, nil
						}
						return nil, trace.Errorf("unhealthy addr")
					},
				},
			}
			err := w.dialEndpoints(ctx)
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
