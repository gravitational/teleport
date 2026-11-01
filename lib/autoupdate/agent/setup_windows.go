/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"

	libutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/eventlog"
)

const (
	defaultInstallDir = `C:\Program Files\Teleport`
	defaultPathDir    = defaultInstallDir + `\bin`
	// windowsTeleportServiceName is the name of the Windows service (SCM) running the Teleport agent.
	windowsTeleportServiceName = "Teleport"
	// UpdaterServiceName is the name of the Windows service (SCM) running the updater loop.
	UpdaterServiceName = "TeleportUpdater"
	// EventSourceName is the Windows Event Log source used by the updater service.
	EventSourceName = "teleport-update"
	// windowsEventMessageFile is the Event Log message file, shipped next to the updater in the install directory.
	windowsEventMessageFile = "msgfile.dll"
	// windowsServiceDescriptionPrefix marks Windows services that are managed by teleport-update.
	// It is the Windows equivalent of the systemd service file marker (see markerPrefix).
	windowsServiceDescriptionPrefix = "Teleport agent managed by teleport-update"
	// windowsServiceRecoveryDelay is the delay before SCM restarts a crashed service.
	windowsServiceRecoveryDelay = 5 * time.Second
	// windowsServiceRecoveryResetPeriod is the period after which the SCM failure count is reset.
	windowsServiceRecoveryResetPeriod = time.Hour
)

// NewNamespace validates and returns a Namespace.
// Install suffixes (namespaces) are not supported on Windows.
func NewNamespace(ctx context.Context, log *slog.Logger, name, installDir string) (ns *Namespace, err error) {
	defer func() { ns.overrideFromConfig(ctx) }()

	if name != "" {
		return nil, trace.BadParameter("install suffixes are not supported on Windows")
	}
	if installDir == "" {
		installDir = defaultInstallDir
	}
	return &Namespace{
		log:                log,
		name:               name,
		installDir:         installDir,
		defaultPathDir:     filepath.Join(installDir, "bin"),
		dataDir:            filepath.Join(installDir, "data"),
		teleportConfigFile: filepath.Join(installDir, "teleport.yaml"),
		updaterIDFile:      filepath.Join(installDir, strings.TrimSuffix(BinaryName, ".exe")+".id"),
	}, nil
}

// teleportServiceName returns the name of the Windows service running the Teleport agent.
// Namespaced installations are suffixed with the namespace name.
func (ns *Namespace) teleportServiceName() string {
	if ns.name == "" {
		return windowsTeleportServiceName
	}
	return windowsTeleportServiceName + "_" + ns.name
}

// windowsServiceDescription returns the description used to mark a Windows service
// as managed by teleport-update for the given revision.
func windowsServiceDescription(rev Revision) string {
	return windowsServiceDescriptionPrefix + " (revision " + rev.Dir() + ")"
}

// isManagedServiceDescription returns true if the Windows service description
// marks the service as managed by teleport-update.
func isManagedServiceDescription(description string) bool {
	return strings.HasPrefix(description, windowsServiceDescriptionPrefix)
}

// WriteTeleportService ensures that the Teleport Windows service exists and that
// the fields owned by teleport-update describe the given revision.
// This is the Windows equivalent of writing the systemd service file.
//
// If the service does not exist, it is created pointing at the versioned teleport.exe,
// running as LocalSystem, with crash recovery enabled. If teleport.yaml does not yet
// exist, the service is created disabled and left stopped. If teleport.yaml already
// exists, the service is created enabled (the equivalent of `systemctl enable --now`)
// and is started immediately, since setting StartType alone only takes effect on the
// next boot.
// If the service exists, only the binPath executable, description, and recovery actions
// are reconciled. Start type, account, dependencies, and environment are left untouched,
// and an existing service is never started, stopped, or restarted.
// Changing the binPath of a running service takes effect on its next start.
// A service that is not managed by the updater and still runs its own binary is never
// reconfigured; see updateTeleportService.
func (ns *Namespace) WriteTeleportService(ctx context.Context, pathDir string, rev Revision) error {
	// Point the service at the versioned binary rather than the pathDir junction,
	// so that os.Executable() of the agent stays under the versions directory
	// and so that the binPath matches what Link sets.
	exe := filepath.Join(ns.Dir(), versionsDirName, rev.Dir(), "bin", "teleport.exe")
	args := []string{"start", "--config", ns.teleportConfigFile}
	name := ns.teleportServiceName()

	m, err := connectServiceManager(windows.SC_MANAGER_CONNECT | windows.SC_MANAGER_CREATE_SERVICE)
	if err != nil {
		return trace.Wrap(err)
	}
	defer m.Disconnect()

	service, err := m.OpenService(name)
	if isServiceNotFound(err) {
		return trace.Wrap(ns.createTeleportService(ctx, m, name, exe, args, rev))
	}
	if err != nil {
		return trace.Wrap(err, "opening Windows service %s", name)
	}
	defer service.Close()
	return trace.Wrap(ns.updateTeleportService(ctx, service, exe, args, rev))
}

