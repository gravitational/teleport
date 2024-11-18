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
	// restartMonitorInterval is the polling interval for determining restart times from LastRestartPath.
	restartMonitorInterval = 2 * time.Second
	// minCleanIntervalsBeforeStable is the number of consecutive intervals before the service is determined stable.
	minCleanIntervalsBeforeStable = 6
	// maxRestartsBeforeFailure is the number of total restarts allowed before the service is marked as crash-looping.
	maxRestartsBeforeFailure = 2
)

// log keys
const (
	unitKey = "unit"
)

// SystemdService manages a systemd service (e.g., teleport or teleport-update).
type SystemdService struct {
	// ServiceName specifies the systemd service name.
	ServiceName string
	// LastRestartPath is a path to a file containing the last restart time.
	LastRestartPath string
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

	// Get initial restart time and initial PID.

	// If getRestartTime fails consistently, error will be returned from monitor.
	initRestartTime, err := readInt64(s.LastRestartPath)
	if err != nil {
		s.Log.DebugContext(ctx, "Initial restart time not present.", unitKey, s.ServiceName)
	}
	initPID, err := readInt(s.PIDPath)
	if err != nil {
		s.Log.DebugContext(ctx, "Initial PID not present.", unitKey, s.ServiceName)
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
		if err := s.verifyPID(ctx, initPID); err != nil {
			return trace.Wrap(err)
		}
	}
	s.Log.InfoContext(ctx, "Monitoring for excessive restarts.", unitKey, s.ServiceName)
	return trace.Wrap(s.monitor(ctx, initRestartTime))
}

func (s SystemdService) verifyPID(ctx context.Context, initPID int64) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tickC := time.NewTicker(restartMonitorInterval).C

	pidC := make(chan int64)
	g := &errgroup.Group{}
	g.Go(func() error {
		return tickFile(ctx, s.PIDPath, pidC, tickC, initPID)
	})
	err := s.waitForStablePID(ctx, initPID, tickC)
	cancel()
	if err := g.Wait(); err != nil {
		s.Log.WarnContext(ctx, "Unable to determine PID. Cannot failed reload.", unitKey, s.ServiceName)
		s.Log.DebugContext(ctx, "Error monitoring for crashing fork.", errorKey, err, unitKey, s.ServiceName)
	}
	return trace.Wrap(err)
}

func (s SystemdService) waitForStablePID(ctx context.Context, baseline int, pidC <-chan int64) error {
	var warnPID int
	var pid int
	for n, last := 0, 0; n < 3; n++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p := <-pidC:
			last = pid
			pid = int(p)
		}
		if pid != last ||
			pid == baseline ||
			pid == warnPID {
			n = 0
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			return trace.Wrap(err)
		}
		err = process.Signal(syscall.Signal(0))
		if errors.Is(err, syscall.ESRCH) {
			if pid != warnPID &&
				pid != baseline {
				s.Log.WarnContext(ctx, "Detecting crashing fork.", unitKey, s.ServiceName, "pid", pid)
				warnPID = pid
			}
			n = 0
			continue
		}
	}
	return nil
}

func readInt64(path string) (int64, error) {
	p, err := readFileN(path, 32)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	i, err := strconv.ParseInt(string(bytes.TrimSpace(p)), 10, 64)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return i, nil
}

func readInt(path string) (int, error) {
	i, err := readInt64(path)
	return int(i), trace.Wrap(err)
}

// monitor for excessive restarts by polling the LastRestartPath file.
// This function detects crash-looping while minimizing its own runtime during updates.
// To accomplish this, monitor fails after seeing maxRestartsBeforeFailure, and stops checking
// after seeing minCleanIntervalsBeforeStable clean intervals.
// initRestartTime may be provided as a baseline restart time, to ensure we catch the initial restart.
func (s SystemdService) monitor(ctx context.Context, initRestartTime int64) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tickC := time.NewTicker(restartMonitorInterval).C
	restartC := make(chan int64)
	g := &errgroup.Group{}
	g.Go(func() error {
		return tickFile(ctx, s.LastRestartPath, restartC, tickC, initRestartTime)
	})
	err := s.monitorRestarts(ctx, restartC, maxRestartsBeforeFailure, minCleanIntervalsBeforeStable)
	cancel()
	if err := g.Wait(); err != nil {
		s.Log.WarnContext(ctx, "Unable to determine last restart time. Cannot detect crash loops.", unitKey, s.ServiceName)
		s.Log.DebugContext(ctx, "Error monitoring for crash loops.", errorKey, err, unitKey, s.ServiceName)
	}
	return trace.Wrap(err)
}

