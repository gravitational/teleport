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

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/gravitational/trace"
)

// JoinFailureError is returned when the Teleport agent is installed but fails to join
// the cluster. It carries structured diagnostics so callers can extract individual fields via [errors.As].
type JoinFailureError struct {
	// Message describes what went wrong at a high level, e.g.
	// "node did not become ready (join cluster) within 5m0s".
	Message string
	// ServiceDiagnostics contains the systemd unit snapshot (ActiveState, SubState, Result).
	ServiceDiagnostics string
	// JournalOutput contains recent journalctl lines for the Teleport service.
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

type systemdPropertyGetter func(context.Context, string, string) (*systemddbus.Property, error)

// openSystemdConn opens a systemd D-Bus connection. When newSystemdConn is set
// (by tests), it uses that override instead of the real D-Bus.
func (a *AutoDiscoverNodeInstaller) openSystemdConn(ctx context.Context) (dbusConn, error) {
	if a.newSystemdConn != nil {
		return a.newSystemdConn(ctx)
	}

	return systemddbus.NewWithContext(ctx)
}

// getDBusStringProperty returns a non-empty string property value from systemd.
// It returns ok=false when the property cannot be retrieved, is missing, is not
// a string, or is empty. All failure modes are logged at debug level.
func (a *AutoDiscoverNodeInstaller) getDBusStringProperty(ctx context.Context, getter systemdPropertyGetter, serviceName, propertyName string) (string, bool) {
	prop, err := getter(ctx, serviceName, propertyName)
	if err != nil {
		a.Logger.DebugContext(ctx, "Could not retrieve systemd property",
			"service", serviceName,
			"property", propertyName,
			"error", err,
		)
		return "", false
	}
	if prop == nil {
		a.Logger.DebugContext(ctx, "Systemd property lookup returned no value",
			"service", serviceName,
			"property", propertyName,
		)
		return "", false
	}

	raw := prop.Value.Value()
	value, ok := raw.(string)
	if !ok {
		a.Logger.DebugContext(ctx, "Ignoring non-string systemd property",
			"service", serviceName,
			"property", propertyName,
			"value_type", fmt.Sprintf("%T", raw),
		)
		return "", false
	}
	if value == "" {
		a.Logger.DebugContext(ctx, "Ignoring empty systemd property",
			"service", serviceName,
			"property", propertyName,
		)
		return "", false
	}

	return value, true
}

// gatherServiceDiagnostics returns a one-shot best-effort systemd snapshot (ActiveState,
// SubState, and Result) for join-failure diagnostics. It never returns an error.
func (a *AutoDiscoverNodeInstaller) gatherServiceDiagnostics(ctx context.Context, serviceName string) string {
	conn, err := a.openSystemdConn(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return defaultServiceDiagnosticsUnavailable
		}

		a.Logger.DebugContext(ctx, "Could not connect to systemd D-Bus while gathering service diagnostics",
			"service", serviceName,
			"error", err,
		)
		return defaultServiceDiagnosticsUnavailable
	}
	defer conn.Close()

	diagnostics := map[string]string{
		"ActiveState": "unknown",
		"SubState":    "unknown",
		"Result":      "unknown",
	}
	if value, ok := a.getDBusStringProperty(ctx, conn.GetUnitPropertyContext, serviceName, "ActiveState"); ok {
		diagnostics["ActiveState"] = value
	}
	if value, ok := a.getDBusStringProperty(ctx, conn.GetUnitPropertyContext, serviceName, "SubState"); ok {
		diagnostics["SubState"] = value
	}
	if value, ok := a.getDBusStringProperty(ctx, conn.GetServicePropertyContext, serviceName, "Result"); ok {
		diagnostics["Result"] = value
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

// getServiceInvocationID retrieves a validated systemd InvocationID for serviceName.
// It returns ("", nil) when the ID is unavailable, invalid, or cannot be retrieved.
// It returns a non-nil error only if ctx is canceled or expires.
func (a *AutoDiscoverNodeInstaller) getServiceInvocationID(ctx context.Context, serviceName string) (string, error) {
	conn, err := a.openSystemdConn(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", trace.Wrap(ctxErr)
		}

		a.Logger.DebugContext(ctx, "Could not connect to systemd D-Bus while retrieving service invocation ID",
			"service", serviceName,
			"error", err,
		)
		return "", nil
	}
	defer conn.Close()

	prop, err := conn.GetUnitPropertyContext(ctx, serviceName, "InvocationID")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", trace.Wrap(ctxErr)
		}

		a.Logger.DebugContext(ctx, "Could not retrieve service invocation ID",
			"service", serviceName,
			"error", err,
		)
		return "", nil
	}
	if prop == nil {
		a.Logger.DebugContext(ctx, "Service invocation ID lookup returned no value",
			"service", serviceName,
		)
		return "", nil
	}

	var invocationID string
	switch value := prop.Value.Value().(type) {
	case []byte:
		if len(value) == 0 {
			a.Logger.DebugContext(ctx, "Ignoring empty service invocation ID",
				"service", serviceName,
			)
			return "", nil
		}
		invocationID = hex.EncodeToString(value)
	case string:
		if value == "" || strings.EqualFold(value, "n/a") {
			return "", nil
		}
		invocationID = value
	default:
		a.Logger.DebugContext(ctx, "Ignoring invalid service invocation ID type",
			"service", serviceName,
			"value_type", fmt.Sprintf("%T", value),
		)
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