// createTeleportService creates the Teleport Windows service.
//
// If teleport.yaml is already present, the service is created with StartType
// Automatic and started immediately. Setting StartType alone does not start
// the service until the next boot, so without an explicit Start call here a
// freshly created service would be left stopped indefinitely.
func (ns *Namespace) createTeleportService(ctx context.Context, m *mgr.Mgr, name, exe string, args []string, rev Revision) error {
	startType := uint32(mgr.StartDisabled)
	if libutils.FileExists(ns.teleportConfigFile) {
		startType = mgr.StartAutomatic
	}
	service, err := m.CreateService(name, exe, mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    startType,
		ErrorControl: mgr.ErrorNormal,
		DisplayName:  name,
		Description:  windowsServiceDescription(rev),
		// ServiceStartName is left empty, which runs the service as LocalSystem.
	}, args...)
	if err != nil {
		return trace.Wrap(err, "creating Windows service %s", name)
	}
	defer service.Close()
	if err := setServiceRecovery(service); err != nil {
		return trace.Wrap(err)
	}
	if startType == mgr.StartDisabled {
		ns.log.InfoContext(ctx, "Created Windows service. The service is disabled until Teleport is configured.",
			unitKey, name, "config", ns.teleportConfigFile)
		return nil
	}
	ns.log.InfoContext(ctx, "Created Windows service.", unitKey, name)

	// Equivalent of `systemctl enable --now`: StartType Automatic only takes
	// effect on the next boot, so the service must be started explicitly here.
	if err := service.Start(); err != nil {
		return trace.Wrap(err, "starting Windows service %s", name)
	}
	ns.log.InfoContext(ctx, "Started Windows service.", unitKey, name)
	return nil
}

// updateTeleportService reconciles the fields of an existing Teleport Windows service
// that are owned by teleport-update: the binPath executable (preserving any existing
// arguments), the description, and the recovery actions.
//
// A service that is not managed by the updater and still runs a binary from outside the
// namespace directory is never reconfigured: updateTeleportService returns ErrFilePresent,
// which asks the caller to pass --overwrite. Taking a service over is the responsibility of
// Link (see forceBinPath), which performs that check before this is reached.
//
// Once the service runs a binary that the updater manages, it is marked as managed by the
// updater, even if it was created elsewhere, so that Teardown removes it instead of leaving
// it pointing at a binary that the updater deleted. This mirrors the systemd service file
// marker on Linux, which is also written when an existing unit file is overwritten.
func (ns *Namespace) updateTeleportService(ctx context.Context, service *mgr.Service, exe string, args []string, rev Revision) error {
	cfg, err := service.Config()
	if err != nil {
		return trace.Wrap(err, "reading configuration of Windows service %s", service.Name)
	}
	curExe, curArgs := splitBinPath(cfg.BinaryPathName)
	if curArgs == "" {
		curArgs = escapeArgs(args)
	}
	managed := isManagedServiceDescription(cfg.Description)
	if !samePath(curExe, exe) {
		// important: do not take over a service that runs a non-updater installation of Teleport
		if !managed && curExe != "" && !isWithinDir(ns.Dir(), curExe) {
			return trace.Wrap(ErrFilePresent, "refusing to replace binPath %s of Windows service %s", curExe, service.Name)
		}
		if err := changeServiceBinPath(service, joinBinPath(exe, curArgs)); err != nil {
			return trace.Wrap(err)
		}
		ns.log.InfoContext(ctx, "Updated binPath of Windows service.", unitKey, service.Name, "binary", exe)
	}
	if desc := windowsServiceDescription(rev); cfg.Description != desc {
		if !managed {
			ns.log.WarnContext(ctx, "Windows service that was not created by the updater is now managed by the updater, and will be removed along with it.",
				unitKey, service.Name, "description", cfg.Description)
		}
		if err := setServiceDescription(service, desc); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(setServiceRecovery(service))
}

// setServiceRecovery configures SCM to restart the service after a crash.
func setServiceRecovery(service *mgr.Service) error {
	err := service.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: windowsServiceRecoveryDelay},
		{Type: mgr.ServiceRestart, Delay: windowsServiceRecoveryDelay},
		{Type: mgr.ServiceRestart, Delay: windowsServiceRecoveryDelay},
	}, uint32(windowsServiceRecoveryResetPeriod.Seconds()))
	return trace.Wrap(err, "setting recovery actions of Windows service %s", service.Name)
}

