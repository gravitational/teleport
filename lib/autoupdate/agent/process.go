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
	"cmp"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/client/debug"
)

// process monitoring consts
const (
	// monitorTimeout is the timeout for determining whether the process has started.
	monitorTimeout = 1 * time.Minute
	// monitorInterval is the polling interval for determining whether the process has started.
	monitorInterval = 2 * time.Second
	// minRunningIntervalsBeforeStable is the number of consecutive intervals with the same running PID detected
	// before the service is determined stable.
	minRunningIntervalsBeforeStable = 6
	// maxCrashesBeforeFailure is the number of total crashes detected before the service is marked as crash-looping.
	maxCrashesBeforeFailure = 2
)

// log keys
const (
	unitKey = "unit"
)

// processMonitor detects crashes and readiness of a process after it has been restarted.
// The process-manager-specific PID source is supplied by the caller, so the same
// crash-detection state machine is shared by all Process implementations
// (e.g., systemd reads a PID file, while SCM queries the service status).
type processMonitor struct {
	// name of the monitored process, used for logging.
	name string
	// ready is a readiness checker. If nil, readiness is not checked.
	ready ReadyChecker
	// log contains a logger.
	log *slog.Logger
	// timeout for the entire monitoring operation.
	// Defaults to monitorTimeout if zero.
	timeout time.Duration
	// interval between PID and readiness polls.
	// Defaults to monitorInterval if zero.
	interval time.Duration
	// tickPID must send the current PID of the process on pidC for each tick received on tickC,
	// or 0 if the process is not running. tickPID must return when ctx is canceled.
	tickPID func(ctx context.Context, pidC chan<- int, tickC <-chan time.Time) error
	// verifyPID must return os.ErrProcessDone (or syscall.ESRCH) if the process
	// identified by pid is not running, and nil if it is running.
	verifyPID func(pid int) error
}

// ReadyChecker returns the service readiness status.
type ReadyChecker interface {
	GetReadiness(ctx context.Context) (debug.Readiness, error)
}

// monitor for a started, healthy process.
// monitor logs all errors that should be displayed to the user.
// initPID is the PID before the restart operation was issued, or 0 if unknown.
func (m processMonitor) monitor(ctx context.Context, initPID int) error {
	ctx, cancel := context.WithTimeout(ctx, cmp.Or(m.timeout, monitorTimeout))
	defer cancel()
	interval := cmp.Or(m.interval, monitorInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	newPID := 0
	if initPID != 0 {
		m.log.InfoContext(ctx, "Monitoring process ID to detect crashes.", unitKey, m.name)
		var err error
		newPID, err = m.monitorPID(ctx, initPID, ticker.C)
		if errors.Is(err, context.DeadlineExceeded) {
			m.log.ErrorContext(ctx, "Timed out monitoring for crashing PID.", unitKey, m.name)
			return trace.Wrap(err)
		}
		if err != nil {
			m.log.ErrorContext(ctx, "Error monitoring for crashing PID.", errorKey, err, unitKey, m.name)
			return trace.Wrap(err)
		}
	}

	m.log.InfoContext(ctx, "Monitoring diagnostic socket to detect readiness.", unitKey, m.name)
	ticker = time.NewTicker(interval)
	defer ticker.Stop()
	err := m.waitForReady(ctx, newPID, ticker.C)
	if errors.Is(err, context.DeadlineExceeded) {
		m.log.ErrorContext(ctx, "Timed out monitoring for process readiness.", unitKey, m.name)
		return trace.Wrap(err)
	}
	if err != nil {
		m.log.ErrorContext(ctx, "Error monitoring for process readiness.", errorKey, err, unitKey, m.name)
		return trace.Wrap(err)
	}
	return nil
}

// monitorPID for the started process to ensure it's running by polling tickPID.
// This function detects several types of crashes while minimizing its own runtime during updates.
// For example, the process may crash by failing to fork (non-running PID), or looping (repeatedly changing PID),
// or getting stuck on quit (no change in PID).
// initPID is the PID before the restart operation has been issued.
// The final PID is returned.
func (m processMonitor) monitorPID(ctx context.Context, initPID int, tickC <-chan time.Time) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pidC := make(chan int)
	var g errgroup.Group
	g.Go(func() error {
		return m.tickPID(ctx, pidC, tickC)
	})
	stablePID, err := m.waitForStablePID(ctx, minRunningIntervalsBeforeStable, maxCrashesBeforeFailure,
		initPID, pidC, m.verifyPID)
	cancel()
	if err := g.Wait(); err != nil {
		m.log.ErrorContext(ctx, "Error monitoring for crashing process.", errorKey, err, unitKey, m.name)
	}
	return stablePID, trace.Wrap(err)
}

