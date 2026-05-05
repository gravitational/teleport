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
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// recordedCmd is one (path, args) tuple captured by cmdRecorder.
type recordedCmd struct {
	path string
	args []string
}

// cmdRecorder is a runCommand replacement that records every invocation
// instead of shelling out. Tests use it to assert which OS commands the
// configurator would have run.
type cmdRecorder struct {
	cmds []recordedCmd
}

func (r *cmdRecorder) run(_ context.Context, path string, args ...string) error {
	r.cmds = append(r.cmds, recordedCmd{path: path, args: args})
	return nil
}

// setupOSConfigTestMu serializes tests in this file that mutate package
// vars (runCommand, resolverPath). Concurrent access to those vars
// races; the mutex enforces serialization even if a test author
// forgets and adds t.Parallel.
var setupOSConfigTestMu sync.Mutex

// setupOSConfigTest installs a cmdRecorder in place of runCommand and
// points resolverPath at a temp dir for the duration of t. Acquires
// setupOSConfigTestMu so callers run serially despite mutating package
// vars.
func setupOSConfigTest(t *testing.T) *cmdRecorder {
	t.Helper()

	setupOSConfigTestMu.Lock()
	t.Cleanup(setupOSConfigTestMu.Unlock)

	recorder := &cmdRecorder{}

	prevRunCommand := runCommand
	runCommand = recorder.run
	t.Cleanup(func() { runCommand = prevRunCommand })

	prevResolverPath := resolverPath
	resolverPath = t.TempDir()
	t.Cleanup(func() { resolverPath = prevResolverPath })

	return recorder
}

// TestPlatformConfigureOS_AppliesIPv4OnFirstCall asserts that a first
// call with an IPv4 address records exactly one ifconfig invocation.
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

// TestPlatformConfigureOS_SkipsIPv4OnSecondCall asserts that two calls
// with the same IPv4 config and the same state pointer record exactly
// one ifconfig invocation. Regression guard for the per-10s alias flap
// that occurs when ifconfig is re-issued unconditionally.
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

// TestPlatformConfigureOS_AppliesNewCidrRange asserts that a CIDR range
// added between calls (e.g. user logs in to a second cluster) records
// exactly one new route add and does not re-run the existing CIDR's
// route. Regression guard for the OS-config loop's "pick up new
// clusters" semantics.
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

// TestPlatformConfigureOS_SkipsExistingCidrRange asserts that a second
// call with the same single-element cidrRanges records zero new
// commands. Regression guard for repeated route-add invocations.
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

// TestPlatformConfigureOS_SkipsIPv6OnSecondCall asserts that the IPv6
// ifconfig and the IPv6 route both run only on the first call. The
// IPv6 path uses the same SIOCSIFADDR family of syscalls as IPv4, so
// the same kernel delete-then-add behavior applies and the same gate
// is required.
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

// TestPlatformConfigureOS_ReappliesAfterDeconfigure asserts that
// calling platformConfigureOS with an empty osConfig (the
// deconfigureOS path) resets the cached state so a subsequent
// populated call re-applies. The test reuses the same
// *platformOSConfigState pointer across all three calls; otherwise it
// would assert nothing about state caching.
//
// The reset-on-deconfigure invariant lets a hypothetical future
// refactor reuse the same osConfigState across a clean shutdown
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

	// First populated call.
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.Equal(t, []recordedCmd{ifconfigCmd}, recorder.cmds)

	// Deconfigure call (empty config). The empty-string guards on
	// tunIPv4/tunIPv6 leave the ifconfig and route blocks unentered,
	// so this records zero new commands.
	require.NoError(t, platformConfigureOS(t.Context(), &osConfig{}, state))
	require.Equal(t, []recordedCmd{ifconfigCmd}, recorder.cmds,
		"deconfigure call must not run any commands")

	// Re-populated call after deconfigure: state must have been reset,
	// so this records the same command as the first call.
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.Equal(t, []recordedCmd{ifconfigCmd, ifconfigCmd}, recorder.cmds,
		"post-deconfigure call must re-apply the original ifconfig")
}

// TestPlatformConfigureOS_PropagatesRunCommandError asserts that an
// error from runCommand bubbles out via trace.Wrap and that the
// corresponding state flag stays unset, so the next call retries the
// same command. This pins the "do not cache failed state" invariant
// against future refactors that move the state assignment ahead of
// the runCommand call.
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

	// Restore the recorder so the retry below records cleanly.
	runCommand = recorder.run

	// Retry: runCommand is now the recorder again, no error, so the
	// configurator should re-attempt the ifconfig and succeed.
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.True(t, state.configuredIPv4,
		"successful retry must mark state configured")
	require.Len(t, recorder.cmds, 2,
		"failing call plus successful retry must record two ifconfig invocations")
}

// TestPlatformConfigureOS_FullConfigSkipsOnSecondCall exercises the
// production shape: IPv4, IPv6, and one or more CIDR ranges in a
// single call. Asserts the four-command sequence runs in production
// order on the first call and that a second call with the same cfg
// records zero new commands.
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