// setServiceDescription sets the description of an open Windows service.
func setServiceDescription(service *mgr.Service, description string) error {
	p, err := windows.UTF16PtrFromString(description)
	if err != nil {
		return trace.Wrap(err, "converting description to UTF16")
	}
	d := windows.SERVICE_DESCRIPTION{Description: p}
	err = windows.ChangeServiceConfig2(service.Handle, windows.SERVICE_CONFIG_DESCRIPTION, (*byte)(unsafe.Pointer(&d)))
	return trace.Wrap(err, "setting description of Windows service %s", service.Name)
}

// escapeArgs joins command line arguments, quoting as needed.
func escapeArgs(args []string) string {
	escaped := make([]string, 0, len(args))
	for _, a := range args {
		escaped = append(escaped, windows.EscapeArg(a))
	}
	return strings.Join(escaped, " ")
}

// connectServiceManager connects to the Windows service manager with the requested access rights.
// Unlike mgr.Connect, this does not request SC_MANAGER_ALL_ACCESS.
func connectServiceManager(access uint32) (*mgr.Mgr, error) {
	h, err := windows.OpenSCManager(nil /*machine*/, nil /*database*/, access)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to Windows service manager")
	}
	return &mgr.Mgr{Handle: h}, nil
}

// HasCustomTbot always returns false on Windows, as tbot is not managed by the updater.
func (ns *Namespace) HasCustomTbot(ctx context.Context) (bool, error) {
	return false, nil
}

// WriteTbotService is not supported on Windows, as tbot is not managed by the updater.
func (ns *Namespace) WriteTbotService(ctx context.Context, pathDir string, rev Revision) error {
	return trace.Wrap(ErrNotSupported)
}

// updaterServiceName returns the name of the Windows service running the updater loop.
// Namespaced installations are suffixed with the namespace name.
func (ns *Namespace) updaterServiceName() string {
	if ns.name == "" {
		return UpdaterServiceName
	}
	return UpdaterServiceName + "_" + ns.name
}

// eventSourceName returns the Windows Event Log source name for the updater.
func (ns *Namespace) eventSourceName() string {
	if ns.name == "" {
		return EventSourceName
	}
	return EventSourceName + "_" + ns.name
}

