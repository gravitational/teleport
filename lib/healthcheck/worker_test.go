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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

func Test_newUnstartedWorker(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	protocol := string(types.TargetHealthProtocolTCP)
	listener, err := net.Listen(protocol, "localhost:0")
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
					HealthChecker: NewTargetDialer(
						func(ctx context.Context) ([]string, error) { return nil, nil },
					),
					GetResource: func() types.ResourceWithLabels { return db },
				},
			},
			wantHealth: types.TargetHealth{
				Address:          "",
				Protocol:         protocol,
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
					HealthChecker: NewTargetDialer(
						func(ctx context.Context) ([]string, error) { return nil, nil },
					),
					GetResource: func() types.ResourceWithLabels { return db },
				},
				getTargetHealthTimeout: time.Millisecond,
			},
			wantHealth: types.TargetHealth{
				Address:          "",
				Protocol:         protocol,
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
					HealthChecker: NewTargetDialer(
						func(ctx context.Context) ([]string, error) { return nil, nil },
					),
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
