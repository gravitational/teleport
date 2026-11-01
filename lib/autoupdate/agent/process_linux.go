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

package agent

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
)

// SystemdService manages a systemd service (e.g., teleport or teleport-update).
type SystemdService struct {
	// ServiceName specifies the systemd service name.
	ServiceName string
	// PIDFile is a path to a file containing the service's PID.
	PIDFile string
	// Ready is a readiness checker.
	Ready ReadyChecker
	// Log contains a logger.
	Log *slog.Logger
	// ForceRestart forces the process to always restart.
	ForceRestart bool
}

// Name of the systemd service.
func (s SystemdService) Name() string {
	return s.ServiceName
}

// Reload the systemd service.
// Attempts a graceful reload before a hard restart.
// See Process interface for more details.
func (s SystemdService) Reload(ctx context.Context, forceStart bool) error {
	// TODO(sclevine): allow server to force restart instead of reload

	if err := s.checkSystem(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Command error codes < 0 indicate that we are unable to run the command.
	// Errors from s.systemctl are logged along with stderr and stdout (debug only).

	// If the service is not running and the caller does not need it started,
	// return nil.
	// Note systemctl reload returns an error if the unit is not active, and
	// try-reload-or-restart is too recent of an addition for centos7.
	code := s.systemctl(ctx, slog.LevelDebug, "is-active", "--quiet", s.ServiceName)
	switch {
	case code < 0:
		return trace.Errorf("unable to determine if systemd service is active")
	case code > 0 && !forceStart:
		s.Log.WarnContext(ctx, "Systemd service not running.", unitKey, s.ServiceName)
		return nil
	}

	// Get initial PID for crash monitoring.

	initialPID, err := readInt(s.PIDFile)
	if errors.Is(err, os.ErrNotExist) {
		s.Log.InfoContext(ctx, "No existing process detected. Skipping crash monitoring.", unitKey, s.ServiceName)
	} else if err != nil {
		s.Log.ErrorContext(ctx, "Error reading initial PID value. Skipping crash monitoring.", unitKey, s.ServiceName, errorKey, err)
	}

	if initialPID == 0 && forceStart {
		// set initial PID to non-zero so we will start monitoring it
		initialPID = -1
	}

	// Attempt graceful reload of running service.
	// forceStart may be requested when the service is not currently running
	// (e.g., it crashed and was not restarted by systemd), in which case
	// reload/try-restart would be a no-op, so fall through to restart instead.
	if !s.ForceRestart && code == 0 {
		code = s.systemctl(ctx, slog.LevelError, "reload", s.ServiceName)
		switch {
		case code < 0:
			return trace.Errorf("unable to reload systemd service")
		case code > 0:
			// Graceful reload fails, try hard restart.
			code = s.systemctl(ctx, slog.LevelError, "try-restart", s.ServiceName)
			if code != 0 {
				return trace.Errorf("hard restart of systemd service failed")
			}
			s.Log.WarnContext(ctx, "Service ungracefully restarted. Connections potentially dropped.", unitKey, s.ServiceName)
		default:
			s.Log.InfoContext(ctx, "Gracefully reloaded.", unitKey, s.ServiceName)
		}
	} else {
		// restart starts the unit if it is not currently active, unlike
		// try-restart, which is a no-op in that case.
		code = s.systemctl(ctx, slog.LevelError, "restart", s.ServiceName)
		if code != 0 {
			return trace.Errorf("hard restart of systemd service failed")
		}
	}

	// monitor logs all relevant errors, so we filter for a few outcomes
	err = s.monitor(ctx, initialPID)
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return trace.Wrap(err)
	}
	if err != nil {
		return trace.Errorf("failed to monitor process")
	}
	return nil
}

// monitor for a started, healthy process by polling PIDFile and the readiness socket.
func (s SystemdService) monitor(ctx context.Context, initPID int) error {
	m := processMonitor{
		name:     s.ServiceName,
		ready:    s.Ready,
		log:      s.Log,
		timeout:  monitorTimeout,
		interval: monitorInterval,
		tickPID: func(ctx context.Context, pidC chan<- int, tickC <-chan time.Time) error {
			return tickFile(ctx, s.PIDFile, pidC, tickC)
		},
		verifyPID: func(pid int) error {
			p, err := os.FindProcess(pid)
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(p.Signal(syscall.Signal(0)))
		},
	}
	return m.monitor(ctx, initPID)
}

// readInt reads an integer from a file.
func readInt(path string) (int, error) {
	p, err := readFileAtMost(path, 32)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	i, err := strconv.ParseInt(string(bytes.TrimSpace(p)), 10, 64)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return int(i), nil
}

