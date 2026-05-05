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

package vnet

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordedCmd struct {
	path string
	args []string
}

// cmdRecorder captures runCommand invocations for tests.
type cmdRecorder struct {
	cmds []recordedCmd
}

func (r *cmdRecorder) run(_ context.Context, path string, args ...string) error {
	r.cmds = append(r.cmds, recordedCmd{path: path, args: args})
	return nil
}

// setupOSConfigTest installs a cmdRecorder in place of runCommand
// and points resolverPath at a temp dir for the duration of t. Tests
// that use this helper mutate package-level vars, so they must not
// call t.Parallel.
func setupOSConfigTest(t *testing.T) *cmdRecorder {
	t.Helper()

	recorder := &cmdRecorder{}

	prevRunCommand := runCommand
	runCommand = recorder.run
	t.Cleanup(func() { runCommand = prevRunCommand })

	prevResolverPath := resolverPath
	resolverPath = t.TempDir()
	t.Cleanup(func() { resolverPath = prevResolverPath })

	return recorder
}

func TestPlatformConfigureOS_AppliesIPv4OnFirstCall(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}

	require.NoError(t, platformConfigureOS(t.Context(), &osConfig{
		tunName: "utun4",
		tunIPv4: "100.64.0.1",
	}, state))

	require.Equal(t, []recordedCmd{
		{path: "ifconfig", args: []string{"utun4", "100.64.0.1", "100.64.0.1", "up"}},
	}, recorder.cmds)
}

// Regression guard: the per-tick alias flap that motivated this gating.
func TestPlatformConfigureOS_SkipsIPv4OnSecondCall(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := &osConfig{
		tunName: "utun4",
		tunIPv4: "100.64.0.1",
	}

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	require.Equal(t, []recordedCmd{
		{path: "ifconfig", args: []string{"utun4", "100.64.0.1", "100.64.0.1", "up"}},
	}, recorder.cmds, "second call must not re-run ifconfig")
}

func TestPlatformConfigureOS_AppliesNewCidrRange(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}

	require.NoError(t, platformConfigureOS(t.Context(), &osConfig{
		tunName:    "utun4",
		tunIPv4:    "100.64.0.1",
		cidrRanges: []string{"100.64.0.0/10"},
	}, state))
	require.NoError(t, platformConfigureOS(t.Context(), &osConfig{
		tunName:    "utun4",
		tunIPv4:    "100.64.0.1",
		cidrRanges: []string{"100.64.0.0/10", "10.0.0.0/8"},
	}, state))

	require.Equal(t, []recordedCmd{
		{path: "ifconfig", args: []string{"utun4", "100.64.0.1", "100.64.0.1", "up"}},
		{path: "route", args: []string{"add", "-net", "100.64.0.0/10", "-interface", "utun4"}},
		{path: "route", args: []string{"add", "-net", "10.0.0.0/8", "-interface", "utun4"}},
	}, recorder.cmds)
}

func TestPlatformConfigureOS_SkipsExistingCidrRange(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := &osConfig{
		tunName:    "utun4",
		tunIPv4:    "100.64.0.1",
		cidrRanges: []string{"100.64.0.0/10"},
	}

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	require.Equal(t, []recordedCmd{
		{path: "ifconfig", args: []string{"utun4", "100.64.0.1", "100.64.0.1", "up"}},
		{path: "route", args: []string{"add", "-net", "100.64.0.0/10", "-interface", "utun4"}},
	}, recorder.cmds, "second call must not re-run ifconfig or route add")
}

func TestPlatformConfigureOS_SkipsIPv6OnSecondCall(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := &osConfig{
		tunName: "utun4",
		tunIPv6: "fdd4:a23:da97::1",
	}

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	require.Equal(t, []recordedCmd{
		{path: "ifconfig", args: []string{"utun4", "inet6", "fdd4:a23:da97::1", "prefixlen", "64"}},
		{path: "route", args: []string{"add", "-inet6", "fdd4:a23:da97::1", "-prefixlen", "64", "-interface", "utun4"}},
	}, recorder.cmds, "second call must not re-run IPv6 ifconfig or route add")
}

