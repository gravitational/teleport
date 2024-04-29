/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// fastShutdownTimeout is how long we're going to wait before connections
	// are forcibly terminated during a fast shutdown.
	fastShutdownTimeout = time.Second * 3

	// fastShutdownGrace is how long we're going to wait for the shutdown
	// procedure to complete after the fastShutdownTimeout is hit.
	fastShutdownGrace = time.Second * 2
)

// printShutdownStatus prints running services until shut down
func (process *TeleportProcess) printShutdownStatus(ctx context.Context) {
	statusInterval := defaults.HighResPollingPeriod
	t := time.NewTimer(statusInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			statusInterval = min(statusInterval*2, defaults.LowResPollingPeriod)
			t.Reset(statusInterval)
			process.logger.InfoContext(process.ExitContext(), "Waiting for services to finish.", "services", process.Supervisor.Services())
		}
	}
}

// WaitForSignals waits for system signals and processes them.
// Should not be called twice by the process.
func (process *TeleportProcess) WaitForSignals(ctx context.Context) error {
	sigC := make(chan os.Signal, 1024)
	// Note: SIGKILL can't be trapped.
	signal.Notify(sigC,
		syscall.SIGQUIT, // graceful shutdown
		syscall.SIGTERM, // fast shutdown
		syscall.SIGINT,  // fast shutdown
		syscall.SIGUSR1, // log process diagnostic info
		syscall.SIGUSR2, // initiate process restart procedure
		syscall.SIGHUP,  // graceful restart procedure
	)
	defer signal.Stop(sigC)

	serviceErrorsC := make(chan Event, 10)
	eventCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	process.ListenForEvents(eventCtx, ServiceExitedWithErrorEvent, serviceErrorsC)

	// Block until a signal is received or handler got an error.
	// Notice how this handler is serialized - it will only receive
	// signals in sequence and will not run in parallel.
	for {
		select {
		case signal := <-sigC:
			switch signal {
			case syscall.SIGQUIT:
				timeoutCtx, cancel := context.WithTimeout(ctx, apidefaults.MaxCertDuration)
				defer cancel()
				process.Shutdown(timeoutCtx)
				process.logger.InfoContext(process.ExitContext(), "All services stopped, exiting.")
				return nil
			case syscall.SIGTERM, syscall.SIGINT:
				process.logger.InfoContext(process.ExitContext(), "Got shutdown signal, exiting within timeout.", "signal", signal, "timeout", fastShutdownTimeout.Seconds())
				// we run the shutdown in a goroutine so we can return and exit
				// the process even if Shutdown takes longer to return than we
				// expected (due to bugs, for example)
				shutdownDone := make(chan struct{})
				go func() {
					defer close(shutdownDone)
					timeoutCtx, cancel := context.WithTimeout(ctx, fastShutdownTimeout)
					defer cancel()
					process.Shutdown(timeoutCtx)
				}()
				graceTimer := time.NewTimer(fastShutdownTimeout + fastShutdownGrace)
				defer graceTimer.Stop()
				select {
				case <-graceTimer.C:
					process.logger.WarnContext(process.ExitContext(), "Shutdown still hasn't completed, exiting anyway.")
				case <-shutdownDone:
					process.logger.InfoContext(process.ExitContext(), "All services stopped or timeout passed, exiting immediately.")
				}
				return nil
			case syscall.SIGUSR1:
				// All programs placed diagnostics on the standard output.
				// This had always caused trouble when the output was redirected into a file, but became intolerable
				// when the output was sent to an unsuspecting process.
				// Nevertheless, unwilling to violate the simplicity of the standard-input-standard-output model,
				// people tolerated this state of affairs through v6. Shortly thereafter Dennis Ritchie cut the Gordian
				// knot by introducing the standard error file.
				// That was not quite enough. With pipelines diagnostics could come from any of several programs running simultaneously.
				// Diagnostics needed to identify themselves.
				// - Doug McIllroy, "A Research UNIX Reader: Annotated Excerpts from the Programmer’s Manual, 1971-1986"
				process.logger.InfoContext(process.ExitContext(), "Got signal SIGUSR1, logging diagnostic info to stderr.")
				writeDebugInfo(os.Stderr)
			case syscall.SIGUSR2:
				process.logger.InfoContext(process.ExitContext(), "Got signal SIGUSR2, forking a new process.")
				if err := process.forkChild(); err != nil {
					process.logger.WarnContext(process.ExitContext(), "Failed to fork process", "error", err)
				} else {
					process.logger.InfoContext(process.ExitContext(), "Successfully started new process.")
				}
			case syscall.SIGHUP:
				process.logger.InfoContext(process.ExitContext(), "Got signal SIGHUP, performing graceful restart.")
				if err := process.forkChild(); err != nil {
					process.logger.WarnContext(process.ExitContext(), "Failed to fork process", "error", err)
					continue
				}
				process.logger.InfoContext(process.ExitContext(), "Successfully started new process, shutting down gracefully.")
				timeoutCtx, cancel := context.WithTimeout(ctx, apidefaults.MaxCertDuration)
				defer cancel()
				process.Shutdown(timeoutCtx)
				process.logger.InfoContext(process.ExitContext(), "All services stopped, exiting.")
				return nil
			default:
				process.logger.InfoContext(process.ExitContext(), "Ignoring unknown signal.", "signal", signal)
			}
		case <-process.ReloadContext().Done():
			// it's fine to signal.Stop the same channel multiple times, and
			// after the function returns we're guaranteed to have restored the
			// default handlers for the signals and that no more signals are
			// pushed into the channel
			signal.Stop(sigC)
			if len(sigC) > 0 {
				// exhaust all signals before the internal reload, so we don't
				// miss signals to exit or to graceful restart instead
				continue
			}
			process.logger.InfoContext(process.ExitContext(), "Exiting signal handler: process has started internal reload.")
			return ErrTeleportReloading
		case <-process.ExitContext().Done():
			process.logger.InfoContext(process.ExitContext(), "Someone else has closed context, exiting.")
			return nil
		case <-ctx.Done():
			process.Close()
			if err := process.Wait(); err != nil {
				process.logger.WarnContext(process.ExitContext(), "Error waiting for all services to exit", "error", err)
			}
			process.logger.InfoContext(process.ExitContext(), "Got request to shutdown, context is closing")
			return nil
		case event := <-serviceErrorsC:
			se, ok := event.Payload.(ExitEventPayload)
			if !ok {
				process.logger.WarnContext(process.ExitContext(), "Failed to decode service exit event", "payload", event.Payload)
				continue
			}
			if se.Service.IsCritical() {
				process.logger.ErrorContext(process.ExitContext(), "Critical service has exited with error, aborting.", "service", se.Service, "error", se.Error)
				if err := process.Close(); err != nil {
					process.logger.ErrorContext(process.ExitContext(), "Error when shutting down teleport.", "error", err)
				}
				return trace.Wrap(se.Error)
			}
			process.logger.WarnContext(process.ExitContext(), "Non-critical service has exited with error , continuing to operate.", "service", se.Service, "error", se.Error)
		}
	}
}