// tickFile reads the current time on tickC, and outputs the last read int from path on ch for each received tick.
// If the path cannot be read, tickFile sends 0 on ch.
// Any error from the last attempt to read path is returned when ctx is canceled, unless the error is os.ErrNotExist.
func tickFile(ctx context.Context, path string, ch chan<- int, tickC <-chan time.Time) error {
	var err error
	for {
		// two select statements -> never skip reads
		select {
		case <-tickC:
		case <-ctx.Done():
			return err
		}
		var t int
		t, err = readInt(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		select {
		case ch <- t:
		case <-ctx.Done():
			return err
		}
	}
}

// Sync systemd service configuration by running systemctl daemon-reload.
// See Process interface for more details.
func (s SystemdService) Sync(ctx context.Context) error {
	if err := s.checkSystem(ctx); err != nil {
		return trace.Wrap(err)
	}
	code := s.systemctl(ctx, slog.LevelError, "daemon-reload")
	if code != 0 {
		return trace.Errorf("unable to reload systemd configuration")
	}
	s.Log.InfoContext(ctx, "Systemd configuration synced.", unitKey, s.ServiceName)
	return nil
}

// Enable the systemd service.
func (s SystemdService) Enable(ctx context.Context, now bool) error {
	if err := s.checkSystem(ctx); err != nil {
		return trace.Wrap(err)
	}
	// The --now flag is not supported in systemd versions older than 220,
	// so perform enable + start commands instead.
	code := s.systemctl(ctx, slog.LevelInfo, "enable", s.ServiceName)
	if code != 0 {
		return trace.Errorf("unable to enable systemd service")
	}
	if now {
		code := s.systemctl(ctx, slog.LevelInfo, "start", s.ServiceName)
		if code != 0 {
			return trace.Errorf("unable to start systemd service")
		}
	}
	s.Log.InfoContext(ctx, "Systemd service enabled.", unitKey, s.ServiceName, "now", now)
	return nil
}

// Disable the systemd service.
func (s SystemdService) Disable(ctx context.Context, now bool) error {
	if err := s.checkSystem(ctx); err != nil {
		return trace.Wrap(err)
	}
	// The --now flag is not supported in systemd versions older than 220,
	// so perform disable + stop commands instead.
	code := s.systemctl(ctx, slog.LevelInfo, "disable", s.ServiceName)
	if code != 0 {
		return trace.Errorf("unable to disable systemd service")
	}
	if now {
		code := s.systemctl(ctx, slog.LevelInfo, "stop", s.ServiceName)
		if code != 0 {
			return trace.Errorf("unable to stop systemd service")
		}
	}
	s.Log.InfoContext(ctx, "Systemd service disabled.", unitKey, s.ServiceName, "now", now)
	return nil
}

// IsEnabled returns true if the service is enabled.
func (s SystemdService) IsEnabled(ctx context.Context) (bool, error) {
	if err := s.checkSystem(ctx); err != nil {
		return false, trace.Wrap(err)
	}
	if hasSystemDBelow(ctx, 238) {
		return false, trace.Wrap(ErrNotAvailable)
	}
	code := s.systemctl(ctx, slog.LevelDebug, "is-enabled", "--quiet", s.ServiceName)
	switch {
	case code < 0:
		return false, trace.Errorf("unable to determine if systemd service %s is enabled", s.ServiceName)
	case code == 0:
		return true, nil
	}
	return false, nil
}

// IsActive returns true if the service is active.
func (s SystemdService) IsActive(ctx context.Context) (bool, error) {
	if err := s.checkSystem(ctx); err != nil {
		return false, trace.Wrap(err)
	}
	code := s.systemctl(ctx, slog.LevelDebug, "is-active", "--quiet", s.ServiceName)
	switch {
	case code < 0:
		return false, trace.Errorf("unable to determine if systemd service %s is active", s.ServiceName)
	case code == 0:
		return true, nil
	}
	return false, nil
}

// IsPresent returns true if the service exists.
func (s SystemdService) IsPresent(ctx context.Context) (bool, error) {
	if err := s.checkSystem(ctx); err != nil {
		return false, trace.Wrap(err)
	}
	if hasSystemDBelow(ctx, 246) {
		return false, trace.Wrap(ErrNotAvailable)
	}
	code := s.systemctl(ctx, slog.LevelDebug, "list-unit-files", "--quiet", s.ServiceName)
	if code < 0 {
		return false, trace.Errorf("unable to determine if systemd service %s is present", s.ServiceName)
	}
	return code == 0, nil
}

// checkSystem returns an error if the system is not compatible with this process manager.
func (s SystemdService) checkSystem(ctx context.Context) error {
	present, err := hasServiceManager()
	if err != nil {
		return trace.Wrap(err)
	}
	if !present {
		return trace.Wrap(ErrNotSupported)
	}
	return nil
}

// hasServiceManager returns true if the system uses the SystemD process manager.
func hasServiceManager() (bool, error) {
	_, err := os.Stat("/run/systemd/system")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}

// hasSystemDBelow returns true the version of systemd can be determined, and it
// is below the provided version.
func hasSystemDBelow(ctx context.Context, i int) bool {
	cmd := exec.CommandContext(ctx, "systemctl", "--version")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	v, ok := parseSystemDVersion(out)
	return ok && v < i
}

// parseSystemDVersion parses the SystemD version from systemctl command output.
func parseSystemDVersion(out []byte) (int, bool) {
	first, _, _ := strings.Cut(string(out), "\n")
	parts := strings.SplitN(first, " ", 3)
	if len(parts) < 2 || parts[0] != "systemd" {
		return 0, false
	}
	version, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, false
	}
	return version, true
}

// systemctl returns a systemctl subcommand, converting the output to logs.
// Output sent to stdout is logged at debug level.
// Output sent to stderr is logged at the level specified by errLevel.
func (s SystemdService) systemctl(ctx context.Context, errLevel slog.Level, args ...string) int {
	cmd := &localExec{
		Log:      s.Log,
		ErrLevel: errLevel,
		OutLevel: slog.LevelDebug,
	}
	code, err := cmd.Run(ctx, "systemctl", args...)
	if err == nil {
		return code
	}
	if code >= 0 {
		s.Log.Log(ctx, errLevel, "Non-zero exit code or error running systemctl.",
			"args", args, "code", code)
		return code
	}
	s.Log.Log(ctx, errLevel, "Unable to run systemctl.",
		"args", args, "code", code, errorKey, err)
	return code
}
