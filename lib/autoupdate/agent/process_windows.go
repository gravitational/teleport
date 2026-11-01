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
	"cmp"
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// SCM process monitoring consts
const (
	// scmMonitorTimeout is the timeout for determining whether a Windows service has started.
	// Windows Desktop Service startup (including LDAP discovery) can be slower than SSH,
	// so this is longer than monitorTimeout used on Linux.
	scmMonitorTimeout = 2 * time.Minute
	// scmStopTimeout is the timeout for a Windows service to reach the Stopped state after
	// a Stop control has been issued.
	scmStopTimeout = 2 * time.Minute
)

// SCM access masks
const (
	// scmReadAccess is sufficient for querying service status and configuration.
	// It does not require an elevated token.
	scmReadAccess = windows.SERVICE_QUERY_STATUS | windows.SERVICE_QUERY_CONFIG
	// scmWriteAccess is required for starting, stopping, reconfiguring, and deleting services.
	scmWriteAccess = windows.SERVICE_ALL_ACCESS | windows.DELETE
)

// hasServiceManager returns true, Windows always have service manager
func hasServiceManager() (bool, error) {
	return true, nil
}

type unsupportedProcess struct {
	name string
}

func (p unsupportedProcess) Name() string                       { return p.name }
func (p unsupportedProcess) Reload(context.Context, bool) error { return nil }
func (p unsupportedProcess) Sync(context.Context) error         { return nil }
func (p unsupportedProcess) IsEnabled(context.Context) (bool, error) {
	return false, trace.Wrap(ErrNotSupported)
}
func (p unsupportedProcess) IsActive(context.Context) (bool, error) {
	return false, trace.Wrap(ErrNotSupported)
}
func (p unsupportedProcess) IsPresent(context.Context) (bool, error) {
	return false, trace.Wrap(ErrNotSupported)
}

type scmService struct {
	// serviceName specifies the Windows service name.
	serviceName string
	// ready is a readiness checker.
	ready ReadyChecker
	// log contains a logger.
	log *slog.Logger

	// pollInterval overrides the interval used to poll service status.
	// Defaults to monitorInterval if zero.
	pollInterval time.Duration
}

func (s *scmService) openService(access uint32) (*mgr.Service, error) {
	return openService(s.serviceName, access)
}

// openService opens the named Windows service with the requested access rights.
// The returned service must be closed by the caller.
func openService(name string, access uint32) (*mgr.Service, error) {
	// Avoid mgr.Connect, which requests SC_MANAGER_ALL_ACCESS and therefore requires elevation,
	// so that read-only operations (e.g., teleport-update status) work for regular users.
	m, err := windows.OpenSCManager(nil /*machine*/, nil /*database*/, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to Windows service manager")
	}
	defer windows.CloseServiceHandle(m)
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, trace.Wrap(err, "converting service name to UTF16")
	}
	h, err := windows.OpenService(m, namePtr, access)
	if err != nil {
		return nil, trace.Wrap(err, "opening Windows service %s", name)
	}
	return &mgr.Service{Name: name, Handle: h}, nil
}

func isServiceNotFound(err error) bool {
	return errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST)
}

func (s *scmService) Name() string {
	return s.serviceName
}

// stop the service and wait for it to reach the Stopped state.
func (s *scmService) stop(ctx context.Context, service *mgr.Service) error {
	status, err := service.Control(svc.Stop)
	if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}
	if status.State == svc.Stopped {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, scmStopTimeout)
	defer cancel()
	ticker := time.NewTicker(cmp.Or(s.pollInterval, monitorInterval))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "timed out waiting for Windows service to stop")
		case <-ticker.C:
		}
		status, err := service.Query()
		if err != nil {
			return trace.Wrap(err)
		}
		if status.State == svc.Stopped {
			return nil
		}
	}
}