// ErrTeleportReloading is returned when signal waiter exits
// because the teleport process has initiaded shutdown
var ErrTeleportReloading = &trace.CompareFailedError{Message: "teleport process is reloading"}

// ErrTeleportExited means that teleport has exited
var ErrTeleportExited = &trace.CompareFailedError{Message: "teleport process has shutdown"}

func (process *TeleportProcess) writeToSignalPipe(signalPipe *os.File, message string) error {
	messageSignalled, cancel := context.WithCancel(context.Background())
	// Below the cancel is called second time, but it's ok.
	// After the first call, subsequent calls to a CancelFunc do nothing.
	defer cancel()
	go func() {
		_, err := signalPipe.Write([]byte(message))
		if err != nil {
			process.logger.DebugContext(process.ExitContext(), "Failed to write to pipe.", "error", trace.DebugReport(err))
			return
		}
		cancel()
	}()

	select {
	case <-time.After(signalPipeTimeout):
		return trace.BadParameter("Failed to write to parent process pipe.")
	case <-messageSignalled.Done():
		process.logger.InfoContext(process.ExitContext(), "Signaled success to parent process.")
	}
	return nil
}

// closeImportedDescriptors closes imported but unused file descriptors,
// what could happen if service has updated configuration
func (process *TeleportProcess) closeImportedDescriptors(prefix string) error {
	process.Lock()
	defer process.Unlock()

	var errors []error
	openDescriptors := make([]*servicecfg.FileDescriptor, 0, len(process.importedDescriptors))
	for _, d := range process.importedDescriptors {
		if strings.HasPrefix(d.Type, prefix) {
			process.logger.InfoContext(process.ExitContext(), "Closing imported but unused descriptor.", "type", d.Type, "address", d.Address)
			errors = append(errors, d.Close())
		} else {
			openDescriptors = append(openDescriptors, d)
		}
	}
	process.importedDescriptors = openDescriptors
	return trace.NewAggregate(errors...)
}

