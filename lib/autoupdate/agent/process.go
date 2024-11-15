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
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
)

// SystemdService manages a Teleport systemd service.
type SystemdService struct {
	// ServiceName specifies the systemd service name.
	ServiceName string
	// LastRestartPath is a path to a file containing the last restart time.
	LastRestartPath string
	// Log contains a logger.
	Log *slog.Logger
}

// Reload a systemd service.
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
		s.Log.WarnContext(ctx, "Teleport systemd service not running.")
		return trace.Wrap(ErrNotNeeded)
	}
	// Attempt graceful reload of running service.
	code = s.systemctl(ctx, slog.LevelError, "reload", s.ServiceName)
	switch {
	case code < 0:
		return trace.Errorf("unable to attempt reload of Teleport systemd service")
	case code > 0:
		// Graceful reload fails, try hard restart.
		code = s.systemctl(ctx, slog.LevelError, "try-restart", s.ServiceName)
		if code != 0 {
			return trace.Errorf("hard restart of Teleport systemd service failed")
		}
		s.Log.WarnContext(ctx, "Teleport ungracefully restarted. Connections potentially dropped.")
	default:
		s.Log.InfoContext(ctx, "Teleport gracefully reloaded.")
	}
	s.Log.InfoContext(ctx, "Monitoring for excessive restarts.")
	return trace.Wrap(s.monitor(ctx))
}

func (s SystemdService) monitor(ctx context.Context) error {
	tickC := time.NewTicker(2 * time.Second).C
	restartC := make(chan int64)
	g := &errgroup.Group{}
	g.Go(func() error {
		return s.tickRestarts(ctx, restartC, tickC)
	})
	err := s.monitorRestarts(ctx, restartC, 2, 6)
	if err := g.Wait(); err != nil {
		s.Log.WarnContext(ctx, "Unable to determine last restart time. Failed to monitor for crash loops.", errorKey, err)
	}
	return trace.Wrap(err)
}

func (s SystemdService) monitorRestarts(ctx context.Context, timeCh <-chan int64, maxStops, minClean int) error {
	var (
		clean, stops int
		restartTime  int64
	)
	// TODO: thread init value of restartTime
	for {
		// wait first to ensure we initial stop has completed
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-timeCh:
			if t != restartTime {
				clean = 0
				restartTime = t
				stops++
			} else {
				clean++
			}
		}
		switch {
		case stops > maxStops:
			return trace.Errorf("detected crash loop")
		case clean >= minClean:
			return nil
		}
	}
}

func (s SystemdService) tickRestarts(ctx context.Context, ch chan<- int64, tickC <-chan time.Time) error {
	var err error
	for {
		// two select statements -> never skip restarts
		select {
		case <-tickC:
		case <-ctx.Done():
			return err
		}
		var t int64
		t, err = s.getRestartTime()
		select {
		case ch <- t:
		case <-ctx.Done():
			return err
		}
	}
}

func (s SystemdService) getRestartTime() (int64, error) {
	b, err := os.ReadFile(s.LastRestartPath)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	restart, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, 64)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return restart, nil
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
