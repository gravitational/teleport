/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package installer

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"testing"

	"github.com/buildkite/bintest/v3"
	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/packagemanager"
)

func TestJoinFailureErrorString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *JoinFailureError
		contains []string
	}{
		{
			name: "all fields populated",
			err: &JoinFailureError{
				Message:            "node did not become ready (join cluster) within 10s",
				ServiceDiagnostics: `systemd service state: ActiveState="failed", SubState="exited", Result="exit-code"`,
				JournalOutput:      "error: token expired",
			},
			contains: []string{
				"node did not become ready (join cluster) within 10s",
				`ActiveState="failed"`,
				"Journal output:\nerror: token expired",
				"agent failed to join the cluster",
			},
		},
		{
			name: "no journal output",
			err: &JoinFailureError{
				Message:            "node did not become ready (join cluster) within 5m0s",
				ServiceDiagnostics: "systemd service state: unavailable",
			},
			contains: []string{
				"node did not become ready (join cluster) within 5m0s",
				"systemd service state: unavailable",
				"agent failed to join the cluster",
			},
		},
		{
			name: "message only",
			err: &JoinFailureError{
				Message: "node did not become ready (join cluster) within 10s",
			},
			contains: []string{
				"node did not become ready (join cluster) within 10s",
				"agent failed to join the cluster",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.contains {
				require.Contains(t, got, want)
			}
		})
	}
}

func newBintestMock(t *testing.T, name string) *bintest.Mock {
	t.Helper()

	mock, err := bintest.NewMock(name)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.Close())
	})

	return mock
}

func expectJournalctlCall(journalctlMock *bintest.Mock, serviceName, invocationID, stdoutOutput, stderrOutput string) {
	args := buildJournalctlArgs(serviceName, invocationID)
	callArgs := make([]any, 0, len(args))
	for _, arg := range args {
		callArgs = append(callArgs, arg)
	}
	call := journalctlMock.Expect(callArgs...)
	if stdoutOutput == "" && stderrOutput == "" {
		return
	}

	call.AndCallFunc(func(c *bintest.Call) {
		if stdoutOutput != "" {
			fmt.Fprintln(c.Stdout, stdoutOutput)
		}
		if stderrOutput != "" {
			fmt.Fprintln(c.Stderr, stderrOutput)
		}
		c.Exit(0)
	})
}

type mockDBusConn struct {
	expectedServiceName string
	unitProperties      map[string]*systemddbus.Property
	serviceProperties   map[string]*systemddbus.Property
	unitErrors          map[string]error
	serviceErrors       map[string]error
	closed              bool
}

func (m *mockDBusConn) GetUnitPropertyContext(_ context.Context, serviceName, propertyName string) (*systemddbus.Property, error) {
	if m.expectedServiceName != "" {
		if serviceName != m.expectedServiceName {
			return nil, fmt.Errorf("unexpected service name %q", serviceName)
		}
	}
	if err := m.unitErrors[propertyName]; err != nil {
		return nil, err
	}
	return m.unitProperties[propertyName], nil
}

func (m *mockDBusConn) GetServicePropertyContext(_ context.Context, serviceName, propertyName string) (*systemddbus.Property, error) {
	if m.expectedServiceName != "" {
		if serviceName != m.expectedServiceName {
			return nil, fmt.Errorf("unexpected service name %q", serviceName)
		}
	}
	if err := m.serviceErrors[propertyName]; err != nil {
		return nil, err
	}
	return m.serviceProperties[propertyName], nil
}

func (m *mockDBusConn) Close() {
	m.closed = true
}

func newDBusProperty(name string, value any) *systemddbus.Property {
	return &systemddbus.Property{Name: name, Value: godbus.MakeVariant(value)}
}

func TestCaptureJournalFiltersByInvocationID(t *testing.T) {
	t.Parallel()

	invocationID := "0123456789abcdef0123456789abcdef"
	invocationBytes, err := hex.DecodeString(invocationID)
	require.NoError(t, err)
	mockConn := &mockDBusConn{
		expectedServiceName: "teleport.service",
		unitProperties: map[string]*systemddbus.Property{
			"InvocationID": newDBusProperty("InvocationID", invocationBytes),
		},
	}
	journalctlMock := newBintestMock(t, "journalctl")
	expectJournalctlCall(journalctlMock, "teleport.service", invocationID, "--unit teleport.service --no-pager --lines 50 _SYSTEMD_INVOCATION_ID="+invocationID, "")

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			binariesLocation: packagemanager.BinariesLocation{
				Journalctl: journalctlMock.Path,
			},
			newSystemdConn: func(context.Context) (dbusConn, error) {
				return mockConn, nil
			},
		},
	}

	got, err := installer.captureJournal(context.Background(), "teleport.service")
	require.NoError(t, err)
	require.Contains(t, got, "_SYSTEMD_INVOCATION_ID="+invocationID)
	require.Contains(t, got, "--unit teleport.service")
	require.True(t, mockConn.closed)
	require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
}