// importOrCreateListener imports listener passed by the parent process (happens during live reload)
// or creates a new listener if there was no listener registered
func (process *TeleportProcess) importOrCreateListener(typ ListenerType, address string) (net.Listener, error) {
	l, err := process.importListener(typ, address)
	if err == nil {
		process.logger.InfoContext(process.ExitContext(), "Using file descriptor passed by the parent process.", "type", typ, "address", address)
		return l, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	process.logger.InfoContext(process.ExitContext(), "Service is creating new listener.", "type", typ, "address", address)
	return process.createListener(typ, address)
}

func (process *TeleportProcess) importSignalPipe() (*os.File, error) {
	process.Lock()
	defer process.Unlock()

	for i, d := range process.importedDescriptors {
		if d.Type == signalPipeName {
			process.importedDescriptors[i] = process.importedDescriptors[len(process.importedDescriptors)-1]
			process.importedDescriptors = process.importedDescriptors[:len(process.importedDescriptors)-1]
			return d.File, nil
		}
	}

	return nil, trace.NotFound("no file descriptor %v was found", signalPipeName)
}

// importListener imports listener passed by the parent process, if no listener is found
// returns NotFound, otherwise removes the file from the list
func (process *TeleportProcess) importListener(typ ListenerType, address string) (net.Listener, error) {
	process.Lock()
	defer process.Unlock()

	for i, d := range process.importedDescriptors {
		if d.Type == string(typ) && d.Address == address {
			listener, err := d.ToListener()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			process.importedDescriptors[i] = process.importedDescriptors[len(process.importedDescriptors)-1]
			process.importedDescriptors = process.importedDescriptors[:len(process.importedDescriptors)-1]
			r := registeredListener{typ: typ, address: address, listener: listener}
			process.registeredListeners = append(process.registeredListeners, r)
			return listener, nil
		}
	}

	return nil, trace.NotFound("no file descriptor for type %v and address %v has been imported", typ, address)
}

// createListener creates listener and adds to a list of tracked listeners
func (process *TeleportProcess) createListener(typ ListenerType, address string) (net.Listener, error) {
	listenersClosed := func() bool {
		process.Lock()
		defer process.Unlock()
		return process.listenersClosed
	}

	if listenersClosed() {
		process.logger.DebugContext(process.ExitContext(), "Listening is blocked, not opening listener.", "type", typ, "address", address)
		return nil, trace.BadParameter("listening is blocked")
	}

	// When the process exists, the socket files are left behind (to cover
	// forking scenarios). To guarantee there won't be errors like "address
	// already in use", delete the file before starting the listener.
	if typ.Network() == "unix" {
		process.logger.DebugContext(process.ExitContext(), "Deleting socket file", "path", address)
		if err := trace.ConvertSystemError(os.Remove(address)); !trace.IsNotFound(err) {
			warnOnErr(process.ExitContext(), err, process.logger)
		}
	}

	listener, err := net.Listen(typ.Network(), address)
	if err != nil {
		process.Lock()
		listener, ok := process.getListenerNeedsLock(typ, address)
		process.Unlock()
		if ok {
			process.logger.DebugContext(process.ExitContext(), "Using existing listener.", "type", typ, "address", address)
			return listener, nil
		}
		return nil, trace.Wrap(err)
	}

	// The default behavior for unix listeners is to delete the file when the
	// listener closes (unlinking). However, if the process forks, the file
	// descriptor will be gone when its parent process exists, causing the new
	// listener to have no socket file.
	if unixListener, ok := listener.(*net.UnixListener); ok {
		unixListener.SetUnlinkOnClose(false)
	}

	process.Lock()
	defer process.Unlock()
	// check this again in case we stopped allowing new listeners halfway
	// through the net.Listen (which can block, if the address is a hostname and
	// needs a dns lookup, so we can't do it while holding the lock)
	if process.listenersClosed {
		listener.Close()
		process.logger.DebugContext(process.ExitContext(), "Listening is blocked, closing newly-created listener.", "type", typ, "address", address)
		return nil, trace.BadParameter("listening is blocked")
	}
	if l, ok := process.getListenerNeedsLock(typ, address); ok {
		listener.Close()
		process.logger.DebugContext(process.ExitContext(), "Using existing listener.", "type", typ, "address", address)
		return l, nil
	}
	r := registeredListener{typ: typ, address: address, listener: listener}
	process.registeredListeners = append(process.registeredListeners, r)
	return listener, nil
}

// getListenerNeedsLock tries to get an existing listener that matches the type/addr.
func (process *TeleportProcess) getListenerNeedsLock(typ ListenerType, address string) (listener net.Listener, ok bool) {
	for _, l := range process.registeredListeners {
		if l.typ == typ && l.address == address {
			return l.listener, true
		}
	}
	return nil, false
}

func (process *TeleportProcess) stopListeners() error {
	process.Lock()
	defer process.Unlock()
	process.listenersClosed = true
	errors := make([]error, 0, len(process.registeredListeners))
	for _, r := range process.registeredListeners {
		errors = append(errors, r.listener.Close())
	}
	process.registeredListeners = nil
	return trace.NewAggregate(errors...)
}

// ExportFileDescriptors exports file descriptors to be passed to child process
func (process *TeleportProcess) ExportFileDescriptors() ([]*servicecfg.FileDescriptor, error) {
	var out []*servicecfg.FileDescriptor
	process.Lock()
	defer process.Unlock()
	for _, r := range process.registeredListeners {
		file, err := utils.GetListenerFile(r.listener)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, &servicecfg.FileDescriptor{
			File:    file,
			Type:    string(r.typ),
			Address: r.address,
		})
	}
	return out, nil
}

