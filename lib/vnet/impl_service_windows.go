package vnet

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

// ServiceHandler abstracts the core service workload behind a Windows service.
type ServiceHandler interface {
	Run(ctx context.Context, args []string) error
}

// WindowsServiceRunner wires a handler into the Windows service lifecycle.
type WindowsServiceRunner struct {
	handler ServiceHandler
}

func setupServiceLogger(source string) (func() error, error) {
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

// RunWindowsServiceMain wires logging, runs the service, and closes logging resources.
func RunWindowsServiceMain(
	serviceName string,
	handler ServiceHandler,
	loggerName string,
) error {
	closeFn, err := setupServiceLogger(loggerName)
	if err != nil {
		return trace.Wrap(err, "setting up logger for service")
	}

	if err := svc.Run(serviceName, &WindowsServiceRunner{
		handler: handler,
	}); err != nil {
		closeFn()
		return trace.Wrap(err, "running Windows service")
	}

	return trace.Wrap(closeFn(), "closing logger")
}

// Execute implements [svc.Handler.Execute].
func (s *WindowsServiceRunner) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	logger := slog.With(teleport.ComponentKey, teleport.Component("vnet", "windows-service"))
	const cmdsAccepted = svc.AcceptStop // Interrogate is always accepted and there is no const for it.
	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.handler.Run(ctx, args)
	}()

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
				logger.InfoContext(ctx, "Received stop command, shutting down service")
				// Cancel the context passed to s.run to terminate the
				// networking stack.
				cancel()
				terminateTimedOut = cmp.Or(terminateTimedOut, time.After(terminateTimeout))
				status <- svc.Status{State: svc.StopPending}
			}
		case <-terminateTimedOut:
			logger.ErrorContext(ctx, "Networking stack failed to terminate within timeout, exiting process",
				slog.Duration("timeout", terminateTimeout))
			exitCode = 1
			break loop
		case err := <-errCh:
			if err == nil || errors.Is(err, context.Canceled) {
				logger.InfoContext(ctx, "Service terminated")
			} else {
				logger.ErrorContext(ctx, "Service terminated", "error", err)
				exitCode = 1
			}
			break loop
		}
	}
	status <- svc.Status{State: svc.Stopped, Win32ExitCode: exitCode}
	return false, exitCode
}