// Deconfigure (empty osConfig) must reset cached state so a
// subsequent populated call re-applies. Pins the invariant that lets
// a future refactor reuse osConfigState across a clean shutdown
// without silently caching stale entries.
func TestPlatformConfigureOS_ReappliesAfterDeconfigure(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := &osConfig{
		tunName: "utun4",
		tunIPv4: "100.64.0.1",
	}
	ifconfigCmd := recordedCmd{
		path: "ifconfig",
		args: []string{"utun4", "100.64.0.1", "100.64.0.1", "up"},
	}

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.Equal(t, []recordedCmd{ifconfigCmd}, recorder.cmds)

	require.NoError(t, platformConfigureOS(t.Context(), &osConfig{}, state))
	require.Equal(t, []recordedCmd{ifconfigCmd}, recorder.cmds,
		"deconfigure call must not run any commands")

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.Equal(t, []recordedCmd{ifconfigCmd, ifconfigCmd}, recorder.cmds,
		"post-deconfigure call must re-apply the original ifconfig")
}

// Pins the "do not cache failed state" invariant: a runCommand error
// must leave the corresponding flag unset so the next call retries.
// Guards against future refactors that move the state assignment
// ahead of the runCommand call.
func TestPlatformConfigureOS_PropagatesRunCommandError(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := &osConfig{
		tunName: "utun4",
		tunIPv4: "100.64.0.1",
	}

	wantErr := errors.New("ifconfig boom")
	runCommand = func(_ context.Context, _ string, _ ...string) error {
		recorder.cmds = append(recorder.cmds, recordedCmd{path: "ifconfig"})
		return wantErr
	}

	err := platformConfigureOS(t.Context(), cfg, state)
	require.ErrorIs(t, err, wantErr)
	require.False(t, state.configuredIPv4,
		"state must not record success on a failing runCommand")

	runCommand = recorder.run

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.True(t, state.configuredIPv4,
		"successful retry must mark state configured")
	require.Len(t, recorder.cmds, 2,
		"failing call plus successful retry must record two ifconfig invocations")
}

// A transient `route add -inet6` failure must leave configuredIPv6Route
// unset so the next tick retries just the route. The ifconfig alias
// must not re-run, otherwise the alias flap returns.
func TestPlatformConfigureOS_RetriesIPv6RouteAfterFailure(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := &osConfig{
		tunName: "utun4",
		tunIPv6: "fdd4:a23:da97::1",
	}

	wantErr := errors.New("route boom")
	runCommand = func(_ context.Context, path string, args ...string) error {
		recorder.cmds = append(recorder.cmds, recordedCmd{path: path, args: args})
		if path == "route" {
			return wantErr
		}
		return nil
	}

	err := platformConfigureOS(t.Context(), cfg, state)
	require.ErrorIs(t, err, wantErr)
	require.True(t, state.configuredIPv6Alias,
		"alias must be marked configured after ifconfig succeeds")
	require.False(t, state.configuredIPv6Route,
		"route must not be marked configured on a failing route add")

	runCommand = recorder.run

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.True(t, state.configuredIPv6Route)

	require.Equal(t, []recordedCmd{
		{path: "ifconfig", args: []string{"utun4", "inet6", "fdd4:a23:da97::1", "prefixlen", "64"}},
		{path: "route", args: []string{"add", "-inet6", "fdd4:a23:da97::1", "-prefixlen", "64", "-interface", "utun4"}},
		{path: "route", args: []string{"add", "-inet6", "fdd4:a23:da97::1", "-prefixlen", "64", "-interface", "utun4"}},
	}, recorder.cmds, "ifconfig must run only once; route must be retried")
}

// Covers the typical production cfg: IPv4, IPv6 and a CIDR together.
func TestPlatformConfigureOS_FullConfigSkipsOnSecondCall(t *testing.T) {
	recorder := setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := &osConfig{
		tunName:    "utun4",
		tunIPv4:    "100.64.0.1",
		tunIPv6:    "fdd4:a23:da97::1",
		cidrRanges: []string{"100.64.0.0/10"},
	}

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	require.Equal(t, []recordedCmd{
		{path: "ifconfig", args: []string{"utun4", "100.64.0.1", "100.64.0.1", "up"}},
		{path: "route", args: []string{"add", "-net", "100.64.0.0/10", "-interface", "utun4"}},
		{path: "ifconfig", args: []string{"utun4", "inet6", "fdd4:a23:da97::1", "prefixlen", "64"}},
		{path: "route", args: []string{"add", "-inet6", "fdd4:a23:da97::1", "-prefixlen", "64", "-interface", "utun4"}},
	}, recorder.cmds, "second call with unchanged cfg must record zero new commands")
}