// importFileDescriptors imports file descriptors from environment if there are any
func importFileDescriptors(log logrus.FieldLogger) ([]*servicecfg.FileDescriptor, error) {
	// These files may be passed in by the parent process
	filesString := os.Getenv(teleportFilesEnvVar)
	os.Unsetenv(teleportFilesEnvVar)
	if filesString == "" {
		return nil, nil
	}

	files, err := filesFromString(filesString)
	if err != nil {
		return nil, trace.BadParameter("child process has failed to read files, error %q", err)
	}

	if len(files) != 0 {
		log.Infof("Child has been passed files: %v", files)
	}

	return files, nil
}

// registeredListener is a listener registered
// within teleport process, can be passed to child process
type registeredListener struct {
	// Type is a listener type, e.g. auth:ssh
	typ ListenerType
	// Address is an address listener is serving on, e.g. 127.0.0.1:3025
	address string
	// Listener is a file listener object
	listener net.Listener
}

const teleportFilesEnvVar = "TELEPORT_OS_FILES"

func execPath() (string, error) {
	name, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	if _, err = os.Stat(name); nil != err {
		return "", err
	}
	return name, err
}

const (
	signalPipeName = "teleport-signal-pipe"
	// signalPipeTimeout is a time parent process is expecting
	// the child process to initialize and write back,
	// or child process is blocked on write to the pipe
	signalPipeTimeout = 2 * time.Minute
)

