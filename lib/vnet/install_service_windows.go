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

package vnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/gravitational/teleport/lib/windowsexec"
)

// InstallService installs the VNet windows service.
//
// Windows services are installed by the service manager, which takes a path to
// the service executable. So that regular users are not able to overwrite the
// executable at that path, we use a path under C:\Program Files\, which is not
// writable by regular users by default.
func InstallService(ctx context.Context, username, logFile string) (err error) {
	// If not already running with elevated permissions, exec a child process of
	// the current executable with the current args with `runas`.
	if !windows.GetCurrentProcessToken().IsElevated() {
		return trace.Wrap(elevateChildProcess(ctx),
			"elevating process to install VNet Windows service")
	}

	if logFile == "" {
		return trace.BadParameter("log-file is required")
	}
	defer func() {
		// Write any errors to logFile so the parent process can read it.
		if err != nil {
			// Not really any point checking the error from WriteFile since
			// noone will be able to read it.
			os.WriteFile(logFile, []byte(err.Error()), 0644)
		}
	}()

	if username == "" {
		return trace.BadParameter("username is required")
	}
	u, err := user.Lookup(username)
	if err != nil {
		return trace.Wrap(err, "looking up user %s", username)
	}

	currentTshPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting current exe path")
	}
	currentWintunPath, err := wintunPath(currentTshPath)
	if err != nil {
		return trace.Wrap(err, "getting current wintun.dll path")
	}

	svcMgr, err := mgr.Connect()
	if err != nil {
		return trace.Wrap(err, "connecting to Windows service manager")
	}

	serviceName, serviceInstallDir := serviceNameAndInstallDir(username)
	targetTshPath := filepath.Join(serviceInstallDir, "tsh.exe")
	targetWintunPath := filepath.Join(serviceInstallDir, "wintun.dll")
	if err := os.Mkdir(serviceInstallDir, 0600); err != nil && !errors.Is(err, fs.ErrExist) {
		return trace.Wrap(err, "creating service installation directory %s", serviceInstallDir)
	}
	if err := copyFile(targetTshPath, currentTshPath); err != nil {
		return trace.Wrap(err, "copying tsh.exe to service installation directory")
	}
	if err := copyFile(targetWintunPath, currentWintunPath); err != nil {
		return trace.Wrap(err, "copying wintun.dll to service installation directory")
	}

	if existingSvc, err := svcMgr.OpenService(serviceName); err == nil {
		// The service already exists, delete it and recreate in case
		// configuration options have changed since it was originally installed.
		if err := existingSvc.Delete(); err != nil {
			return trace.Wrap(err, "deleting existing service %s", serviceName)
		}
		_ = existingSvc.Close()
		// The above marks the service for deletion asynchronously,
		// wait for it to actually be deleted.
		timeout := time.After(10 * time.Second)
		ticker := time.Tick(time.Second)
		for {
			existingSvc, err = svcMgr.OpenService(serviceName)
			if err != nil {
				break // Service deleted.
			}
			_ = existingSvc.Close()
			log.InfoContext(ctx, "Waiting for existing service to be deleted...")
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err(), "context canceled while waiting for existing service to be deleted")
			case <-timeout:
				return trace.Errorf("timed out waiting for existing service to be deleted")
			case <-ticker:
			}
		}
	}
	svc, err := svcMgr.CreateService(
		serviceName,
		targetTshPath,
		mgr.Config{
			StartType: mgr.StartManual,
		},
		ServiceCommand,
	)
	if err != nil {
		return trace.Wrap(err, "creating VNet Windows service")
	}
	_ = svc.Close()
	if err := grantServiceRights(serviceName, u.Username); err != nil {
		return trace.Wrap(err, "granting %s permissions to control the VNet Windows service", username)
	}
	return nil
}

