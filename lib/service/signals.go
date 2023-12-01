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
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

// printShutdownStatus prints running services until shut down
func (process *TeleportProcess) printShutdownStatus(ctx context.Context) {
	t := time.NewTicker(defaults.HighResReportingPeriod)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			process.log.Infof("Waiting for services: %v to finish.", process.Supervisor.Services())
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
		syscall.SIGCHLD, // collect child status
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
				process.Shutdown(ctx)
				process.log.Infof("All services stopped, exiting.")
				return nil
			case syscall.SIGTERM, syscall.SIGINT:
				timeout := getShutdownTimeout(process.log)
				cancelCtx, cancelFunc := context.WithTimeout(ctx, timeout)
				process.log.Infof("Got signal %q, exiting within %vs.", signal, timeout.Seconds())
				go func() {
					defer cancelFunc()
					process.Shutdown(cancelCtx)
				}()
				<-cancelCtx.Done()
				process.log.Infof("All services stopped or timeout passed, exiting immediately.")
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
				process.log.Infof("Got signal %q, logging diagnostic info to stderr.", signal)
				writeDebugInfo(os.Stderr)
			case syscall.SIGUSR2:
				process.log.Infof("Got signal %q, forking a new process.", signal)
				if err := process.forkChild(); err != nil {
					process.log.Warningf("Failed to fork: %v", err)
				} else {
					process.log.Infof("Successfully started new process.")
				}
			case syscall.SIGHUP:
				process.log.Infof("Got signal %q, performing graceful restart.", signal)
				if err := process.forkChild(); err != nil {
					process.log.Warningf("Failed to fork: %v", err)
					continue
				}
				process.log.Infof("Successfully started new process, shutting down gracefully.")
				process.Shutdown(ctx)
				process.log.Infof("All services stopped, exiting.")
				return nil
			case syscall.SIGCHLD:
				process.collectStatuses()
			default:
				process.log.Infof("Ignoring %q.", signal)
			}
		case <-process.ReloadContext().Done():
			process.log.Infof("Exiting signal handler: process has started internal reload.")
			return ErrTeleportReloading
		case <-process.ExitContext().Done():
			process.log.Infof("Someone else has closed context, exiting.")
			return nil
		case <-ctx.Done():
			process.Close()
			if err := process.Wait(); err != nil {
				process.log.Warnf("Error waiting for all services to exit: %v", err)
			}
			process.log.Info("Got request to shutdown, context is closing")
			return nil
		case event := <-serviceErrorsC:
			se, ok := event.Payload.(ExitEventPayload)
			if !ok {
				process.log.Warningf("Failed to decode service exit event, %T", event.Payload)
				continue
			}
			if se.Service.IsCritical() {
				process.log.Errorf("Critical service %v has exited with error %v, aborting.", se.Service, se.Error)
				if err := process.Close(); err != nil {
					process.log.Errorf("Error when shutting down teleport %v.", err)
				}
				return trace.Wrap(se.Error)
			}
			process.log.Warningf("Non-critical service %v has exited with error %v, continuing to operate.", se.Service, se.Error)
		}
	}
}

const defaultShutdownTimeout = time.Second * 3
const maxShutdownTimeout = time.Minute * 10

