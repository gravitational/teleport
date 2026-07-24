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
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	require.True(t, state.configuredIPv6,
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

func dnsTestOSConfig() *osConfig {
	return &osConfig{
		tunName:  "utun4",
		tunIPv4:  "100.64.0.1",
		dnsAddrs: []string{"100.64.0.2", "fdd4:a23:da97::2"},
		dnsZones: []string{"example.com", "leaf.example.com"},
	}
}

func resolverFileForZone(zone string) string {
	return filepath.Join(resolverPath, zone)
}

func requireResolverFiles(t *testing.T, nameservers []string, zones ...string) {
	t.Helper()
	want := string(resolverFileContents(nameservers))
	entries, err := os.ReadDir(resolverPath)
	require.NoError(t, err)
	require.Len(t, entries, len(zones))
	for _, zone := range zones {
		contents, err := os.ReadFile(resolverFileForZone(zone))
		require.NoError(t, err)
		require.Equal(t, want, string(contents))
	}
}

func TestConfigureDNSWritesResolverFiles(t *testing.T) {
	setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := dnsTestOSConfig()

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	requireResolverFiles(t, cfg.dnsAddrs, "example.com", "leaf.example.com")
}

func TestConfigureDNSSkipsRewriteWhenUnchanged(t *testing.T) {
	setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := dnsTestOSConfig()

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	// Backdate the files; if the second call rewrote them the mtime
	// would jump back to ~now.
	past := time.Now().Add(-time.Hour)
	for _, zone := range cfg.dnsZones {
		require.NoError(t, os.Chtimes(resolverFileForZone(zone), past, past))
	}

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	for _, zone := range cfg.dnsZones {
		info, err := os.Stat(resolverFileForZone(zone))
		require.NoError(t, err)
		require.True(t, info.ModTime().Equal(past),
			"second call with unchanged config must not rewrite %s", zone)
	}
}

func TestConfigureDNSRestoresDeletedResolverFile(t *testing.T) {
	setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := dnsTestOSConfig()

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.NoError(t, os.Remove(resolverFileForZone("example.com")))
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	requireResolverFiles(t, cfg.dnsAddrs, "example.com", "leaf.example.com")
}

func TestConfigureDNSRestoresModifiedResolverFile(t *testing.T) {
	setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := dnsTestOSConfig()

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	// Tamper without changing the file size to make sure drift detection
	// doesn't rely on cheap size checks.
	tampered := bytes.ToUpper(resolverFileContents(cfg.dnsAddrs))
	require.NoError(t, os.WriteFile(
		resolverFileForZone("example.com"), tampered, 0644))
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	requireResolverFiles(t, cfg.dnsAddrs, "example.com", "leaf.example.com")
}

func TestConfigureDNSReappliesOnZoneChange(t *testing.T) {
	setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := dnsTestOSConfig()

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	cfg.dnsZones = []string{"example.com", "other.example.org"}
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))

	requireResolverFiles(t, cfg.dnsAddrs, "example.com", "other.example.org")
}

func TestConfigureDNSDeconfigureRemovesResolverFiles(t *testing.T) {
	setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := dnsTestOSConfig()

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.NoError(t, platformConfigureOS(t.Context(), &osConfig{}, state))

	entries, err := os.ReadDir(resolverPath)
	require.NoError(t, err)
	require.Empty(t, entries, "deconfigure must remove all managed resolver files")
}

func TestConfigureDNSLeavesUnmanagedFilesAlone(t *testing.T) {
	setupOSConfigTest(t)
	state := &platformOSConfigState{}
	cfg := dnsTestOSConfig()

	unmanaged := filepath.Join(resolverPath, "corp.internal")
	require.NoError(t, os.WriteFile(unmanaged, []byte("nameserver 10.0.0.53\n"), 0644))

	require.NoError(t, platformConfigureOS(t.Context(), cfg, state))
	require.NoError(t, platformConfigureOS(t.Context(), &osConfig{}, state))

	contents, err := os.ReadFile(unmanaged)
	require.NoError(t, err)
	require.Equal(t, "nameserver 10.0.0.53\n", string(contents))
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
