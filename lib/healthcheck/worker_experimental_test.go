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
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types"
)

func TestGetTargetHealth(t *testing.T) {
	t.Parallel()
	enabledHCC := healthCheckConfig{
		interval:           time.Minute,
		timeout:            time.Minute,
		healthyThreshold:   10,
		unhealthyThreshold: 10,
	}
	tests := []struct {
		desc              string
		healthCheckConfig *healthCheckConfig
		dialErr           error
		wantStatus        types.TargetHealthStatus
		wantReason        types.TargetHealthTransitionReason
	}{
		{
			desc:              "healthy",
			healthCheckConfig: &enabledHCC,
			wantStatus:        "healthy",
			wantReason:        types.TargetHealthTransitionReasonThreshold,
		},
		{
			desc:       "disabled",
			wantStatus: "unknown",
			wantReason: types.TargetHealthTransitionReasonDisabled,
		},
		{
			desc:              "unhealthy",
			healthCheckConfig: &enabledHCC,
			dialErr:           trace.Errorf("error dialing"),
			wantStatus:        "unhealthy",
			wantReason:        types.TargetHealthTransitionReasonThreshold,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				worker, err := newWorker(t.Context(), workerConfig{
					HealthCheckCfg: test.healthCheckConfig,
					Target: Target{
						GetResource: func() types.ResourceWithLabels { return nil },
						ResolverFn: func(ctx context.Context) ([]string, error) {
							return []string{"localhost:1234"}, nil
						},
						dialFn: func(ctx context.Context, network, addr string) (net.Conn, error) {
							time.Sleep(5*time.Second - time.Nanosecond)
							synctest.Wait()
							return fakeConn{}, test.dialErr
						},
					},
				})
				assert.NoError(t, err)
				defer worker.Close()
				health := worker.GetTargetHealth()
				assert.Equal(t, test.wantStatus, types.TargetHealthStatus(health.Status))
				assert.Equal(t, test.wantReason, types.TargetHealthTransitionReason(health.TransitionReason))
			})
		})
	}
}
