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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type mockEvents struct {
	types.Events
}

type mockHealthCheckConfigReader struct {
	services.HealthCheckConfigReader
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		desc    string
		wantErr string
		cfg     ManagerConfig
	}{
		{
			desc:    "missing Component",
			wantErr: "missing Component",
			cfg: ManagerConfig{
				Events:                  &mockEvents{},
				HealthCheckConfigReader: &mockHealthCheckConfigReader{},
			},
		},
		{
			desc:    "missing Events",
			wantErr: "missing Events",
			cfg: ManagerConfig{
				Component:               "test-component",
				HealthCheckConfigReader: &mockHealthCheckConfigReader{},
			},
		},
		{
			desc:    "missing HealthCheckConfigReader",
			wantErr: "missing HealthCheckConfigReader",
			cfg: ManagerConfig{
				Component: "test-component",
				Events:    &mockEvents{},
			},
		},
		{
			desc: "success with missing clock applies default",
			cfg: ManagerConfig{
				Component:               "test-component",
				Events:                  &mockEvents{},
				HealthCheckConfigReader: &mockHealthCheckConfigReader{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			mgr, err := NewManager(test.cfg)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, mgr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, mgr)
		})
	}
}

func TestManager(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	healthConfigSvc, err := local.NewHealthCheckConfigService(bk)
	require.NoError(t, err)

	// create a health check config that only matches prod databases
	prodHCC := healthCheckConfigFixture(t, "prod")
	prodHCC.Spec.Match.DbLabelsExpression = `labels.env == "prod"`
	_, err = healthConfigSvc.CreateHealthCheckConfig(ctx, prodHCC)
	require.NoError(t, err)

	mgr, err := NewManager(ManagerConfig{
		Component:               "test",
		Events:                  local.NewEventsService(bk),
		HealthCheckConfigReader: healthConfigSvc,
		Clock:                   clock,
	})
	require.NoError(t, err)
	require.NoError(t, mgr.Start(ctx))

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	prodDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "prodDB",
		Labels: map[string]string{
			"env": "prod",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      listener.Addr().String(),
	})
	require.NoError(t, err)
	devDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "devDB",
		Labels: map[string]string{
			"env": "dev",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      listener.Addr().String(),
	})
	require.NoError(t, err)

	var endpointMu sync.Mutex
	enableEndpoint := func() {
		endpointMu.Lock()
		defer endpointMu.Unlock()
		prodDB.SetURI(listener.Addr().String())
		devDB.SetURI(listener.Addr().String())
	}
	disableEndpoint := func() {
		endpointMu.Lock()
		defer endpointMu.Unlock()
		// use an unresolvable domain [1], instead of just closing the listener,
		// to avoid test flakiness in the case where some other test or service
		// re-uses the closed listener port.
		// [1] https://datatracker.ietf.org/doc/html/rfc2606
		const unresolvable = "example.invalid:12345"
		prodDB.SetURI(unresolvable)
		devDB.SetURI(unresolvable)
	}

	err = mgr.AddTarget(ctx, Target{
		Resource:             prodDB,
		GetUpdatedResourceFn: func() types.ResourceWithLabels { return prodDB },
		ResolverFn: func(ctx context.Context) ([]string, error) {
			endpointMu.Lock()
			defer endpointMu.Unlock()
			return []string{prodDB.GetURI()}, nil
		},
	})
	require.NoError(t, err)
	err = mgr.AddTarget(ctx, Target{
		Resource:             devDB,
		GetUpdatedResourceFn: func() types.ResourceWithLabels { return devDB },
		ResolverFn: func(ctx context.Context) ([]string, error) {
			endpointMu.Lock()
			defer endpointMu.Unlock()
			return []string{devDB.GetURI()}, nil
		},
	})
	require.NoError(t, err)

	// try to add a duplicate target
	err = mgr.AddTarget(ctx, Target{
		Resource:             devDB,
		GetUpdatedResourceFn: func() types.ResourceWithLabels { return nil },
		ResolverFn:           func(ctx context.Context) ([]string, error) { return nil, nil },
	})
	require.Error(t, err)
	require.IsType(t, trace.AlreadyExists(""), err)

	requireTargetHealth := func(t *testing.T, r types.ResourceWithLabels, status types.TargetHealthStatus, reason types.TargetHealthTransitionReason) {
		t.Helper()
		health, err := mgr.GetTargetHealth(r)
		require.NoError(t, err)
		require.Equal(t, string(status), health.Status)
		require.Equal(t, string(reason), health.TransitionReason)
	}
	waitForTargetHealth := func(t *testing.T, r types.ResourceWithLabels, status types.TargetHealthStatus, reason types.TargetHealthTransitionReason, advance bool) {
		t.Helper()
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			if advance {
				clock.Advance(prodHCC.Spec.Interval.AsDuration())
			}
			health, err := mgr.GetTargetHealth(r)
			assert.NoError(t, err)
			assert.Equal(t, string(status), health.Status)
			assert.Equal(t, string(reason), health.TransitionReason)
		}, 5*time.Second, 100*time.Millisecond, "waiting for health update")
	}

	// initially checks should be disabled for dev but enabled for prod
	requireTargetHealth(t, devDB, types.TargetHealthStatusUnknown, types.TargetHealthTransitionReasonDisabled)
	waitForTargetHealth(t, prodDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonInit, true)

	// now reject health check connections to simulate an unhealthy endpoint
	disableEndpoint()
	waitForTargetHealth(t, prodDB, types.TargetHealthStatusUnhealthy, types.TargetHealthTransitionReasonThreshold, true)

	// enable dev health checks for dev db
	devHCC := healthCheckConfigFixture(t, "dev")
	devHCC.Spec.Match.DbLabelsExpression = `labels.env == "dev"`
	devHCC.Spec.UnhealthyThreshold = 5
	devHCC, err = healthConfigSvc.CreateHealthCheckConfig(ctx, devHCC)
	require.NoError(t, err)

	// the first failing check after dev checks are enabled will transition to unhealthy regardless of unhealthy threshold
	waitForTargetHealth(t, devDB, types.TargetHealthStatusUnhealthy, types.TargetHealthTransitionReasonInit, true)
	// the next dev check should update health status because the unhealthy threshold was met
	waitForTargetHealth(t, devDB, types.TargetHealthStatusUnhealthy, types.TargetHealthTransitionReasonThreshold, true)

	// now reset the listener to simulate the endpoint becoming healthy again
	enableEndpoint()
	waitForTargetHealth(t, devDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold, true)
	waitForTargetHealth(t, prodDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold, true)

	// now disable the prod health checks
	err = healthConfigSvc.DeleteHealthCheckConfig(ctx, "prod")
	require.NoError(t, err)
	// prod db should be disabled eventually
	waitForTargetHealth(t, prodDB, types.TargetHealthStatusUnknown, types.TargetHealthTransitionReasonDisabled, true)
	// but dev db should still be healthy
	requireTargetHealth(t, devDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold)

	// fail 1-2 checks (timing dependent), then update config to lower the unhealthy threshold
	disableEndpoint()
	clock.Advance(devHCC.GetSpec().GetInterval().AsDuration())
	requireTargetHealth(t, devDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold)
	devHCC.Spec.UnhealthyThreshold = 1
	devHCC.Spec.Interval = durationpb.New(time.Second * 100)
	_, err = healthConfigSvc.UpdateHealthCheckConfig(ctx, devHCC)
	require.NoError(t, err)
	// config update should set unhealthy status since the new threshold is already met
	waitForTargetHealth(t, devDB, types.TargetHealthStatusUnhealthy, types.TargetHealthTransitionReasonThreshold, false)

	// remove a target
	err = mgr.RemoveTarget(devDB)
	require.NoError(t, err)
	// shouldn't be any target health after the target is removed
	_, err = mgr.GetTargetHealth(devDB)
	require.Error(t, err)
	require.IsType(t, trace.NotFound(""), err)

	err = mgr.RemoveTarget(devDB)
	require.Error(t, err)
	require.IsType(t, trace.NotFound(""), err)

	// prodDB should still be disabled
	requireTargetHealth(t, prodDB, types.TargetHealthStatusUnknown, types.TargetHealthTransitionReasonDisabled)
}

func healthCheckConfigFixture(t *testing.T, name string) *healthcheckconfigv1.HealthCheckConfig {
	t.Helper()
	out, err := healthcheckconfig.NewHealthCheckConfig(name,
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Match: &healthcheckconfigv1.Matcher{
				DbLabelsExpression: `labels["*"] == "*"`,
			},
			Interval:           durationpb.New(apidefaults.HealthCheckInterval),
			Timeout:            durationpb.New(apidefaults.HealthCheckTimeout),
			HealthyThreshold:   2,
			UnhealthyThreshold: 2,
		},
	)
	require.NoError(t, err)
	return out
}