// Setup installs the TeleportUpdater Windows service, which periodically runs the update subcommand.
// This is the Windows equivalent of writing and enabling the systemd timer.
//
// The service is created or updated to run the updater binary of the given revision,
// configured to start automatically (delayed) and to be restarted by SCM on any failure,
// and started if it is not already running. The Event Log source used by the service is registered.
// SELinux and tbot features are not supported on Windows and are ignored.
func (ns *Namespace) Setup(ctx context.Context, path string, rev Revision, f SetupFeatures) error {
	if f.SELinuxSSH {
		ns.log.DebugContext(ctx, "SELinux is not supported on Windows, ignoring.")
	}
	if f.Tbot {
		ns.log.DebugContext(ctx, "Tbot is not managed by the updater on Windows, ignoring.")
	}

	if err := ns.installEventSource(ctx); err != nil {
		ns.log.WarnContext(ctx, "Failed to register Event Log source. Updater service logs may not be readable.", errorKey, err)
	}

	// Run the updater binary of the given revision directly, so that the service does not
	// depend on the pathDir junction and so that the running updater is always the active one.
	exe := filepath.Join(ns.Dir(), versionsDirName, rev.Dir(), "bin", BinaryName)
	args := []string{"--install-dir", ns.installDir, "windows-service"}
	name := ns.updaterServiceName()

	m, err := connectServiceManager(windows.SC_MANAGER_CONNECT | windows.SC_MANAGER_CREATE_SERVICE)
	if err != nil {
		return trace.Wrap(err)
	}
	defer m.Disconnect()

	service, err := m.OpenService(name)
	if isServiceNotFound(err) {
		service, err = m.CreateService(name, exe, mgr.Config{
			ServiceType:      windows.SERVICE_WIN32_OWN_PROCESS,
			StartType:        mgr.StartAutomatic,
			DelayedAutoStart: true,
			ErrorControl:     mgr.ErrorNormal,
			DisplayName:      name,
			Description:      windowsServiceDescription(rev),
			// ServiceStartName is left empty, which runs the service as LocalSystem.
		}, args...)
		if err != nil {
			return trace.Wrap(err, "creating Windows service %s", name)
		}
		ns.log.InfoContext(ctx, "Created Windows service.", unitKey, name)
	} else if err != nil {
		return trace.Wrap(err, "opening Windows service %s", name)
	} else if err := ns.updateUpdaterService(ctx, service, exe, args, rev); err != nil {
		service.Close()
		return trace.Wrap(err)
	}
	defer service.Close()

	if err := setUpdaterServiceRecovery(service); err != nil {
		return trace.Wrap(err)
	}

	// Equivalent of `systemctl enable --now`: the service was set to start automatically above,
	// so only start it here if it is not already running.
	err = service.Start()
	if errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return nil
	}
	if err != nil {
		return trace.Wrap(err, "starting Windows service %s", name)
	}
	ns.log.InfoContext(ctx, "Started Windows service.", unitKey, name)
	return nil
}

// updateUpdaterService reconciles the configuration of an existing TeleportUpdater service:
// the binPath (executable and arguments), the description, and the automatic (delayed) start type.
func (ns *Namespace) updateUpdaterService(ctx context.Context, service *mgr.Service, exe string, args []string, rev Revision) error {
	cfg, err := service.Config()
	if err != nil {
		return trace.Wrap(err, "reading configuration of Windows service %s", service.Name)
	}
	if binPath := joinBinPath(exe, escapeArgs(args)); cfg.BinaryPathName != binPath {
		if err := changeServiceBinPath(service, binPath); err != nil {
			return trace.Wrap(err)
		}
		ns.log.InfoContext(ctx, "Updated binPath of Windows service.", unitKey, service.Name, "binary", exe)
	}
	if desc := windowsServiceDescription(rev); cfg.Description != desc {
		if err := setServiceDescription(service, desc); err != nil {
			return trace.Wrap(err)
		}
	}
	if cfg.StartType != mgr.StartAutomatic || !cfg.DelayedAutoStart {
		if err := setServiceDelayedAutoStart(service); err != nil {
			return trace.Wrap(err)
		}
		ns.log.InfoContext(ctx, "Enabled Windows service.", unitKey, service.Name)
	}
	return nil
}

// setUpdaterServiceRecovery configures SCM to restart the updater service after any failure,
// including non-zero exits. The updater service exits non-zero after updating itself so that
// SCM restarts it using the new binPath.
func setUpdaterServiceRecovery(service *mgr.Service) error {
	if err := setServiceRecovery(service); err != nil {
		return trace.Wrap(err)
	}
	err := service.SetRecoveryActionsOnNonCrashFailures(true)
	return trace.Wrap(err, "setting recovery actions of Windows service %s", service.Name)
}

// setServiceDelayedAutoStart configures a service to start automatically, after other auto-start services.
func setServiceDelayedAutoStart(service *mgr.Service) error {
	err := windows.ChangeServiceConfig(service.Handle,
		windows.SERVICE_NO_CHANGE, // serviceType
		mgr.StartAutomatic,        // startType
		windows.SERVICE_NO_CHANGE, // errorControl
		nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return trace.Wrap(err, "setting start type of Windows service %s", service.Name)
	}
	info := windows.SERVICE_DELAYED_AUTO_START_INFO{IsDelayedAutoStartUp: 1}
	err = windows.ChangeServiceConfig2(service.Handle, windows.SERVICE_CONFIG_DELAYED_AUTO_START_INFO, (*byte)(unsafe.Pointer(&info)))
	return trace.Wrap(err, "setting delayed start of Windows service %s", service.Name)
}

