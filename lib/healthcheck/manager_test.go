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
	"errors"
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
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
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
	t.Parallel()
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
			mgr, err := NewManager(context.Background(), test.cfg)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, mgr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, mgr.Close())
			require.NotNil(t, mgr)
		})
	}
}

func TestManager(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	t.Cleanup(cancel)
	bk, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	healthConfigSvc, err := local.NewHealthCheckConfigService(bk)
	require.NoError(t, err)

	// create a health check config that only matches prod databases
	prodHCC := healthCheckConfigFixture(t, "prod")
	prodHCC.Spec.Match.DbLabelsExpression = `labels.env == "prod"`
	_, err = healthConfigSvc.CreateHealthCheckConfig(ctx, prodHCC)
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()
	eventsCh := make(chan testEvent, 1024)
	mgr, err := NewManager(ctx, ManagerConfig{
		Component:               "test",
		Events:                  local.NewEventsService(bk),
		HealthCheckConfigReader: healthConfigSvc,
		Clock:                   clock,
	})
	require.NoError(t, err)
	require.NoError(t, mgr.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, mgr.Close())
	})

	{
		mgr := mgr.(*manager)
		mgr.mu.RLock()
		configs := mgr.configs[:]
		mgr.mu.RUnlock()
		require.Len(t, configs, 1, "starting the manager should have blocked until configs were initialized")
	}

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
	prodDialer := fakeDialer{}
	err = mgr.AddTarget(Target{
		GetResource: func() types.ResourceWithLabels {
			endpointMu.Lock()
			defer endpointMu.Unlock()
			return prodDB
		},
		ResolverFn: func(ctx context.Context) ([]string, error) {
			endpointMu.Lock()
			defer endpointMu.Unlock()
			return []string{prodDB.GetURI()}, nil
		},
		dialFn: prodDialer.DialContext,
		onHealthCheck: func(lastResultErr error) {
			eventsCh <- lastResultTestEvent(prodDB.GetName(), lastResultErr)
		},
		onConfigUpdate: func() {
			eventsCh <- configUpdateTestEvent(prodDB.GetName())
		},
		onClose: func() {
			eventsCh <- closedTestEvent(prodDB.GetName())
		},
	})
	require.NoError(t, err)

	devDialer := fakeDialer{}
	err = mgr.AddTarget(Target{
		GetResource: func() types.ResourceWithLabels {
			endpointMu.Lock()
			defer endpointMu.Unlock()
			return devDB
		},
		ResolverFn: func(ctx context.Context) ([]string, error) {
			endpointMu.Lock()
			defer endpointMu.Unlock()
			return []string{devDB.GetURI()}, nil
		},
		dialFn: devDialer.DialContext,
		onHealthCheck: func(lastResultErr error) {
			eventsCh <- lastResultTestEvent(devDB.GetName(), lastResultErr)
		},
		onConfigUpdate: func() {
			eventsCh <- configUpdateTestEvent(devDB.GetName())
		},
		onClose: func() {
			eventsCh <- closedTestEvent(devDB.GetName())
		},
	})
	require.NoError(t, err)

	t.Run("duplicate target is an error", func(t *testing.T) {
		err = mgr.AddTarget(Target{
			GetResource: func() types.ResourceWithLabels { return devDB },
			ResolverFn:  func(ctx context.Context) ([]string, error) { return nil, nil },
		})
		require.Error(t, err)
		require.IsType(t, trace.AlreadyExists(""), err)
	})
	t.Run("unsupported target resource is an error", func(t *testing.T) {
		err = mgr.AddTarget(Target{
			GetResource: func() types.ResourceWithLabels { return &fakeResource{kind: "node"} },
			ResolverFn:  func(ctx context.Context) ([]string, error) { return nil, nil },
		})
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)
	})

	requireTargetHealth := func(t *testing.T, r types.ResourceWithLabels, status types.TargetHealthStatus, reason types.TargetHealthTransitionReason) {
		t.Helper()
		health, err := mgr.GetTargetHealth(r)
		require.NoError(t, err)
		require.Equal(t, string(status), health.Status)
		require.Equal(t, string(reason), health.TransitionReason)
	}

	prodPass := lastResultPassTestEvent(prodDB.GetName())
	prodFail := lastResultFailTestEvent(prodDB.GetName())
	// initially checks should be disabled for dev but enabled for prod
	awaitTestEvents(t, eventsCh, nil,
		expect(prodPass),
		deny(prodFail),
		denyAll(devDB.GetName()),
	)
	requireTargetHealth(t, devDB, types.TargetHealthStatusUnknown, types.TargetHealthTransitionReasonDisabled)
	requireTargetHealth(t, prodDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold)

	// another check should reach the prodHCC configured threshold.
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC),
		expect(prodPass),
		deny(prodFail),
		denyAll(devDB.GetName()),
	)
	requireTargetHealth(t, prodDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold)

	// now reject health check connections to simulate an unhealthy endpoint
	prodDialer.deny()
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC),
		advanceCount(2),
		expect(prodFail, prodFail),
		deny(prodPass),
		denyAll(devDB.GetName()))
	requireTargetHealth(t, prodDB, types.TargetHealthStatusUnhealthy, types.TargetHealthTransitionReasonThreshold)

	// enable dev health checks for dev db and simulate an unhealthy endpoint on init
	devDialer.deny()
	devHCC := healthCheckConfigFixture(t, "dev")
	devHCC.Spec.Match.DbLabelsExpression = `labels.env == "dev"`
	devHCC, err = healthConfigSvc.CreateHealthCheckConfig(ctx, devHCC)
	require.NoError(t, err)

	devConfigUpdate := configUpdateTestEvent(devDB.GetName())
	awaitTestEvents(t, eventsCh, nil,
		expect(devConfigUpdate),
		denyAll(prodDB.GetName()),
	)
	// the first failing check after dev checks are enabled will transition to unhealthy regardless of unhealthy threshold
	devPass := lastResultPassTestEvent(devDB.GetName())
	devFail := lastResultFailTestEvent(devDB.GetName())
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC, devHCC),
		expect(devFail, prodFail),
		deny(prodPass, devPass),
	)
	requireTargetHealth(t, devDB, types.TargetHealthStatusUnhealthy, types.TargetHealthTransitionReasonThreshold)

	// the next dev check should update health status because the unhealthy threshold was met
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC, devHCC),
		expect(devFail, prodFail),
		deny(prodPass, devPass),
	)
	requireTargetHealth(t, devDB, types.TargetHealthStatusUnhealthy, types.TargetHealthTransitionReasonThreshold)

	// set the unhealthy threshold high for dev, so we can simulate several
	// failing checks after dev becomes healthy without making it become unhealthy
	devHCC.Spec.UnhealthyThreshold = 10
	devHCC, err = healthConfigSvc.UpsertHealthCheckConfig(ctx, devHCC)
	require.NoError(t, err)
	awaitTestEvents(t, eventsCh, nil,
		expect(devConfigUpdate),
		denyAll(prodDB.GetName()),
	)

	// now simulate the endpoints becoming healthy again
	prodDialer.allow()
	devDialer.allow()
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC, devHCC),
		expect(devPass, prodPass),
		deny(devFail, prodFail),
	)
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC, devHCC),
		expect(devPass, prodPass),
		deny(devFail, prodFail),
	)
	requireTargetHealth(t, devDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold)
	requireTargetHealth(t, prodDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold)

	// now disable the prod health checks
	err = healthConfigSvc.DeleteHealthCheckConfig(ctx, "prod")
	require.NoError(t, err)

	awaitTestEvents(t, eventsCh, nil,
		expect(configUpdateTestEvent(prodDB.GetName())),
		denyAll(devDB.GetName()),
	)
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC, devHCC),
		advanceCount(2),
		expect(devPass, devPass),
		deny(devFail, prodFail),
	)
	// prod db should be disabled eventually
	requireTargetHealth(t, prodDB, types.TargetHealthStatusUnknown, types.TargetHealthTransitionReasonDisabled)
	// but dev db should still be healthy
	requireTargetHealth(t, devDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold)

	// fail some checks, then update config to lower the unhealthy threshold
	devDialer.deny()
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC, devHCC),
		advanceCount(3),
		expect(devFail, devFail, devFail),
		denyAll(prodDB.GetName()),
	)
	requireTargetHealth(t, devDB, types.TargetHealthStatusHealthy, types.TargetHealthTransitionReasonThreshold)
	devHCC.Spec.UnhealthyThreshold = 1
	devHCC.Spec.Interval = durationpb.New(time.Second * 100)
	_, err = healthConfigSvc.UpdateHealthCheckConfig(ctx, devHCC)
	require.NoError(t, err)
	awaitTestEvents(t, eventsCh, nil,
		expect(devConfigUpdate),
		denyAll(prodDB.GetName()),
		// dev shouldn't need to run any checks to observe this behavior
		deny(devPass, devFail),
	)
	// config update should set unhealthy status since the new threshold is already met
	requireTargetHealth(t, devDB, types.TargetHealthStatusUnhealthy, types.TargetHealthTransitionReasonThreshold)

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
	err = mgr.RemoveTarget(prodDB)
	require.NoError(t, err)

	// make sure the workers have stopped.
	awaitTestEvents(t, eventsCh, clock,
		advanceByHCC(prodHCC, devHCC),
		expect(closedTestEvent(devDB.GetName()), closedTestEvent(prodDB.GetName())),
	)
}

