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

package systemd

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
)

func newBintestMock(t *testing.T, name string) *bintest.Mock {
	t.Helper()

	mock, err := bintest.NewMock(name)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.Close())
	})

	return mock
}

func expectJournalctlCall(journalctlMock *bintest.Mock, unit, invocationID string, lines int, stdoutOutput, stderrOutput string) {
	args := buildJournalctlArgs(unit, invocationID, lines)
	callArgs := make([]any, 0, len(args))
	for _, arg := range args {
		callArgs = append(callArgs, arg)
	}
	call := journalctlMock.Expect(callArgs...)
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

type mockConn struct {
	expectedUnit      string
	unitProperties    map[string]*systemddbus.Property
	serviceProperties map[string]*systemddbus.Property
	unitErrors        map[string]error
	serviceErrors     map[string]error
	closed            bool
}

func (m *mockConn) GetUnitPropertyContext(_ context.Context, unit, propertyName string) (*systemddbus.Property, error) {
	if m.expectedUnit != "" && unit != m.expectedUnit {
		return nil, fmt.Errorf("unexpected unit %q", unit)
	}
	if err := m.unitErrors[propertyName]; err != nil {
		return nil, err
	}
	return m.unitProperties[propertyName], nil
}

func (m *mockConn) GetServicePropertyContext(_ context.Context, unit, propertyName string) (*systemddbus.Property, error) {
	if m.expectedUnit != "" && unit != m.expectedUnit {
		return nil, fmt.Errorf("unexpected unit %q", unit)
	}
	if err := m.serviceErrors[propertyName]; err != nil {
		return nil, err
	}
	return m.serviceProperties[propertyName], nil
}

func (m *mockConn) Close() {
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
	conn := &mockConn{
		expectedUnit: "teleport.service",
		unitProperties: map[string]*systemddbus.Property{
			"InvocationID": newDBusProperty("InvocationID", invocationBytes),
		},
	}
	journalctlMock := newBintestMock(t, "journalctl")
	expectJournalctlCall(journalctlMock, "teleport.service", invocationID, 50, "--unit teleport.service --no-pager --lines 50 _SYSTEMD_INVOCATION_ID="+invocationID, "")

	client := NewClient(Config{
		Logger:         slog.Default(),
		JournalctlPath: journalctlMock.Path,
		NewConn: func(context.Context) (Conn, error) {
			return conn, nil
		},
	})

	got, err := client.CaptureJournal(t.Context(), "teleport.service", JournalOptions{
		Lines:                   50,
		FilterCurrentInvocation: true,
	})
	require.NoError(t, err)
	require.Equal(t, "--unit teleport.service --no-pager --lines 50 _SYSTEMD_INVOCATION_ID="+invocationID, got)
	require.True(t, conn.closed)
	require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
}

func TestCaptureJournalFallsBackWithoutInvocationID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		unitProperties map[string]*systemddbus.Property
	}{
		{
			name: "wrong type",
			unitProperties: map[string]*systemddbus.Property{
				"InvocationID": newDBusProperty("InvocationID", "active"),
			},
		},
		{
			name:           "nil prop",
			unitProperties: nil,
		},
		{
			name: "empty bytes",
			unitProperties: map[string]*systemddbus.Property{
				"InvocationID": newDBusProperty("InvocationID", []byte{}),
			},
		},
		{
			name: "invalid length",
			unitProperties: map[string]*systemddbus.Property{
				"InvocationID": newDBusProperty("InvocationID", []byte{1, 2, 3}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockConn{
				expectedUnit:   "teleport.service",
				unitProperties: tt.unitProperties,
			}
			journalctlMock := newBintestMock(t, "journalctl")
			expectJournalctlCall(journalctlMock, "teleport.service", "", 50, "--unit teleport.service --no-pager --lines 50", "")

			client := NewClient(Config{
				Logger:         slog.Default(),
				JournalctlPath: journalctlMock.Path,
				NewConn: func(context.Context) (Conn, error) {
					return conn, nil
				},
			})

			got, err := client.CaptureJournal(t.Context(), "teleport.service", JournalOptions{
				Lines:                   50,
				FilterCurrentInvocation: true,
			})
			require.NoError(t, err)
			require.Equal(t, "--unit teleport.service --no-pager --lines 50", got)
			require.True(t, conn.closed)
			require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
		})
	}
}