// installEventSource registers the updater's Event Log source, pointing at the message file in the install directory.
func (ns *Namespace) installEventSource(ctx context.Context) error {
	msgFile := filepath.Join(ns.installDir, windowsEventMessageFile)
	if !libutils.FileExists(msgFile) {
		ns.log.WarnContext(ctx, "Event Log message file not found. Updater service logs may be displayed without formatting.", "path", msgFile)
	}
	return trace.Wrap(eventlog.Install(eventlog.LogName, ns.eventSourceName(), msgFile, false /* useExpandKey */))
}

// Teardown removes all traces of the auto-updater, including its configuration.
// Both the TeleportUpdater service and, if it is managed by the updater, the Teleport service
// are stopped and deleted. A Teleport service that was created elsewhere and then taken over by
// the updater is managed by it, and is therefore removed as well.
// The versions directory, updater configuration, and bin junction are removed.
// The Teleport configuration file and data directory are preserved.
func (ns *Namespace) Teardown(ctx context.Context) error {
	m, err := connectServiceManager(windows.SC_MANAGER_CONNECT)
	if err != nil {
		return trace.Wrap(err)
	}
	defer m.Disconnect()

	if err := ns.deleteService(ctx, m, ns.updaterServiceName(), false); err != nil {
		return trace.Wrap(err)
	}
	// Note: the Teleport service must be deleted before the versions directory below,
	// so that it is never left pointing at a deleted executable.
	if err := ns.deleteService(ctx, m, ns.teleportServiceName(), true); err != nil {
		return trace.Wrap(err)
	}
	if err := eventlog.Remove(eventlog.LogName, ns.eventSourceName()); err != nil {
		ns.log.WarnContext(ctx, "Failed to remove Event Log source.", errorKey, err)
	}

	// Remove the bin junction if it points into the namespace directory.
	target, err := readJunction(ns.defaultPathDir)
	if err == nil && isWithinDir(ns.Dir(), target) {
		if err := os.Remove(ns.defaultPathDir); err != nil {
			ns.log.WarnContext(ctx, "Failed to remove bin junction.", "path", ns.defaultPathDir, errorKey, err)
		}
	}

	// Deleting a still-loaded executable fails on Windows. If this process is running from
	// the versions directory, its own binary will remain until it exits.
	if err := os.RemoveAll(ns.Dir()); err != nil {
		ns.log.WarnContext(ctx, "Failed to fully remove versions directory. It may be removed manually after this process exits.", "path", ns.Dir(), errorKey, err)
	}
	return nil
}

// deleteService stops and deletes the named Windows service, if present.
// If requireManaged is true, the service is only deleted if its description marks it as
// managed by the updater; otherwise, it is left in place with a warning. Services that the
// updater did not create are marked as managed when they are taken over, so this only skips
// services that still run their own binary.
func (ns *Namespace) deleteService(ctx context.Context, m *mgr.Mgr, name string, requireManaged bool) error {
	service, err := m.OpenService(name)
	if isServiceNotFound(err) {
		ns.log.DebugContext(ctx, "Windows service not present.", unitKey, name)
		return nil
	}
	if err != nil {
		return trace.Wrap(err, "opening Windows service %s", name)
	}
	defer service.Close()

	if requireManaged {
		cfg, err := service.Config()
		if err != nil {
			return trace.Wrap(err, "reading configuration of Windows service %s", name)
		}
		if !isManagedServiceDescription(cfg.Description) {
			ns.log.WarnContext(ctx, "Skipping removal of Windows service that is not managed by the updater.", unitKey, name)
			// Reachable if the service was taken over, but marking it failed or never ran.
			if exe, _ := splitBinPath(cfg.BinaryPathName); isWithinDir(ns.Dir(), exe) {
				ns.log.WarnContext(ctx, "The service runs a binary that is about to be removed, and must be reconfigured or removed manually.",
					unitKey, name, "binPath", cfg.BinaryPathName)
			}
			return nil
		}
	}

	s := &scmService{serviceName: name, log: ns.log}
	if err := s.stop(ctx, service); err != nil {
		return trace.Wrap(err, "stopping Windows service %s", name)
	}
	err = service.Delete()
	if errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		ns.log.DebugContext(ctx, "Windows service already marked for deletion.", unitKey, name)
		return nil
	}
	if err != nil {
		return trace.Wrap(err, "deleting Windows service %s", name)
	}
	ns.log.InfoContext(ctx, "Removed Windows service.", unitKey, name)
	return nil
}
