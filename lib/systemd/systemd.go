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
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/gravitational/trace"
)

// Conn is the subset of the systemd D-Bus API used by Teleport.
type Conn interface {
	GetUnitPropertyContext(context.Context, string, string) (*systemddbus.Property, error)
	GetServicePropertyContext(context.Context, string, string) (*systemddbus.Property, error)
	Close()
}

// NewConnFunc opens a new systemd D-Bus connection.
type NewConnFunc func(context.Context) (Conn, error)

var _ Conn = (*systemddbus.Conn)(nil)

// Config configures a systemd client.
type Config struct {
	Logger         *slog.Logger
	JournalctlPath string
	NewConn        NewConnFunc
}

// JournalOptions configures journal capture.
type JournalOptions struct {
	Lines                   int
	FilterCurrentInvocation bool
}

// ServiceState is a one-shot snapshot of a systemd unit state.
type ServiceState struct {
	ActiveState string
	SubState    string
	Result      string
}

// String formats the service state for diagnostics.
func (s ServiceState) String() string {
	return fmt.Sprintf("systemd service state: ActiveState=%q, SubState=%q, Result=%q", s.ActiveState, s.SubState, s.Result)
}

// Client retrieves systemd state and journal output.
type Client struct {
	log            *slog.Logger
	journalctlPath string
	newConn        NewConnFunc
}

// NewClient returns a new systemd client.
func NewClient(cfg Config) *Client {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.JournalctlPath == "" {
		cfg.JournalctlPath = "journalctl"
	}
	if cfg.NewConn == nil {
		cfg.NewConn = func(ctx context.Context) (Conn, error) {
			return systemddbus.NewWithContext(ctx)
		}
	}

	return &Client{
		log:            cfg.Logger,
		journalctlPath: cfg.JournalctlPath,
		newConn:        cfg.NewConn,
	}
}

type propertyGetter func(context.Context, string, string) (*systemddbus.Property, error)

const invocationIDBytes = 16

func (c *Client) getStringProperty(ctx context.Context, getter propertyGetter, unit, propertyName string) (string, bool) {
	prop, err := getter(ctx, unit, propertyName)
	if err != nil {
		c.log.DebugContext(ctx, "Could not retrieve systemd property",
			"service", unit,
			"property", propertyName,
			"error", err,
		)
		return "", false
	}
	if prop == nil {
		c.log.DebugContext(ctx, "Systemd property lookup returned no value",
			"service", unit,
			"property", propertyName,
		)
		return "", false
	}

	raw := prop.Value.Value()
	value, ok := raw.(string)
	if !ok {
		c.log.DebugContext(ctx, "Ignoring non-string systemd property",
			"service", unit,
			"property", propertyName,
			"value_type", fmt.Sprintf("%T", raw),
		)
		return "", false
	}
	if value == "" {
		c.log.DebugContext(ctx, "Ignoring empty systemd property",
			"service", unit,
			"property", propertyName,
		)
		return "", false
	}

	return value, true
}

// ReadServiceState returns a best-effort systemd state snapshot for a unit.
func (c *Client) ReadServiceState(ctx context.Context, unit string) (ServiceState, error) {
	conn, err := c.newConn(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ServiceState{}, trace.Wrap(ctx.Err())
		}

		c.log.DebugContext(ctx, "Could not connect to systemd D-Bus while gathering service diagnostics",
			"service", unit,
			"error", err,
		)
		return ServiceState{}, trace.Wrap(err)
	}
	defer conn.Close()

	state := ServiceState{
		ActiveState: "unknown",
		SubState:    "unknown",
		Result:      "unknown",
	}
	if value, ok := c.getStringProperty(ctx, conn.GetUnitPropertyContext, unit, "ActiveState"); ok {
		state.ActiveState = value
	}
	if value, ok := c.getStringProperty(ctx, conn.GetUnitPropertyContext, unit, "SubState"); ok {
		state.SubState = value
	}
	if value, ok := c.getStringProperty(ctx, conn.GetServicePropertyContext, unit, "Result"); ok {
		state.Result = value
	}

	return state, nil
}

// InvocationID retrieves a validated systemd InvocationID for unit.
// It returns ("", nil) when the ID is unavailable, invalid, or cannot be retrieved.
// It returns a non-nil error only if ctx is canceled or expires.
func (c *Client) InvocationID(ctx context.Context, unit string) (string, error) {
	conn, err := c.newConn(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return "", trace.Wrap(ctx.Err())
		}

		c.log.DebugContext(ctx, "Could not connect to systemd D-Bus while retrieving service invocation ID",
			"service", unit,
			"error", err,
		)
		return "", nil
	}
	defer conn.Close()

	prop, err := conn.GetUnitPropertyContext(ctx, unit, "InvocationID")
	if err != nil {
		if ctx.Err() != nil {
			return "", trace.Wrap(ctx.Err())
		}

		c.log.DebugContext(ctx, "Could not retrieve service invocation ID",
			"service", unit,
			"error", err,
		)
		return "", nil
	}
	if prop == nil {
		c.log.DebugContext(ctx, "Service invocation ID lookup returned no value",
			"service", unit,
		)
		return "", nil
	}

	value, ok := prop.Value.Value().([]byte)
	if !ok {
		c.log.DebugContext(ctx, "Ignoring invalid service invocation ID type",
			"service", unit,
			"value_type", fmt.Sprintf("%T", prop.Value.Value()),
		)
		return "", nil
	}
	if len(value) != invocationIDBytes {
		c.log.DebugContext(ctx, "Ignoring invalid service invocation ID length",
			"service", unit,
			"length", len(value),
		)
		return "", nil
	}

	return hex.EncodeToString(value), nil
}

func buildJournalctlArgs(unit, invocationID string, lines int) []string {
	args := []string{"--unit", unit, "--no-pager"}
	if lines > 0 {
		args = append(args, "--lines", fmt.Sprintf("%d", lines))
	}
	if invocationID != "" {
		// journalctl accepts field matches as positional arguments in addition to flags.
		args = append(args, "_SYSTEMD_INVOCATION_ID="+invocationID)
	}
	return args
}

// CaptureJournal is a best-effort helper that runs journalctl to retrieve recent log lines
// for the given systemd unit.
//
// Non-zero journalctl exits are treated as best-effort diagnostics failures: they are
// logged and any stdout is still returned. Command launch failures return an error so
// callers can distinguish them from empty journal output.
func (c *Client) CaptureJournal(ctx context.Context, unit string, opts JournalOptions) (string, error) {
	invocationID := ""
	if opts.FilterCurrentInvocation {
		var err error
		invocationID, err = c.InvocationID(ctx, unit)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	cmd := exec.CommandContext(ctx, c.journalctlPath, buildJournalctlArgs(unit, invocationID, opts.Lines)...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stdoutOutput := strings.TrimSpace(stdoutBuf.String())
	stderrOutput := strings.TrimSpace(stderrBuf.String())
	if err != nil {
		if ctx.Err() != nil {
			return "", trace.Wrap(ctx.Err())
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			c.log.DebugContext(ctx, "journalctl exited non-zero",
				"service", unit,
				"exit_code", exitErr.ExitCode(),
				"stderr", stderrOutput,
			)
			return stdoutOutput, nil
		} else {
			c.log.WarnContext(ctx, "Failed to capture journal output",
				"service", unit,
				"error", err,
				"stderr", stderrOutput,
			)
			return "", trace.Wrap(err)
		}
	}

	return stdoutOutput, nil
}