// UninstallService uninstalls the VNet windows service.
func UninstallService(ctx context.Context, username, logFile string) (err error) {
	// If not already running with elevated permissions, exec a child process of
	// the current executable with the current args with `runas`.
	if !windows.GetCurrentProcessToken().IsElevated() {
		return trace.Wrap(elevateChildProcess(ctx),
			"elevating process to uninstall VNet Windows service")
	}

	if logFile == "" {
		return trace.BadParameter("log-file is required")
	}
	defer func() {
		// Write any errors to logFile so the parent process can read it.
		if err != nil {
			// Not really any point checking the error from WriteFile since
			// noone will be able to read it.
			os.WriteFile(logFile, []byte(err.Error()), 0644)
		}
	}()

	if username == "" {
		return trace.BadParameter("username is required")
	}
	serviceName, serviceInstallDir := serviceNameAndInstallDir(username)

	deleteServiceErr := trace.Wrap(deleteService(serviceName),
		"deleting Windows service %s", serviceName)
	removeFilesErr := trace.Wrap(os.RemoveAll(serviceInstallDir),
		"removing VNet service installation directory %s", serviceInstallDir)
	return trace.NewAggregate(removeFilesErr, deleteServiceErr)
}

func deleteService(serviceName string) error {
	svcMgr, err := mgr.Connect()
	if err != nil {
		return trace.Wrap(err, "connecting to Windows service manager")
	}
	svc, err := svcMgr.OpenService(serviceName)
	if err != nil {
		return trace.Wrap(err, "opening Windows service")
	}
	err = trace.Wrap(svc.Delete(), "deleting Windows service")
	_ = svc.Close()
	return err
}

func serviceNameAndInstallDir(username string) (string, string) {
	serviceName := userServiceName(username)
	serviceInstallDir := filepath.Join(`C:\Program Files`, serviceName)
	return serviceName, serviceInstallDir
}

func grantServiceRights(serviceName, username string) error {
	// Get the current security info for the service, requesting only the DACL
	// (discretionary access control list).
	si, err := windows.GetNamedSecurityInfo(serviceName, windows.SE_SERVICE, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return trace.Wrap(err, "getting current service security information")
	}
	// Get the DACL from the security info.
	dacl, _ /*defaulted*/, err := si.DACL()
	if err != nil {
		return trace.Wrap(err, "getting current service DACL")
	}
	// Build an explicit access entry allowing our user to start, stop, and
	// query the service.
	ea := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.SERVICE_QUERY_STATUS | windows.SERVICE_START | windows.SERVICE_STOP,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_NAME,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromString(username),
		},
	}}
	// Merge the new explicit access entry with the existing DACL.
	dacl, err = windows.ACLFromEntries(ea, dacl)
	if err != nil {
		return trace.Wrap(err, "merging service DACL entries")
	}
	// Set the DACL on the service security info.
	if err := windows.SetNamedSecurityInfo(
		serviceName,
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

// wintunPath returns the path to wintun.dll which must be in the same directory
// as tshPath.
func wintunPath(tshPath string) (string, error) {
	dir := filepath.Dir(tshPath)
	wintunPath := filepath.Join(dir, "wintun.dll")
	switch _, err := os.Stat(wintunPath); {
	case os.IsNotExist(err):
		return "", trace.Wrap(err, "wintun.dll not found")
	case err != nil:
		return "", trace.Wrap(err, "checking for existence of wintun.dll")
	}
	return wintunPath, nil
}

func copyFile(dstPath, srcPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return trace.Wrap(err, "opening %s for reading", srcPath)
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return trace.Wrap(err, "opening %s for writing", dstPath)
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return trace.Wrap(err, "copying %s to %s", srcPath, dstPath)
	}
	return nil
}

// elevateChildProcess uses `runas` to trigger a child process
// with elevated privileges. This is necessary in order to install or uninstall
// the service with the service control manager.
func elevateChildProcess(ctx context.Context) error {
	username, _, err := currentUsernameAndSID()
	if err != nil {
		return trace.Wrap(err)
	}
	exe, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "determining current executable path")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return trace.Wrap(err, "determining current working directory")
	}
	f, err := os.CreateTemp("", "vnet-install-*.log")
	if err != nil {
		return trace.Wrap(err, "creating log file for elevated child process")
	}
	defer f.Close()
	args := append(os.Args[1:],
		"--username", username,
		"--log-file", f.Name())
	if err := windowsexec.RunAsAndWait(exe, cwd, time.Second*10, args); err != nil {
		err = trace.Wrap(err, "elevating process to manage VNet Windows service")
		output, readOutputErr := io.ReadAll(io.LimitReader(f, 1024))
		if readOutputErr != nil {
			return trace.NewAggregate(err, trace.Wrap(readOutputErr, "reading elevated process log"))
		}
		return trace.NewAggregate(err, fmt.Errorf("elevated process log: %s", string(output)))
	}
	return nil
}
