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
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/buildkite/bintest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/packagemanager"
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

func expectSystemctlShowInvocationID(systemctlMock *bintest.Mock, serviceName, invocationID, stderr string) {
	call := systemctlMock.Expect("show", serviceName, "--property", "InvocationID", "--value")
	if invocationID == "" && stderr == "" {
		return
	}

	call.AndCallFunc(func(c *bintest.Call) {
		if stderr != "" {
			fmt.Fprintln(c.Stderr, stderr)
		}
		if invocationID != "" {
			fmt.Fprintln(c.Stdout, invocationID)
		}
		c.Exit(0)
	})
}

func expectSystemctlShowServiceDiagnostics(systemctlMock *bintest.Mock, serviceName, activeState, subState, result string) {
	call := systemctlMock.Expect("show", serviceName, "--property", "ActiveState", "--property", "SubState", "--property", "Result")
	call.AndCallFunc(func(c *bintest.Call) {
		fmt.Fprintf(c.Stdout, "ActiveState=%s\nSubState=%s\nResult=%s\n", activeState, subState, result)
		c.Exit(0)
	})
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

func TestCaptureJournalFiltersByInvocationID(t *testing.T) {
	t.Parallel()

	invocationID := "0123456789abcdef0123456789abcdef"
	systemctlMock := newBintestMock(t, "systemctl")
	journalctlMock := newBintestMock(t, "journalctl")
	expectSystemctlShowInvocationID(systemctlMock, "teleport", invocationID, "systemctl warning")
	expectJournalctlCall(journalctlMock, "teleport", invocationID, "--unit teleport --no-pager --lines 50 _SYSTEMD_INVOCATION_ID="+invocationID, "")

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			binariesLocation: packagemanager.BinariesLocation{
				Systemctl:  systemctlMock.Path,
				Journalctl: journalctlMock.Path,
			},
		},
	}

	got, err := installer.captureJournal(context.Background(), "teleport")
	require.NoError(t, err)
	require.Contains(t, got, "_SYSTEMD_INVOCATION_ID="+invocationID)
	require.Contains(t, got, "--unit teleport")
	require.True(t, systemctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "systemctl")
	require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
}

func TestCaptureJournalFallsBackWithoutInvocationID(t *testing.T) {
	t.Parallel()

	systemctlMock := newBintestMock(t, "systemctl")
	journalctlMock := newBintestMock(t, "journalctl")
	expectSystemctlShowInvocationID(systemctlMock, "teleport", "active", "")
	expectJournalctlCall(journalctlMock, "teleport", "", "--unit teleport --no-pager --lines 50", "")

	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			binariesLocation: packagemanager.BinariesLocation{
				Systemctl:  systemctlMock.Path,
				Journalctl: journalctlMock.Path,
			},
		},
	}

	got, err := installer.captureJournal(context.Background(), "teleport")
	require.NoError(t, err)
	require.NotContains(t, got, "_SYSTEMD_INVOCATION_ID=")
	require.Contains(t, got, "--unit teleport")
	require.True(t, systemctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "systemctl")
	require.True(t, journalctlMock.Check(t), "mismatch between expected invocations and actual calls for %q", "journalctl")
}

func TestCaptureJournalPropagatesContextCancellation(t *testing.T) {
	t.Parallel()

	mockDir := t.TempDir()
	installer := &AutoDiscoverNodeInstaller{
		AutoDiscoverNodeInstallerConfig: &AutoDiscoverNodeInstallerConfig{
			Logger: slog.Default(),
			binariesLocation: packagemanager.BinariesLocation{
				Systemctl:  filepath.Join(mockDir, "missing-systemctl"),
				Journalctl: filepath.Join(mockDir, "missing-journalctl"),
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := installer.captureJournal(ctx, "teleport")
	require.Empty(t, got)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}
