/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package agent

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
)

const (
	// crashMonitorInterval is the polling interval for determining restart times from PIDFile.
	crashMonitorInterval = 2 * time.Second
	// minRunningIntervalsBeforeStable is the number of consecutive intervals with the same running PID detected
	// before the service is determined stable.
	minRunningIntervalsBeforeStable = 6
	// maxCrashesBeforeFailure is the number of total crashes detected before the service is marked as crash-looping.
	maxCrashesBeforeFailure = 2
	// crashMonitorTimeout
	crashMonitorTimeout = 30 * time.Second
)

// log keys
const (
	unitKey = "unit"
)

// SystemdService manages a systemd service (e.g., teleport or teleport-update).
type SystemdService struct {
	// ServiceName specifies the systemd service name.
	ServiceName string
	// PIDPath is a path to a file containing the service's PID.
	PIDPath string
	// Log contains a logger.
	Log *slog.Logger
}

// Reload the systemd service.
// Attempts a graceful reload before a hard restart.
// See Process interface for more details.
func (s SystemdService) Reload(ctx context.Context) error {
	// TODO(sclevine): allow server to force restart instead of reload

	if err := s.checkSystem(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Command error codes < 0 indicate that we are unable to run the command.
	// Errors from s.systemctl are logged along with stderr and stdout (debug only).

	// If the service is not running, return ErrNotNeeded.
	// Note systemctl reload returns an error if the unit is not active, and
	// try-reload-or-restart is too recent of an addition for centos7.
	code := s.systemctl(ctx, slog.LevelDebug, "is-active", "--quiet", s.ServiceName)
	switch {
	case code < 0:
		return trace.Errorf("unable to determine if systemd service is active")
	case code > 0:
		s.Log.WarnContext(ctx, "Systemd service not running.", unitKey, s.ServiceName)
		return trace.Wrap(ErrNotNeeded)
	}

	// Get initial PID for crash monitoring.

	initPID, err := readInt(s.PIDPath)
	if errors.Is(err, os.ErrNotExist) {
		s.Log.InfoContext(ctx, "No existing process detected. Skipping crash monitoring.", unitKey, s.ServiceName)
	} else if err != nil {
		s.Log.ErrorContext(ctx, "Error reading initial PID value. Skipping crash monitoring.", unitKey, s.ServiceName, errorKey, err)
	}

	// Attempt graceful reload of running service.
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
	if initPID != 0 {
		s.Log.InfoContext(ctx, "Monitoring PID file to detect crashes.", unitKey, s.ServiceName)
		err := s.monitor(ctx, initPID)
		if errors.Is(err, context.DeadlineExceeded) {
			return trace.Errorf("timed out while waiting for process to start")
		}
		return trace.Wrap(err)
	}
	return nil
}

// monitor for the started process to ensure it's running by polling PIDFile.
// This function detects several types of crashes while minimizing its own runtime during updates.
// For example, the process may crash by failing to fork (non-running PID), or looping (repeatedly changing PID),
// or getting stuck on quit (no change in PID).
// initPID is the PID before the restart operation has been issued.
func (s SystemdService) monitor(ctx context.Context, initPID int) error {
	ctx, cancel := context.WithTimeout(ctx, crashMonitorTimeout)
	defer cancel()
	tickC := time.NewTicker(crashMonitorInterval).C

	pidC := make(chan int)
	g := &errgroup.Group{}
	g.Go(func() error {
		return tickFile(ctx, s.PIDPath, pidC, tickC)
	})
	err := s.waitForStablePID(ctx, minRunningIntervalsBeforeStable, maxCrashesBeforeFailure,
		initPID, pidC, func(pid int) error {
			p, err := os.FindProcess(pid)
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(p.Signal(syscall.Signal(0)))
		})
	cancel()
	if err := g.Wait(); err != nil {
		s.Log.ErrorContext(ctx, "Error monitoring for crashing process.", errorKey, err, unitKey, s.ServiceName)
	}
	return trace.Wrap(err)
}

// waitForStablePID monitors a service's PID via pidC and determines whether the service is crashing.
// verifyPID must be passed so that waitForStablePID can determine whether the process is running.
// verifyPID must return os.ErrProcessDone in the case that the PID cannot be found, or nil otherwise.
// baselinePID is the initial PID before any operation that might cause the process to start crashing.
// minStable is the number of times pidC must return the same running PID before waitForStablePID returns nil.
// minCrashes is the number of times pidC conveys a process crash or bad state before waitForStablePID returns an error.
func (s SystemdService) waitForStablePID(ctx context.Context, minStable, maxCrashes, baselinePID int, pidC <-chan int, verifyPID func(pid int) error) error {
	pid := baselinePID
	var last, stale int
	var crashes int
	for stable := 0; stable < minStable; stable++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p := <-pidC:
			last = pid
			pid = p
		}
		// A "crash" is defined as a transition away from a new (non-baseline) PID, or
		// an interval where the current PID remains non-running (stale) since the last check.
		if (last != 0 && pid != last && last != baselinePID) ||
			(stale != 0 && pid == stale && last == stale) {
			crashes++
		}
		if crashes > maxCrashes {
			return trace.Errorf("detected crashing process")
		}

		// PID can only be stable if it is a real PID that is not new,
		// has changed at least once, and hasn't been observed as missing.
		if pid == 0 ||
			pid == baselinePID ||
			pid == stale ||
			pid != last {
			stable = -1
			continue
		}
		err := verifyPID(pid)
		// A stale PID most likely indicates that the process forked and crashed without systemd noticing.
		// There is a small chance that we read the PID file before systemd removed it.
		// Note: we only perform this check on PIDs that survive one iteration.
		if errors.Is(err, os.ErrProcessDone) ||
			errors.Is(err, syscall.ESRCH) {
			if pid != stale &&
				pid != baselinePID {
				stale = pid
				s.Log.WarnContext(ctx, "Detected stale PID.", unitKey, s.ServiceName, "pid", stale)
			}
			stable = -1
			continue
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// readInt reads an integer from a file.
func readInt(path string) (int, error) {
	p, err := readFileN(path, 32)
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
	args := []string{"enable", s.ServiceName}
	if now {
		args = append(args, "--now")
	}
	code := s.systemctl(ctx, slog.LevelInfo, args...)
	if code != 0 {
		return trace.Errorf("unable to enable systemd service")
	}
	s.Log.InfoContext(ctx, "Service enabled.", unitKey, s.ServiceName)
	return nil
}

// Disable the systemd service.
func (s SystemdService) Disable(ctx context.Context) error {
	if err := s.checkSystem(ctx); err != nil {
		return trace.Wrap(err)
	}
	code := s.systemctl(ctx, slog.LevelInfo, "disable", s.ServiceName)
	if code != 0 {
		return trace.Errorf("unable to disable systemd service")
	}
	s.Log.InfoContext(ctx, "Systemd service disabled.", unitKey, s.ServiceName)
	return nil
}

// IsEnabled returns true if the service is enabled, or if it's disabled but still active.
func (s SystemdService) IsEnabled(ctx context.Context) (bool, error) {
	if err := s.checkSystem(ctx); err != nil {
		return false, trace.Wrap(err)
	}
	code := s.systemctl(ctx, slog.LevelDebug, "is-enabled", "--quiet", s.ServiceName)
	switch {
	case code < 0:
		return false, trace.Errorf("unable to determine if systemd service %s is enabled", s.ServiceName)
	case code == 0:
		return true, nil
	}
	code = s.systemctl(ctx, slog.LevelDebug, "is-active", "--quiet", s.ServiceName)
	switch {
	case code < 0:
		return false, trace.Errorf("unable to determine if systemd service %s is active", s.ServiceName)
	case code == 0:
		return true, nil
	}
	return false, nil
}

// checkSystem returns an error if the system is not compatible with this process manager.
func (s SystemdService) checkSystem(ctx context.Context) error {
	_, err := os.Stat("/run/systemd/system")
	if errors.Is(err, os.ErrNotExist) {
		s.Log.ErrorContext(ctx, "This system does not support systemd, which is required by the updater.")
		return trace.Wrap(ErrNotSupported)
	}
	return trace.Wrap(err)
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
		s.Log.Log(ctx, errLevel, "Error running systemctl.",
			"args", args, "code", code)
		return code
	}
	s.Log.Log(ctx, errLevel, "Unable to run systemctl.",
		"args", args, "code", code, errorKey, err)
	return code
}

// localExec runs a command locally, logging any output.
type localExec struct {
	// Log contains a slog logger.
	// Defaults to slog.Default() if nil.
	Log *slog.Logger
	// ErrLevel is the log level for stderr.
	ErrLevel slog.Level
	// OutLevel is the log level for stdout.
	OutLevel slog.Level
}

// Run the command. Same arguments as exec.CommandContext.
// Outputs the status code, or -1 if out-of-range or unstarted.
func (c *localExec) Run(ctx context.Context, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stderr := &lineLogger{ctx: ctx, log: c.Log, level: c.ErrLevel, prefix: "[stderr] "}
	stdout := &lineLogger{ctx: ctx, log: c.Log, level: c.OutLevel, prefix: "[stdout] "}
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	err := cmd.Run()
	stderr.Flush()
	stdout.Flush()
	code := cmd.ProcessState.ExitCode()

	// Treat out-of-range exit code (255) as an error executing the command.
	// This allows callers to treat codes that are more likely OS-related as execution errors
	// instead of intentionally returned error codes.
	if code == 255 {
		code = -1
	}
	return code, trace.Wrap(err)
}

// lineLogger logs each line written to it.
type lineLogger struct {
	ctx    context.Context
	log    *slog.Logger
	level  slog.Level
	prefix string

	last bytes.Buffer
}

func (w *lineLogger) out(s string) {
	w.log.Log(w.ctx, w.level, w.prefix+s) //nolint:sloglint // msg cannot be constant
}

func (w *lineLogger) Write(p []byte) (n int, err error) {
	lines := bytes.Split(p, []byte("\n"))
	// Finish writing line
	if len(lines) > 0 {
		n, err = w.last.Write(lines[0])
		lines = lines[1:]
	}
	// Quit if no newline
	if len(lines) == 0 || err != nil {
		return n, trace.Wrap(err)
	}

	// Newline found, log line
	w.out(w.last.String())
	n += 1
	w.last.Reset()

	// Log lines that are already newline-terminated
	for _, line := range lines[:len(lines)-1] {
		w.out(string(line))
		n += len(line) + 1
	}

	// Store remaining line non-newline-terminated line.
	n2, err := w.last.Write(lines[len(lines)-1])
	n += n2
	return n, trace.Wrap(err)
}

// Flush logs any trailing bytes that were never terminated with a newline.
func (w *lineLogger) Flush() {
	if w.last.Len() == 0 {
		return
	}
	w.out(w.last.String())
	w.last.Reset()
}
