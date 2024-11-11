package servicemini

import (
	"context"
	"io"
	"os"
	"os/signal"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/trace"
)

// NewProcess is a function that creates new teleport from config
type NewProcess func(cfg *servicecfg.Config) (Process, error)

func newTeleportProcess(cfg *servicecfg.Config) (Process, error) {
	return NewTeleportMini(cfg)
}

// Run installs a signal handler for relevant control signals, starts the
// Teleport process and waits for signals to terminate it or trigger a fork. It
// will also close the process if a critical service exits with an error. The
// process will be closed when the context is done.
func Run(ctx context.Context, cfg servicecfg.Config, newTeleport NewProcess) error {
	sigC := make(chan os.Signal, 1024)
	// this should happen before the very first newTeleport, as that's the point
	// where we MUST handle all the relevant OS signals
	signal.Notify(sigC, teleportSignals...)
	defer signal.Stop(sigC)

	return trace.Wrap(RunWithSignalChannel(ctx, cfg, newTeleport, sigC))
}

// RunWithSignalChannel starts the Teleport process and waits for signals to
// terminate it or trigger a fork. It will also close the process if a critical
// service exits with an error. The process will be closed when the context is
// done.
func RunWithSignalChannel(ctx context.Context, cfg servicecfg.Config, newTeleport NewProcess, sigC <-chan os.Signal) error {
	if newTeleport == nil {
		newTeleport = newTeleportProcess
	}
	copyCfg := cfg
	srv, err := newTeleport(&copyCfg)
	if err != nil {
		return trace.Wrap(err, "initialization failed")
	}
	if srv == nil {
		return trace.BadParameter("process has returned nil server")
	}
	if err := srv.Start(); err != nil {
		return trace.Wrap(err, "startup failed")
	}
	return trace.Wrap(srv.WaitForSignals(ctx, sigC))
}

// Process is a interface for processes
type Process interface {
	// Closer closes all resources used by the process
	io.Closer
	// Start starts the process in a non-blocking way
	Start() error
	// WaitForSignals waits for and handles system process signals.
	WaitForSignals(context.Context, <-chan os.Signal) error
	// ExportFileDescriptors exports service listeners
	// file descriptors used by the process.
	ExportFileDescriptors() ([]*servicecfg.FileDescriptor, error)
	// Shutdown starts graceful shutdown of the process,
	// blocks until all resources are freed and go-routines are
	// shut down.
	Shutdown(context.Context)
	// WaitForEvent waits for one event with the specified name (returns the
	// latest such event if at least one has been broadcasted already, ignoring
	// the context). Returns an error if the context is canceled before an event
	// is received.
	WaitForEvent(ctx context.Context, name string) (Event, error)
	// WaitWithContext waits for the service to stop. This is a blocking
	// function.
	WaitWithContext(ctx context.Context)
}