func getShutdownTimeout(log logrus.FieldLogger) time.Duration {
	timeout := defaultShutdownTimeout

	// read undocumented env var TELEPORT_UNSTABLE_SHUTDOWN_TIMEOUT.
	// TODO(Tener): DELETE IN 15.0. after ironing out all possible shutdown bugs.
	override := os.Getenv("TELEPORT_UNSTABLE_SHUTDOWN_TIMEOUT")
	if override != "" {
		t, err := time.ParseDuration(override)
		if err != nil {
			log.Warnf("Cannot parse timeout override %q, using default instead.", override)
		}
		if err == nil {
			if t > maxShutdownTimeout {
				log.Warnf("Timeout override %q exceeds maximum value, reducing.", override)
				t = maxShutdownTimeout
			}
			timeout = t
		}
	}
	return timeout
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
			process.log.Debugf("Failed to write to pipe: %v.", trace.DebugReport(err))
			return
		}
		cancel()
	}()

	select {
	case <-time.After(signalPipeTimeout):
		return trace.BadParameter("Failed to write to parent process pipe.")
	case <-messageSignalled.Done():
		process.log.Infof("Signaled success to parent process.")
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
			process.log.Infof("Closing imported but unused descriptor %v %v.", d.Type, d.Address)
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
		process.log.Infof("Using file descriptor %v %v passed by the parent process.", typ, address)
		return l, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	process.log.Infof("Service %v is creating new listener on %v.", typ, address)
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
		process.log.Debugf("Listening is blocked, not opening listener for type %v and address %v.", typ, address)
		return nil, trace.BadParameter("listening is blocked")
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		process.Lock()
		listener, ok := process.getListenerNeedsLock(typ, address)
		process.Unlock()
		if ok {
			process.log.Debugf("Using existing listener for type %v and address %v.", typ, address)
			return listener, nil
		}
		return nil, trace.Wrap(err)
	}
	process.Lock()
	defer process.Unlock()
	// check this again in case we stopped allowing new listeners halfway
	// through the net.Listen (which can block, if the address is a hostname and
	// needs a dns lookup, so we can't do it while holding the lock)
	if process.listenersClosed {
		listener.Close()
		process.log.Debugf("Listening is blocked, closing newly-created listener for type %v and address %v.", typ, address)
		return nil, trace.BadParameter("listening is blocked")
	}
	if l, ok := process.getListenerNeedsLock(typ, address); ok {
		listener.Close()
		process.log.Debugf("Using existing listener for type %v and address %v.", typ, address)
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

	log := process.log.WithFields(logrus.Fields{"path": path, "workingDir": workingDir})

	log.Info("Forking child.")

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

	log.Infof("Passing %s to child", vals)
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
	process.pushForkedPID(p.Pid)
	log.WithFields(logrus.Fields{"pid": p.Pid}).Infof("Forked new child process.")

	messageReceived, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go func() {
		data := make([]byte, 1024)
		len, err := readPipe.Read(data)
		if err != nil {
			log.Debug("Failed to read from pipe")
			return
		}
		log.Infof("Received message from pid %v: %v", p.Pid, string(data[:len]))
		cancel()
	}()

	select {
	case <-time.After(signalPipeTimeout):
		return trace.BadParameter("Failed waiting from process")
	case <-messageReceived.Done():
		log.WithFields(logrus.Fields{"pid": p.Pid}).Infof("Child process signals success.")
	}

	return nil
}

// collectStatuses attempts to collect exit statuses from
// forked teleport child processes.
// If forked teleport process exited with an error during graceful
// restart, parent process has to collect the child process status
// otherwise the child process will become a zombie process.
// Call Wait4(-1) is trying to collect status of any child
// leads to warnings in logs, because other parts of the program could
// have tried to collect the status of this process.
// Instead this logic tries to collect statuses of the processes
// forked during restart procedure.
func (process *TeleportProcess) collectStatuses() {
	pids := process.getForkedPIDs()
	if len(pids) == 0 {
		return
	}
	for _, pid := range pids {
		var wait syscall.WaitStatus
		rpid, err := syscall.Wait4(pid, &wait, syscall.WNOHANG, nil)
		if err != nil {
			process.log.Errorf("Wait call failed: %v.", err)
			continue
		}
		if rpid == pid {
			process.popForkedPID(pid)
			process.log.Warningf("Forked teleport process %v has exited with status: %v.", pid, wait.ExitStatus())
		}
	}
}

func (process *TeleportProcess) pushForkedPID(pid int) {
	process.Lock()
	defer process.Unlock()
	process.forkedPIDs = append(process.forkedPIDs, pid)
}

func (process *TeleportProcess) popForkedPID(pid int) {
	process.Lock()
	defer process.Unlock()
	for i, p := range process.forkedPIDs {
		if p == pid {
			process.forkedPIDs = append(process.forkedPIDs[:i], process.forkedPIDs[i+1:]...)
			return
		}
	}
}

func (process *TeleportProcess) getForkedPIDs() []int {
	process.Lock()
	defer process.Unlock()
	if len(process.forkedPIDs) == 0 {
		return nil
	}
	out := make([]int, len(process.forkedPIDs))
	copy(out, process.forkedPIDs)
	return out
}