// monitorRestarts receives restart times on timeCh.
// Each restart time that differs from the preceding restart time counts as a restart.
// If maxRestarts is exceeded, monitorRestarts returns an error.
// Each restart time that matches the proceeding restart time counts as a clean reading.
// If minClean is reached before maxRestarts is exceeded, monitorRestarts runs nil.
func (s SystemdService) monitorRestarts(ctx context.Context, timeCh <-chan int64, maxRestarts, minClean int) error {
	var (
		same, diff  int
		restartTime int64
	)
	for {
		// wait first to ensure we initial stop has completed
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-timeCh:
			switch t {
			case restartTime:
				same++
			default:
				same = 0
				restartTime = t
				diff++
			}
		}
		switch {
		case diff > maxRestarts+1:
			return trace.Errorf("detected crash loop")
		case same >= minClean:
			return nil
		}
	}
}

// tickRestarts reads the current time on tickC, and outputs the last restart time on ch for each received tick.
// If the current time cannot be read, tickRestarts sends 0 on ch.
// Any error from the last attempt to receive restart times is returned when ctx is cancelled.
// The baseline restart time is sent as soon as the method is called
func (s SystemdService) tickRestarts(ctx context.Context, ch chan<- int64, tickC <-chan time.Time, baseline int64) error {
	t := baseline
	var err error
	select {
	case ch <- t:
	case <-ctx.Done():
		return err
	}
	for {
		// two select statements -> never skip restarts
		select {
		case <-tickC:
		case <-ctx.Done():
			return err
		}
		var t int64
		t, err = readInt64(s.LastRestartPath)
		select {
		case ch <- t:
		case <-ctx.Done():
			return err
		}
	}
}

func tickFile(ctx context.Context, path string, ch chan<- int64, tickC <-chan time.Time, baseline int64) error {
	t := baseline
	var err error
	select {
	case ch <- t:
	case <-ctx.Done():
		return err
	}
	for {
		// two select statements -> never skip reads
		select {
		case <-tickC:
		case <-ctx.Done():
			return err
		}
		var t int64
		t, err = readInt64(path)
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
	s.Log.InfoContext(ctx, "Systemd configuration synced.")
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
	code := s.systemctl(ctx, slog.LevelError, args...)
	if code != 0 {
		return trace.Errorf("unable to enable systemd service")
	}
	s.Log.InfoContext(ctx, "Service enabled.", unitKey, s.ServiceName)
	return nil
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
	if err != nil {
		s.Log.Log(ctx, errLevel, "Failed to run systemctl.",
			"args", args,
			"code", code,
			errorKey, err)
	}
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
	stderr := &lineLogger{ctx: ctx, log: c.Log, level: c.ErrLevel}
	stdout := &lineLogger{ctx: ctx, log: c.Log, level: c.OutLevel}
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
	ctx   context.Context
	log   *slog.Logger
	level slog.Level

	last bytes.Buffer
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
	w.log.Log(w.ctx, w.level, w.last.String()) //nolint:sloglint // msg cannot be constant
	n += 1
	w.last.Reset()

	// Log lines that are already newline-terminated
	for _, line := range lines[:len(lines)-1] {
		w.log.Log(w.ctx, w.level, string(line)) //nolint:sloglint // msg cannot be constant
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
	w.log.Log(w.ctx, w.level, w.last.String()) //nolint:sloglint // msg cannot be constant
	w.last.Reset()
}