// waitForStablePID monitors a service's PID via pidC and determines whether the service is crashing.
// verifyPID must be passed so that waitForStablePID can determine whether the process is running.
// verifyPID must return os.ErrProcessDone in the case that the PID cannot be found, or nil otherwise.
// baselinePID is the initial PID before any operation that might cause the process to start crashing.
// minStable is the number of times pidC must return the same running PID before waitForStablePID returns nil.
// minCrashes is the number of times pidC conveys a process crash or bad state before waitForStablePID returns an error.
// The last reported PID is returned.
func (m processMonitor) waitForStablePID(ctx context.Context, minStable, maxCrashes, baselinePID int, pidC <-chan int, verifyPID func(pid int) error) (int, error) {
	pid := baselinePID
	var last, stale int
	var crashes int
	for stable := 0; stable < minStable; stable++ {
		select {
		case <-ctx.Done():
			return pid, ctx.Err()
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
			return pid, trace.Errorf("detected crashing process")
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
		// A stale PID most likely indicates that the process forked and crashed without the process manager noticing.
		// There is a small chance that we read the PID before the process manager cleared it.
		// Note: we only perform this check on PIDs that survive one iteration.
		if errors.Is(err, os.ErrProcessDone) ||
			errors.Is(err, syscall.ESRCH) {
			if pid != stale &&
				pid != baselinePID {
				stale = pid
				m.log.WarnContext(ctx, "Detected stale PID.", unitKey, m.name, "pid", stale)
			}
			stable = -1
			continue
		}
		if err != nil {
			return pid, trace.Wrap(err)
		}
	}
	return pid, nil
}

// waitForReady polls the readiness checker.
// If one request returns ready before the timeout, the service is considered ready.
func (m processMonitor) waitForReady(ctx context.Context, pid int, tickC <-chan time.Time) error {
	if m.ready == nil {
		return nil
	}
	var lastErr error
	var readiness debug.Readiness
	for {
		resp, err := m.ready.GetReadiness(ctx)
		if err == nil &&
			resp.Ready &&
			equalOrZero(resp.PID, pid) {
			return nil
		}
		// If the Readiness check fails to due to intervention, we must not interpret
		// the error as a disabled socket, which results in a passing check.
		if !errors.Is(err, context.Canceled) &&
			!errors.Is(err, context.DeadlineExceeded) {
			lastErr = err
			readiness = resp
		}
		select {
		case <-ctx.Done():
			if errors.Is(lastErr, os.ErrNotExist) ||
				errors.Is(lastErr, syscall.EINVAL) ||
				errors.Is(lastErr, os.ErrInvalid) ||
				errors.As(lastErr, new(net.Error)) {
				m.log.WarnContext(ctx, "Socket appears to be disabled. Proceeding without check.", unitKey, m.name)
				m.log.DebugContext(ctx, "Found error after timeout polling socket.", unitKey, m.name, errorKey, lastErr)
				return nil
			}
			if lastErr != nil {
				m.log.WarnContext(ctx, "Unexpected error after timeout polling socket. Proceeding without check.", unitKey, m.name, errorKey, lastErr)
				return nil
			}
			if readiness.Status != "" {
				m.log.ErrorContext(ctx, "Process not ready by deadline.", unitKey, m.name, "status", readiness.Status)
			}
			if !equalOrZero(readiness.PID, pid) {
				m.log.ErrorContext(ctx, "Readiness PID response does not match monitored PID.", unitKey, m.name, "monitored_pid", pid, "ready_pid", readiness.PID)
			} else {
				m.log.DebugContext(ctx, "PIDs are not mismatched.", unitKey, m.name, "monitored_pid", pid, "ready_pid", readiness.PID)
			}
			return ctx.Err()
		case <-tickC:
		}
	}
}

// equalOrZero returns true if a and b are equal, or if either has the zero-value.
func equalOrZero[T comparable](a, b T) bool {
	var empty T
	if a == empty || b == empty {
		return true
	}
	return a == b
}

// ProcessGroup is a group of other Teleport processes.
type ProcessGroup []Process

func (p ProcessGroup) Name() string {
	return "Teleport services"
}

// Reload reloads all processes in the process group.
func (p ProcessGroup) Reload(ctx context.Context) error {
	// TODO(sclevine): consider reloading in parallel if this is too slow for users
	for _, process := range p {
		if err := process.Reload(ctx); err != nil {
			return trace.Wrap(err, "failed to reload %s", process.Name())
		}
	}
	return nil
}

// Sync syncs only the first process in the group, and fails if no processes are present.
// The systemctl daemon-reload command is global, so we only need to sync once.
func (p ProcessGroup) Sync(ctx context.Context) error {
	if len(p) == 0 {
		return trace.Errorf("no services to sync")
	}
	return trace.Wrap(p[0].Sync(ctx))
}

// IsEnabled returns true if any processes in the group are enabled.
func (p ProcessGroup) IsEnabled(ctx context.Context) (bool, error) {
	return p.anyAreTrue(ctx, func(ctx context.Context, p Process) (bool, error) {
		return p.IsEnabled(ctx)
	})
}

// IsPresent returns true if any processes in the group are present.
func (p ProcessGroup) IsPresent(ctx context.Context) (bool, error) {
	return p.anyAreTrue(ctx, func(ctx context.Context, p Process) (bool, error) {
		return p.IsPresent(ctx)
	})
}

// IsActive returns true if any processes in the group are active.
func (p ProcessGroup) IsActive(ctx context.Context) (bool, error) {
	return p.anyAreTrue(ctx, func(ctx context.Context, p Process) (bool, error) {
		return p.IsActive(ctx)
	})
}

func (p ProcessGroup) anyAreTrue(ctx context.Context, f func(ctx context.Context, p Process) (bool, error)) (bool, error) {
	for _, process := range p {
		ok, err := f(ctx, process)
		if err != nil {
			return ok, trace.Wrap(err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// localExec runs a command locally, logging any output.
type localExec struct {
	// Dir specifies the working directory of the local command.
	Dir string
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
	cmd.Dir = c.Dir
	stderr := &lineLogger{ctx: ctx, log: c.Log, level: c.ErrLevel, prefix: "[stderr] "}
	stdout := &lineLogger{ctx: ctx, log: c.Log, level: c.OutLevel, prefix: "[stdout] "}
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	err := cmd.Run()
	stderr.Flush()
	stdout.Flush()
	code := cmd.ProcessState.ExitCode()
	return code, trace.Wrap(err)
}

// Output runs the command and returns combined output from stdout and stderr.
// Same arguments as exec.CommandContext.
func (c *localExec) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	var buf bytes.Buffer
	stderrLogger := &lineLogger{ctx: ctx, log: c.Log, level: c.ErrLevel, prefix: "[stderr] "}
	stdoutLogger := &lineLogger{ctx: ctx, log: c.Log, level: c.OutLevel, prefix: "[stdout] "}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = c.Dir
	stderr := io.MultiWriter(&buf, stderrLogger)
	stdout := io.MultiWriter(&buf, stdoutLogger)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	err := cmd.Run()
	stderrLogger.Flush()
	stdoutLogger.Flush()
	return buf.Bytes(), trace.Wrap(err)
}
