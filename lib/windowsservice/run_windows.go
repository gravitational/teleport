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

package windowsservice

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/svc"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const defaultTerminateTimeout = 30 * time.Second

// ServiceHandler abstracts the core service workload behind a Windows service.
type ServiceHandler interface {
	// Execute will be called by the package code at the start of
	// the service, and the service will exit once Execute completes.
	Execute(ctx context.Context, args []string) error
}

// RunConfig defines the inputs for running a Windows service.
type RunConfig struct {
	// Name is the Windows service name registered with the SCM.
	Name string
	// Handler runs the service workload.
	Handler ServiceHandler
	// Logger is logger for the service.
	Logger *slog.Logger
	// TerminateTimeout bounds how long the service waits for shutdown.
	// If zero, a default timeout is used.
	TerminateTimeout time.Duration
}

// runner wires a handler into the Windows service lifecycle.
type runner struct {
	handler          ServiceHandler
	logger           *slog.Logger
	terminateTimeout time.Duration
}

// InitSlogEventLogger sets up a new slog handler that writes to the Windows Event Log as source.
func InitSlogEventLogger(source string) (func() error, error) {
	level := slog.LevelInfo
	if envVar := os.Getenv(teleport.VerboseLogsEnvVar); envVar != "" {
		isDebug, err := strconv.ParseBool(envVar)
		if err != nil {
			return nil, trace.Wrap(err, "parsing %s", teleport.VerboseLogsEnvVar)
		}
		if isDebug {
			level = slog.LevelDebug
		}
	}

	handler, close, err := logutils.NewSlogEventLogHandler(source, level)
	if err != nil {
		return nil, trace.Wrap(err, "initializing log handler")
	}
	slog.SetDefault(slog.New(handler))
	return close, nil
}

// Run wires logging, runs the service, and closes logging resources.
func Run(cfg *RunConfig) error {
	if cfg.Name == "" {
		return trace.BadParameter("service name is required")
	}
	if cfg.Handler == nil {
		return trace.BadParameter("handler is required")
	}

	terminateTimeout := cfg.TerminateTimeout
	if terminateTimeout == 0 {
		terminateTimeout = defaultTerminateTimeout
	}

	err := svc.Run(cfg.Name, &runner{
		handler:          cfg.Handler,
		logger:           cfg.Logger,
		terminateTimeout: terminateTimeout,
	})
	return trace.Wrap(err, "running Windows service")
}

// Execute implements [svc.Handler.Execute].
func (s *runner) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop // Interrogate is always accepted and there is no const for it.
	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error)
	go func() { errCh <- s.handler.Execute(ctx, args) }()

	var terminateTimedOut <-chan time.Time
loop:
	for {
		select {
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				state := svc.Running
				if ctx.Err() != nil {
					state = svc.StopPending
				}
				status <- svc.Status{State: state, Accepts: cmdsAccepted}
			case svc.Stop:
				s.logger.InfoContext(ctx, "Received stop command, shutting down service")
				// Cancel the context passed to s.handler.Execute to terminate the service.
				cancel()
				terminateTimedOut = cmp.Or(terminateTimedOut, time.After(s.terminateTimeout))
				status <- svc.Status{State: svc.StopPending}
			}
		case <-terminateTimedOut:
			s.logger.ErrorContext(ctx, "Service failed to terminate within timeout, exiting process",
				slog.Duration("timeout", s.terminateTimeout))
			exitCode = 1
			break loop
		case err := <-errCh:
			if err == nil || errors.Is(err, context.Canceled) {
				s.logger.InfoContext(ctx, "Service terminated")
			} else {
				s.logger.ErrorContext(ctx, "Service terminated", "error", err)
				exitCode = 1
			}
			break loop
		}
	}
	status <- svc.Status{State: svc.Stopped, Win32ExitCode: exitCode}
	return false, exitCode
}
