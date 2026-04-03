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
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

const (
	// maxJournalLines is the number of recent journalctl lines to capture.
	maxJournalLines = 50

	// defaultServiceDiagnosticsUnavailable is appended when systemd state cannot
	// be retrieved while preparing a join-failure error.
	defaultServiceDiagnosticsUnavailable = "systemd service state: unavailable"
)

// ErrJoinFailure is returned when the Teleport agent is installed but fails to join the cluster.
var ErrJoinFailure = errors.New("agent failed to join the cluster")

// gatherServiceDiagnostics returns a one-shot best-effort systemd snapshot (ActiveState,
// SubState, and Result) for join-failure diagnostics. It never returns an error.
func (a *AutoDiscoverNodeInstaller) gatherServiceDiagnostics(ctx context.Context, serviceName string) string {
	cmd := exec.CommandContext(ctx, a.binariesLocation.Systemctl,
		"show", serviceName,
		"--property", "ActiveState",
		"--property", "SubState",
		"--property", "Result",
	)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return defaultServiceDiagnosticsUnavailable
		}

		stderr := strings.TrimSpace(stderrBuf.String())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// systemctl show exited non-zero (e.g. unknown service); stdout
			// may still contain partial output, so fall through to parse it.
			a.Logger.DebugContext(ctx, "systemctl show exited non-zero while gathering service diagnostics",
				"service", serviceName, "exit_code", exitErr.ExitCode(), "stderr", stderr)
		} else {
			// Infrastructure error (binary not found, permission denied, etc.).
			a.Logger.DebugContext(ctx, "Could not gather service diagnostics",
				"service", serviceName, "error", err, "stderr", stderr)
			return defaultServiceDiagnosticsUnavailable
		}
	}

	diagnostics := map[string]string{
		"ActiveState": "unknown",
		"SubState":    "unknown",
		"Result":      "unknown",
	}
	for line := range strings.SplitSeq(strings.TrimSpace(stdoutBuf.String()), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}

		diagnostics[key] = value
	}

	return fmt.Sprintf("systemd service state: ActiveState=%q, SubState=%q, Result=%q", diagnostics["ActiveState"], diagnostics["SubState"], diagnostics["Result"])
}

func isSystemdInvocationID(value string) bool {
	value = strings.ReplaceAll(value, "-", "")
	if len(value) != 32 {
		return false
	}

	_, err := hex.DecodeString(value)
	return err == nil
}

func buildJournalctlArgs(serviceName, invocationID string) []string {
	args := []string{
		"--unit", serviceName,
		"--no-pager",
		"--lines", fmt.Sprintf("%d", maxJournalLines),
	}

	if invocationID != "" {
		args = append(args, "_SYSTEMD_INVOCATION_ID="+invocationID)
	}

	return args
}

func (a *AutoDiscoverNodeInstaller) getServiceInvocationID(ctx context.Context, serviceName string) (string, error) {
	cmd := exec.CommandContext(ctx, a.binariesLocation.Systemctl, "show", serviceName, "--property", "InvocationID", "--value")
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	stdout := strings.TrimSpace(stdoutBuf.String())
	stderr := strings.TrimSpace(stderrBuf.String())
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", trace.Wrap(ctxErr)
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			a.Logger.DebugContext(ctx, "Failed to retrieve service invocation ID (systemctl show exited non-zero)",
				"service", serviceName,
				"exit_code", exitErr.ExitCode(),
				"stdout", stdout,
				"stderr", stderr,
			)
			return "", nil
		}

		a.Logger.DebugContext(ctx, "Could not retrieve service invocation ID", "service", serviceName, "error", err, "stdout", stdout, "stderr", stderr)
		return "", nil
	}

	invocationID := stdout
	if invocationID == "" || strings.EqualFold(invocationID, "n/a") {
		return "", nil
	}

	if !isSystemdInvocationID(invocationID) {
		a.Logger.DebugContext(ctx, "Ignoring invalid service invocation ID", "service", serviceName, "invocation_id", invocationID)
		return "", nil
	}

	return invocationID, nil
}

// captureJournal is a best-effort helper that runs journalctl to retrieve recent log lines
// for the given systemd unit. Stderr is logged internally but not returned, to keep
// caller-facing diagnostics focused on journal contents.
func (a *AutoDiscoverNodeInstaller) captureJournal(ctx context.Context, serviceName string) (string, error) {
	invocationID, err := a.getServiceInvocationID(ctx, serviceName)
	if err != nil {
		return "", trace.Wrap(err)
	}
	args := buildJournalctlArgs(serviceName, invocationID)

	cmd := exec.CommandContext(ctx, a.binariesLocation.Journalctl, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stderrOutput := strings.TrimSpace(stderrBuf.String())
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", trace.Wrap(ctxErr)
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			a.Logger.DebugContext(ctx, "journalctl exited non-zero", "service", serviceName, "exit_code", exitErr.ExitCode(), "stderr", stderrOutput)
		} else {
			// Infrastructure error (binary not found, permission denied, etc.).
			// Log at Warn since this is unexpected, unlike a non-zero exit which
			// journalctl uses for normal conditions (e.g. no matching entries).
			a.Logger.WarnContext(ctx, "Failed to capture journal output", "service", serviceName, "error", err, "stderr", stderrOutput)
		}
	}

	return strings.TrimSpace(stdoutBuf.String()), nil
}

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
		if ctxErr := ctx.Err(); ctxErr != nil {
			return trace.Wrap(ctxErr)
		}

		stdout := strings.TrimSpace(stdoutBuf.String())
		stderr := strings.TrimSpace(stderrBuf.String())

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return trace.Wrap(err,
				"systemctl %s exited with code %d (stdout: %s, stderr: %s)",
				strings.Join(args, " "),
				exitErr.ExitCode(),
				stdout,
				stderr,
			)
		}

		return trace.Wrap(err,
			"failed to execute systemctl %s (stdout: %s, stderr: %s)",
			strings.Join(args, " "),
			stdout,
			stderr,
		)
	}

	return nil
}
