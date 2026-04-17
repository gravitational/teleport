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
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

// JoinFailureError is returned when the Teleport agent is installed but fails to join
// the cluster. It carries structured diagnostics so callers can extract individual fields via [errors.As].
type JoinFailureError struct {
	// Message describes what went wrong at a high level.
	Message string
	// ServiceDiagnostics contains the systemd unit snapshot (ActiveState, SubState, Result).
	ServiceDiagnostics string
	// JournalOutput contains recent journalctl lines for the Teleport service,
	// or an explanatory note when logs could not be collected.
	JournalOutput string
	// LastError is the last unexpected error from the readyz endpoint, if any.
	// It surfaces errors that would otherwise be lost when the poll loop times out.
	LastError string
}

// Error returns the formatted join-failure diagnostics.
func (e *JoinFailureError) Error() string {
	parts := []string{e.Message}
	if e.ServiceDiagnostics != "" {
		parts = append(parts, e.ServiceDiagnostics)
	}
	if e.LastError != "" {
		parts = append(parts, "last readyz error: "+e.LastError)
	}
	if e.JournalOutput != "" {
		parts = append(parts, "\nJournal output:\n"+e.JournalOutput)
	}
	return strings.Join(parts, "; ") + ": agent failed to join the cluster"
}

const (
	// maxJournalLines is the number of recent journalctl lines to capture.
	maxJournalLines = 100

	// defaultServiceDiagnosticsUnavailable is appended when systemd state cannot
	// be retrieved while preparing a join-failure error.
	defaultServiceDiagnosticsUnavailable = "systemd service state: unavailable"
)

// enableAndRestartTeleportService will enable and (re)start the configured Teleport service.
// This function must be idempotent because we can call it in either one of the following scenarios:
// - teleport was just installed and teleport.service is inactive
// - teleport was already installed but the service is failing
func (ani *AutoDiscoverNodeInstaller) enableAndRestartTeleportService(ctx context.Context) error {
	serviceName := ani.buildTeleportSystemdUnitName()

	if err := ani.runSystemctlCommand(ctx, "enable", serviceName); err != nil {
		return trace.Wrap(err)
	}

	if err := ani.runSystemctlCommand(ctx, "restart", serviceName); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *AutoDiscoverNodeInstaller) runSystemctlCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, a.binariesLocation.Systemctl, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err != nil {
		// Prefer the context error when present so callers can distinguish user- or
		// timeout-driven cancellation from systemctl execution failures.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return trace.Wrap(ctxErr)
		}

		stdout := strings.TrimSpace(stdoutBuf.String())
		stderr := strings.TrimSpace(stderrBuf.String())

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// systemctl uses non-zero exits for command failures after launch, so include
			// the exit code and captured streams to preserve the observed service state.
			return trace.Wrap(err,
				"%q exited with code %d (stdout: %s, stderr: %s)",
				cmd,
				exitErr.ExitCode(),
				stdout,
				stderr,
			)
		}

		return trace.Wrap(err,
			"failed to execute %q (stdout: %s, stderr: %s)",
			cmd,
			stdout,
			stderr,
		)
	}

	return nil
}