type fileDescriptor struct {
	Address  string `json:"addr"`
	Type     string `json:"type"`
	FileFD   int    `json:"fd"`
	FileName string `json:"fileName"`
}

// filesToString serializes file descriptors as well as accompanying information (like socket host and port)
func filesToString(files []*servicecfg.FileDescriptor) (string, error) {
	out := make([]fileDescriptor, len(files))
	for i, f := range files {
		out[i] = fileDescriptor{
			// Once files will be passed to the child process and their FDs will change.
			// The first three passed files are stdin, stdout and stderr, every next file will have the index + 3
			// That's why we rearrange the FDs for child processes to get the correct file descriptors.
			FileFD:   i + 3,
			FileName: f.File.Name(),
			Address:  f.Address,
			Type:     f.Type,
		}
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// filesFromString de-serializes the file descriptors and turns them in the os.Files
func filesFromString(in string) ([]*servicecfg.FileDescriptor, error) {
	var out []fileDescriptor
	if err := json.Unmarshal([]byte(in), &out); err != nil {
		return nil, err
	}
	files := make([]*servicecfg.FileDescriptor, len(out))
	for i, o := range out {
		files[i] = &servicecfg.FileDescriptor{
			File:    os.NewFile(uintptr(o.FileFD), o.FileName),
			Address: o.Address,
			Type:    o.Type,
		}
	}
	return files, nil
}

func (process *TeleportProcess) forkChild() error {
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer readPipe.Close()
	defer writePipe.Close()

	path, err := execPath()
	if err != nil {
		return trace.Wrap(err)
	}

	workingDir, err := os.Getwd()
	if nil != err {
		return err
	}

	logger := process.logger.With("path", path, "working_dir", workingDir)

	logger.InfoContext(process.ExitContext(), "Forking child.")

	listenerFiles, err := process.ExportFileDescriptors()
	if err != nil {
		return trace.Wrap(err)
	}

	listenerFiles = append(listenerFiles, &servicecfg.FileDescriptor{
		File:    writePipe,
		Type:    signalPipeName,
		Address: "127.0.0.1:0",
	})

	// These files will be passed to the child process
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	for _, f := range listenerFiles {
		files = append(files, f.File)
	}

	// Serialize files to JSON string representation
	vals, err := filesToString(listenerFiles)
	if err != nil {
		return trace.Wrap(err)
	}

	logger.InfoContext(process.ExitContext(), "Passing files to child", "files", vals)
	env := append(os.Environ(), fmt.Sprintf("%s=%s", teleportFilesEnvVar, vals))

	p, err := os.StartProcess(path, os.Args, &os.ProcAttr{
		Dir:   workingDir,
		Env:   env,
		Files: files,
		Sys:   &syscall.SysProcAttr{},
	})
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	logger.InfoContext(process.ExitContext(), "Forked new child process.", "pid", p.Pid)
	logger = process.logger.With("pid", p.Pid)

	process.forkedTeleportCount.Add(1)
	go func() {
		defer process.forkedTeleportCount.Add(-1)
		state, err := p.Wait()
		if err != nil {
			logger.ErrorContext(process.ExitContext(), "Failed waiting for forked Teleport process.", "error", err)
			return
		}
		logger.WarnContext(process.ExitContext(), "Forked Teleport process has exited.", "status", state.String())
	}()

	_ = writePipe.Close()
	readPipe.SetReadDeadline(time.Now().Add(signalPipeTimeout))
	buf := make([]byte, 1024)
	// we require at least one byte from the child, otherwise we can't
	// distinguish the child dying (and closing the pipe) and a deliberate close
	// without data; conversely, we don't care if we get an I/O or timeout error
	// if we know that the child has sent at least one byte
	n, err := io.ReadAtLeast(readPipe, buf, 1)
	if err != nil {
		return trace.Wrap(err, "waiting for forked Teleport process to signal successful start")
	}
	logger.InfoContext(process.ExitContext(), "Forked Teleport process signaled successful start.", "data", string(buf[:n]))

	return nil
}