func TestCaptureJournalFallsBackWithoutInvocationID(t *testing.T) {
	t.Parallel()

	mockConn := &mockDBusConn{
		expectedServiceName: "teleport.service",
		unitProperties: map[string]*systemddbus.Property{
			"InvocationID": newDBusProperty("InvocationID", "active"),
		},
	}
	journalctlMock := newBintestMock(t, "journalctl")
	expectJournalctlCall(journalctlMock, "teleport.service", "", "--unit teleport.service --no-pager --lines 50", "")

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			binariesLocation: packagemanager.BinariesLocation{
				Journalctl: journalctlMock.Path,
			},
			newSystemdConn: func(context.Context) (dbusConn, error) {
				return mockConn, nil
			},
		},
	}

	got, err := installer.captureJournal(context.Background(), "teleport.service")
	require.NoError(t, err)
	require.NotContains(t, got, "_SYSTEMD_INVOCATION_ID=")
	require.Contains(t, got, "--unit teleport.service")
	require.True(t, mockConn.closed)
	require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
}

func TestCaptureJournalFallsBackWithoutInvocationIDOnNilProp(t *testing.T) {
	t.Parallel()

	mockConn := &mockDBusConn{expectedServiceName: "teleport.service"}
	journalctlMock := newBintestMock(t, "journalctl")
	expectJournalctlCall(journalctlMock, "teleport.service", "", "--unit teleport.service --no-pager --lines 50", "")

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			binariesLocation: packagemanager.BinariesLocation{
				Journalctl: journalctlMock.Path,
			},
			newSystemdConn: func(context.Context) (dbusConn, error) {
				return mockConn, nil
			},
		},
	}

	got, err := installer.captureJournal(context.Background(), "teleport.service")
	require.NoError(t, err)
	require.NotContains(t, got, "_SYSTEMD_INVOCATION_ID=")
	require.Contains(t, got, "--unit teleport.service")
	require.True(t, mockConn.closed)
	require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
}

func TestCaptureJournalFallsBackWithoutInvocationIDOnEmptyBytes(t *testing.T) {
	t.Parallel()

	mockConn := &mockDBusConn{
		expectedServiceName: "teleport.service",
		unitProperties: map[string]*systemddbus.Property{
			"InvocationID": newDBusProperty("InvocationID", []byte{}),
		},
	}
	journalctlMock := newBintestMock(t, "journalctl")
	expectJournalctlCall(journalctlMock, "teleport.service", "", "--unit teleport.service --no-pager --lines 50", "")

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			binariesLocation: packagemanager.BinariesLocation{
				Journalctl: journalctlMock.Path,
			},
			newSystemdConn: func(context.Context) (dbusConn, error) {
				return mockConn, nil
			},
		},
	}

	got, err := installer.captureJournal(context.Background(), "teleport.service")
	require.NoError(t, err)
	require.NotContains(t, got, "_SYSTEMD_INVOCATION_ID=")
	require.Contains(t, got, "--unit teleport.service")
	require.True(t, mockConn.closed)
	require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
}

func TestCaptureJournalPropagatesContextCancellation(t *testing.T) {
	t.Parallel()

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			newSystemdConn: func(ctx context.Context) (dbusConn, error) {
				return nil, ctx.Err()
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := installer.captureJournal(ctx, "teleport.service")
	require.Empty(t, got)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestGatherServiceDiagnosticsUsesDBusProperties(t *testing.T) {
	t.Parallel()

	mockConn := &mockDBusConn{
		expectedServiceName: "teleport.service",
		unitProperties: map[string]*systemddbus.Property{
			"ActiveState": newDBusProperty("ActiveState", "failed"),
			"SubState":    newDBusProperty("SubState", "exited"),
		},
		serviceProperties: map[string]*systemddbus.Property{
			"Result": newDBusProperty("Result", "exit-code"),
		},
	}

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			newSystemdConn: func(context.Context) (dbusConn, error) {
				return mockConn, nil
			},
		},
	}

	got := installer.gatherServiceDiagnostics(context.Background(), "teleport.service")
	require.Equal(t, `systemd service state: ActiveState="failed", SubState="exited", Result="exit-code"`, got)
	require.True(t, mockConn.closed)
}

func TestGatherServiceDiagnosticsHandlesPartialDBusFailures(t *testing.T) {
	t.Parallel()

	mockConn := &mockDBusConn{
		expectedServiceName: "teleport.service",
		unitProperties: map[string]*systemddbus.Property{
			"ActiveState": newDBusProperty("ActiveState", "failed"),
		},
		serviceProperties: map[string]*systemddbus.Property{
			"Result": newDBusProperty("Result", "exit-code"),
		},
		unitErrors: map[string]error{
			"SubState": assert.AnError,
		},
	}

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			newSystemdConn: func(context.Context) (dbusConn, error) {
				return mockConn, nil
			},
		},
	}

	got := installer.gatherServiceDiagnostics(context.Background(), "teleport.service")
	require.Equal(t, `systemd service state: ActiveState="failed", SubState="unknown", Result="exit-code"`, got)
	require.True(t, mockConn.closed)
}

func TestGatherServiceDiagnosticsFallsBackWhenDBusUnavailable(t *testing.T) {
	t.Parallel()

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			newSystemdConn: func(context.Context) (dbusConn, error) {
				return nil, assert.AnError
			},
		},
	}

	got := installer.gatherServiceDiagnostics(context.Background(), "teleport.service")
	require.Equal(t, defaultServiceDiagnosticsUnavailable, got)
}
