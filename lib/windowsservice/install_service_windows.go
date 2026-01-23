// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/gravitational/teleport"
	eventlogutils "github.com/gravitational/teleport/lib/utils/log/eventlog"
)

// InstallConfig defines parameters for installing a Windows service
// that is implemented by tsh.exe.
type InstallConfig struct {
	// Name is the service name.
	Name string
	// Command is the tsh subcommand that the service manager invokes on start.
	Command string
	// EventSourceName is the name of an event source that will log service events.
	EventSourceName string
	// AccessPermissions defines which service control actions are granted to
	// authenticated users (e.g., start/stop/query).
	AccessPermissions windows.ACCESS_MASK
}

// Install installs a Windows service implemented by tsh.exe.
//
// Windows services are installed by the service manager, which takes a path to
// the service executable. So that regular users are not able to overwrite the
// executable at that path, we use a path under %PROGRAMFILES%, which is not
// writable by regular users by default.
func Install(ctx context.Context, cfg *InstallConfig) (err error) {
	if cfg.Name == "" {
		return trace.BadParameter("service name is required")
	}
	if cfg.Command == "" {
		return trace.BadParameter("command is required")
	}
	if cfg.EventSourceName == "" {
		return trace.BadParameter("event source name is required")
	}
	if cfg.AccessPermissions == 0 {
		return trace.BadParameter("access permissions is required")
	}

	tshPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting current exe path")
	}
	if err := assertTshInProgramFiles(tshPath); err != nil {
		return trace.Wrap(err, "checking if tsh.exe is installed under %%PROGRAMFILES%%")
	}

	svcMgr, err := mgr.Connect()
	if err != nil {
		return trace.Wrap(err, "connecting to Windows service manager")
	}
	svc, err := svcMgr.OpenService(cfg.Name)
	if err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return trace.Wrap(err, "unexpected error checking if Windows service %s exists", cfg.Name)
		}
		// The service has not been created yet and must be installed.
		svc, err = svcMgr.CreateService(
			cfg.Name,
			tshPath,
			mgr.Config{
				StartType: mgr.StartManual,
			},
			cfg.Command,
		)
		if err != nil {
			return trace.Wrap(err, "creating VNet Windows service")
		}
	}
	if err := svc.Close(); err != nil {
		return trace.Wrap(err, "closing VNet Windows service")
	}
	if err := grantServiceRights(cfg.Name, cfg.AccessPermissions); err != nil {
		return trace.Wrap(err, "granting authenticated users permission to control the VNet Windows service")
	}
	if err := installEventSource(cfg.EventSourceName); err != nil {
		trace.Wrap(err, "creating event source for logging")
	}
	if err := logInstallationEvent(cfg.EventSourceName, "service installed"); err != nil {
		trace.Wrap(err, "logging installation event")
	}
	return nil
}

// UninstallConfig defines parameters for removing a Windows service.
type UninstallConfig struct {
	// Name is the service name.
	Name string
	// EventSourceName is the event source to remove from the Windows Event Log.
	EventSourceName string
}

// Uninstall uninstalls the Windows service.
func Uninstall(ctx context.Context, cfg *UninstallConfig) (err error) {
	if cfg.Name == "" {
		return trace.BadParameter("service name is required")
	}
	if cfg.EventSourceName == "" {
		return trace.BadParameter("event source name is required")
	}
	svcMgr, err := mgr.Connect()
	if err != nil {
		return trace.Wrap(err, "connecting to Windows service manager")
	}
	svc, err := svcMgr.OpenService(cfg.Name)
	if err != nil {
		return trace.Wrap(err, "opening Windows service %s", cfg.Name)
	}
	if err := svc.Delete(); err != nil {
		return trace.Wrap(err, "deleting Windows service %s", cfg.Name)
	}
	if err := svc.Close(); err != nil {
		return trace.Wrap(err, "closing VNet Windows service")
	}

	if err := logInstallationEvent(cfg.EventSourceName, "Service uninstalled"); err != nil {
		trace.Wrap(err, "logging installation event")
	}
	if err := eventlogutils.Remove(eventlogutils.LogName, cfg.EventSourceName); err != nil {
		return trace.Wrap(err, "removing event source for logging")
	}

	return nil
}