func (s *scmService) Reload(ctx context.Context, forceStart bool) error {
	service, err := s.openService(scmWriteAccess)
	if isServiceNotFound(err) {
		s.log.WarnContext(ctx, "Windows service not installed.", unitKey, s.serviceName)
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return trace.Wrap(err, "unable to determine if Windows service is running")
	}
	if status.State != svc.Running && !forceStart {
		s.log.WarnContext(ctx, "Windows service not running.", unitKey, s.serviceName)
		return nil
	}
	initialPID := int(status.ProcessId)

	if initialPID == 0 && forceStart {
		// set initial PID to non-zero so we will start monitoring it
		initialPID = -1
	}

	config, err := service.Config()
	if err != nil {
		return trace.Wrap(err, "unable to determine Windows service configuration")
	}
	if config.StartType == mgr.StartDisabled {
		// service was disabled without stopping, to restart it we have to temporarily
		// set start type to StartManual and restore StartDisabled afterward.
		config.StartType = mgr.StartManual
		if err := service.UpdateConfig(config); err != nil {
			return trace.Wrap(err, "unable to update Windows service configuration")
		}
		defer func() {
			config.StartType = mgr.StartDisabled
			if err := service.UpdateConfig(config); err != nil {
				s.log.WarnContext(ctx, "unable to update Windows service configuration, it will be left in manual start mode")
			}
		}()
	}

	if err := s.stop(ctx, service); err != nil {
		return trace.Wrap(err, "stop Windows service")
	}
	if err := service.Start(); err != nil {
		return trace.Wrap(err, "start Windows service")
	}
	s.log.WarnContext(ctx, "Service restarted. Connections potentially dropped.", unitKey, s.serviceName)

	// monitor logs all relevant errors, so we filter for a few outcomes
	err = s.monitor(ctx, service, initialPID)
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return trace.Wrap(err)
	}
	if err != nil {
		return trace.Wrap(err, "monitor process")
	}
	return nil
}

// monitor for a started, healthy process by polling the service status and the readiness socket.
// The service's reported process ID is fed into the shared crash-detection logic:
// a PID of 0 (service not running) or a changed PID (restarted by SCM recovery) counts as a crash.
func (s *scmService) monitor(ctx context.Context, service *mgr.Service, initPID int) error {
	m := processMonitor{
		name:     s.serviceName,
		ready:    s.ready,
		log:      s.log,
		timeout:  scmMonitorTimeout,
		interval: cmp.Or(s.pollInterval, monitorInterval),
		tickPID: func(ctx context.Context, pidC chan<- int, tickC <-chan time.Time) error {
			return tickServicePID(ctx, service, pidC, tickC)
		},
		verifyPID: processRunning,
	}
	return m.monitor(ctx, initPID)
}

func processRunning(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		// OpenProcess returns ERROR_INVALID_PARAMETER for PIDs that do not exist.
		return os.ErrProcessDone
	}
	if err != nil {
		return trace.Wrap(err, "opening process %d", pid)
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return trace.Wrap(err, "reading exit code of process %d", pid)
	}
	if code != stillActive {
		return os.ErrProcessDone
	}
	return nil
}

// stillActive is the exit code reported by GetExitCodeProcess for a running process (STILL_ACTIVE).
const stillActive = 259

// tickServicePID reads the current time on tickC, and outputs the service's process ID on ch for each received tick.
// If the service is not running or cannot be queried, tickServicePID sends 0 on ch.
// Any error from the last query is returned when ctx is canceled.
func tickServicePID(ctx context.Context, service *mgr.Service, ch chan<- int, tickC <-chan time.Time) error {
	var err error
	for {
		// two select statements -> never skip reads
		select {
		case <-tickC:
		case <-ctx.Done():
			return err
		}
		var pid int
		var status svc.Status
		status, err = service.Query()
		if err == nil {
			pid = int(status.ProcessId)
		}
		select {
		case ch <- pid:
		case <-ctx.Done():
			return err
		}
	}
}

func (s *scmService) Sync(ctx context.Context) error {
	// This function is no-op as Windows doesn't need any synchronization after updating service.
	// It sees updated state right away.
	return nil
}

// IsEnabled returns true if the service is configured to start automatically.
func (s *scmService) IsEnabled(ctx context.Context) (bool, error) {
	service, err := s.openService(scmReadAccess)
	if isServiceNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err, "unable to determine if Windows service %s is enabled", s.serviceName)
	}
	defer service.Close()
	cfg, err := service.Config()
	if err != nil {
		return false, trace.Wrap(err, "unable to determine if Windows service %s is enabled", s.serviceName)
	}
	return cfg.StartType == mgr.StartAutomatic, nil
}

// IsActive returns true if the service is running.
func (s *scmService) IsActive(ctx context.Context) (bool, error) {
	service, err := s.openService(scmReadAccess)
	if isServiceNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err, "unable to determine if Windows service %s is active", s.serviceName)
	}
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return false, trace.Wrap(err, "unable to determine if Windows service %s is active", s.serviceName)
	}
	return status.State == svc.Running, nil
}

// IsPresent returns true if the service is installed.
func (s *scmService) IsPresent(ctx context.Context) (bool, error) {
	service, err := s.openService(scmReadAccess)
	if isServiceNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err, "unable to determine if Windows service %s is present", s.serviceName)
	}
	service.Close()
	return true, nil
}