func healthCheckConfigFixture(t *testing.T, name string) *healthcheckconfigv1.HealthCheckConfig {
	t.Helper()
	out, err := healthcheckconfig.NewHealthCheckConfig(name,
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Match: &healthcheckconfigv1.Matcher{
				DbLabels: []*labelv1.Label{{
					Name:   types.Wildcard,
					Values: []string{types.Wildcard},
				}},
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

type testEventOptions struct {
	expect       map[testEvent]int
	assertions   []func(*testing.T, testEvent) bool
	advance      time.Duration
	advanceCount int
}

type eventOption func(*testEventOptions)

func expect(events ...testEvent) eventOption {
	return func(opts *testEventOptions) {
		for _, evt := range events {
			opts.expect[evt] = opts.expect[evt] + 1
		}
	}
}

func deny(events ...testEvent) eventOption {
	denied := make(map[testEvent]struct{})
	for _, event := range events {
		denied[event] = struct{}{}
	}
	return func(opts *testEventOptions) {
		opts.assertions = append(opts.assertions, func(t *testing.T, event testEvent) bool {
			t.Helper()
			return assert.NotContains(t, denied, event, "observed a denied event")
		})
	}
}

// denyAll denies all events associated with the target
func denyAll(deniedTarget string) eventOption {
	return func(opts *testEventOptions) {
		opts.assertions = append(opts.assertions, func(t *testing.T, event testEvent) bool {
			t.Helper()
			return assert.NotEqual(t, deniedTarget, event.target, "unexpected event target")
		})
	}
}

// advanceByHCC advances the clock by the largest interval to trigger all matching targets to run a check
func advanceByHCC(first *healthcheckconfigv1.HealthCheckConfig, rest ...*healthcheckconfigv1.HealthCheckConfig) eventOption {
	max := first.GetSpec().GetInterval().AsDuration()
	for _, hcc := range rest {
		d := hcc.GetSpec().GetInterval().AsDuration()
		if d > max {
			max = d
		}
	}
	return advanceBy(max)
}

func advanceBy(d time.Duration) eventOption {
	return func(opts *testEventOptions) {
		opts.advance = d
	}
}

// advanceCount advance the clock clock times.
// Probably a bad idea to advance more than once when you expect more than one type of event in the await loop.
func advanceCount(count int) eventOption {
	return func(opts *testEventOptions) {
		opts.advanceCount = count
	}
}

func awaitTestEvents(t *testing.T, ch <-chan testEvent, clock *clockwork.FakeClock, optFns ...eventOption) {
	t.Helper()
	options := testEventOptions{
		expect:       make(map[testEvent]int),
		advance:      30 * time.Second,
		advanceCount: 1,
	}
	for _, o := range optFns {
		o(&options)
	}
	for {
		if len(options.expect) == 0 {
			return
		}
		if clock != nil && options.advanceCount > 0 {
			options.advanceCount--
			clock.Advance(options.advance)
		}

		select {
		case event := <-ch:
			for _, denyFn := range options.assertions {
				if !denyFn(t, event) {
					require.Failf(t, "unexpected event", "event=%v", event)
				}
			}

			options.expect[event] = options.expect[event] - 1
			if options.expect[event] < 1 {
				delete(options.expect, event)
			}
		case <-time.After(time.Second * 5):
			require.Failf(t, "timeout waiting for events", "expect=%+v", options.expect)
		}
	}
}

type fakeDialer struct {
	net.Dialer
	mu      sync.RWMutex
	fakeErr error
}

func (f *fakeDialer) allow() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fakeErr = nil
}

func (f *fakeDialer) deny() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fakeErr = errors.New("fake error in test")
}

func (f *fakeDialer) getFakeErr() error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fakeErr
}

func (f *fakeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if err := f.getFakeErr(); err != nil {
		return nil, err
	}
	return f.Dialer.DialContext(ctx, network, addr)
}

type testEvent struct {
	name   testEventName
	target string
}

type testEventName string

const (
	closed         testEventName = "closed"
	configUpdate   testEventName = "configUpdate"
	lastResultPass testEventName = "lastResultPass"
	lastResultFail testEventName = "lastResultFail"
)

func closedTestEvent(targetName string) testEvent {
	return testEvent{
		name:   closed,
		target: targetName,
	}
}

func configUpdateTestEvent(targetName string) testEvent {
	return testEvent{
		name:   configUpdate,
		target: targetName,
	}
}

func lastResultTestEvent(targetName string, err error) testEvent {
	if err != nil {
		return lastResultFailTestEvent(targetName)
	}
	return lastResultPassTestEvent(targetName)
}

func lastResultPassTestEvent(targetName string) testEvent {
	return testEvent{
		name:   lastResultPass,
		target: targetName,
	}
}

func lastResultFailTestEvent(targetName string) testEvent {
	return testEvent{
		name:   lastResultFail,
		target: targetName,
	}
}