func grantServiceRights(name string, accessPermissions windows.ACCESS_MASK) error {
	// Get the current security info for the service, requesting only the DACL
	// (discretionary access control list).
	si, err := windows.GetNamedSecurityInfo(name, windows.SE_SERVICE, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return trace.Wrap(err, "getting current service security information")
	}
	// Get the DACL from the security info.
	dacl, _ /*defaulted*/, err := si.DACL()
	if err != nil {
		return trace.Wrap(err, "getting current service DACL")
	}
	// This is the universal well-known SID for "Authenticated Users".
	authenticatedUsersSID, err := windows.StringToSid("S-1-5-11")
	if err != nil {
		return trace.Wrap(err, "parsing authenticated users SID")
	}
	// Build an explicit access entry allowing authenticated users to start,
	// stop, and query the service.
	ea := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: accessPermissions,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(authenticatedUsersSID),
		},
	}}
	// Merge the new explicit access entry with the existing DACL.
	dacl, err = windows.ACLFromEntries(ea, dacl)
	if err != nil {
		return trace.Wrap(err, "merging service DACL entries")
	}
	// Set the DACL on the service security info.
	if err := windows.SetNamedSecurityInfo(
		name,
		windows.SE_SERVICE,
		windows.DACL_SECURITY_INFORMATION,
		nil,  // owner
		nil,  // group
		dacl, // dacl
		nil,  // sacl
	); err != nil {
		return trace.Wrap(err, "setting service DACL")
	}
	return nil
}

// assertTshInProgramFiles asserts that tsh is a regular file installed under
// the program files directory (usually C:\Program Files\).
func assertTshInProgramFiles(tshPath string) error {
	if err := assertRegularFile(tshPath); err != nil {
		return trace.Wrap(err)
	}
	programFiles := os.Getenv("PROGRAMFILES")
	if programFiles == "" {
		return trace.Errorf("PROGRAMFILES env var is not set")
	}
	// Windows file paths are case-insensitive.
	cleanedProgramFiles := strings.ToLower(filepath.Clean(programFiles)) + string(filepath.Separator)
	cleanedTshPath := strings.ToLower(filepath.Clean(tshPath))
	if !strings.HasPrefix(cleanedTshPath, cleanedProgramFiles) {
		return trace.BadParameter(
			"tsh.exe is currently installed at %s, it must be installed under %s in order to install the VNet Windows service",
			tshPath, programFiles)
	}
	return nil
}

func assertRegularFile(path string) error {
	switch info, err := os.Lstat(path); {
	case os.IsNotExist(err):
		return trace.Wrap(err, "%s not found", path)
	case err != nil:
		return trace.Wrap(err, "unexpected error checking %s", path)
	case !info.Mode().IsRegular():
		return trace.BadParameter("%s is not a regular file", path)
	}
	return nil
}

func installEventSource(name string) error {
	exe, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	// Assume that the message file is shipped next to tsh.exe.
	msgFilePath := filepath.Join(filepath.Dir(exe), "msgfile.dll")

	// This should create a registry entry under
	// SYSTEM\CurrentControlSet\Services\EventLog\Teleport\<name> with an absolute path to msgfile.dll.
	// If the user moves Teleport Connect to some other directory, logs will still be captured, but
	// they might display a message about missing event ID until the user reinstalls the app.
	err = eventlogutils.Install(eventlogutils.LogName, name, msgFilePath, false /* useExpandKey */)
	return trace.Wrap(err)
}

func logInstallationEvent(name string, eventMessage string) error {
	log, err := eventlog.Open(name)
	if err != nil {
		return trace.Wrap(err, "opening logger")
	}

	if err := log.Info(eventlogutils.EventID, fmt.Sprintf("%s version:%s", eventMessage, teleport.Version)); err != nil {
		return trace.Wrap(err, "writing log message")
	}

	return trace.Wrap(log.Close(), "closing logger")
}