func TestCaptureJournalReturnsStdoutOnNonZeroExit(t *testing.T) {
	t.Parallel()

	journalctlMock := newBintestMock(t, "journalctl")
	call := journalctlMock.Expect("--unit", "teleport.service", "--no-pager", "--lines", "50")
	call.AndCallFunc(func(c *bintest.Call) {
		fmt.Fprintln(c.Stdout, "recent log line")
		fmt.Fprintln(c.Stderr, "journalctl warning")
		c.Exit(1)
	})

	client := NewClient(Config{
		Logger:         slog.Default(),
		JournalctlPath: journalctlMock.Path,
	})

	got, err := client.CaptureJournal(t.Context(), "teleport.service", JournalOptions{Lines: 50})
	require.NoError(t, err)
	require.Equal(t, "recent log line", got)
	require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
}

func TestCaptureJournalReturnsErrorWhenJournalctlCannotStart(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{
		Logger:         slog.Default(),
		JournalctlPath: "/nonexistent/journalctl",
	})

	got, err := client.CaptureJournal(t.Context(), "teleport.service", JournalOptions{Lines: 50})
	require.Empty(t, got)
	require.Error(t, err)
}

func TestCaptureJournalPropagatesContextCancellation(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{
		Logger: slog.Default(),
		NewConn: func(ctx context.Context) (Conn, error) {
			return nil, ctx.Err()
		},
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	got, err := client.CaptureJournal(ctx, "teleport.service", JournalOptions{FilterCurrentInvocation: true})
	require.Empty(t, got)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestReadServiceStateUsesDBusProperties(t *testing.T) {
	t.Parallel()

	conn := &mockConn{
		expectedUnit: "teleport.service",
		unitProperties: map[string]*systemddbus.Property{
			"ActiveState": newDBusProperty("ActiveState", "failed"),
			"SubState":    newDBusProperty("SubState", "exited"),
		},
		serviceProperties: map[string]*systemddbus.Property{
			"Result": newDBusProperty("Result", "exit-code"),
		},
	}

	client := NewClient(Config{
		Logger: slog.Default(),
		NewConn: func(context.Context) (Conn, error) {
			return conn, nil
		},
	})

	state, err := client.ReadServiceState(t.Context(), "teleport.service")
	require.NoError(t, err)
	require.Equal(t, `systemd service state: ActiveState="failed", SubState="exited", Result="exit-code"`, state.String())
	require.True(t, conn.closed)
}

func TestReadServiceStateHandlesPartialDBusFailures(t *testing.T) {
	t.Parallel()

	conn := &mockConn{
		expectedUnit: "teleport.service",
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

	client := NewClient(Config{
		Logger: slog.Default(),
		NewConn: func(context.Context) (Conn, error) {
			return conn, nil
		},
	})

	state, err := client.ReadServiceState(t.Context(), "teleport.service")
	require.NoError(t, err)
	require.Equal(t, `systemd service state: ActiveState="failed", SubState="unknown", Result="exit-code"`, state.String())
	require.True(t, conn.closed)
}

func TestReadServiceStateReturnsUnknownsWhenAllPropertiesFail(t *testing.T) {
	t.Parallel()

	conn := &mockConn{
		expectedUnit: "teleport.service",
		unitErrors: map[string]error{
			"ActiveState": assert.AnError,
			"SubState":    assert.AnError,
		},
		serviceErrors: map[string]error{
			"Result": assert.AnError,
		},
	}

	client := NewClient(Config{
		Logger: slog.Default(),
		NewConn: func(context.Context) (Conn, error) {
			return conn, nil
		},
	})

	state, err := client.ReadServiceState(t.Context(), "teleport.service")
	require.NoError(t, err)
	require.Equal(t, `systemd service state: ActiveState="unknown", SubState="unknown", Result="unknown"`, state.String())
	require.True(t, conn.closed)
}

func TestReadServiceStateReturnsErrorWhenDBusUnavailable(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{
		Logger: slog.Default(),
		NewConn: func(context.Context) (Conn, error) {
			return nil, assert.AnError
		},
	})

	_, err := client.ReadServiceState(t.Context(), "teleport.service")
	require.Error(t, err)
}
