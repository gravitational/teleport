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

package windowsexec

// this package is called windowsexec rather than just `windows` because
// calling the package `windows` causes `mkwinsyscall` to generate code
// without the appropriate package name.

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unsafe"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go syscall_windows.go
//sys	shellExecuteExW(info *shellExecuteInfoW) (err error) [failretval==0] = shell32.ShellExecuteExW

// shellExecuteInfoW is the input/output struct for ShellExecuteExW.
// See the docs for information about the fields:
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shellexecuteinfow
type shellExecuteInfoW struct {
	cbSize         uint32
	fMask          uint32
	hwnd           windows.Handle
	lpVerb         uintptr
	lpFile         uintptr
	lpParameters   uintptr
	lpDirectory    uintptr
	nShow          int
	hInstApp       windows.Handle
	lpIDList       uintptr
	lpClass        uintptr
	hkeyClass      windows.Handle
	dwHotKey       uint32
	hIconOrMonitor windows.Handle
	hProcess       windows.Handle
}

// These consts are copied verbatim from
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shellexecuteinfow
const (
	// SEE_MASK_NOCLOSEPROCESS (0x00000040):
	// Use to indicate that the hProcess member receives the process handle.
	// This handle is typically used to allow an application to find out when a
	// process created with ShellExecuteEx terminates. In some cases, such as
	// when execution is satisfied through a DDE conversation, no handle will be
	// returned. The calling application is responsible for closing the handle
	// when it is no longer needed.
	SEE_MASK_NOCLOSEPROCESS = 0x40
)

// RunAsAndWait uses `ShellExecuteExW` to create a new process with elevated
// privileges on Windows. It waits for the process to exit, or until timeout,
// is exhausted. It will return an error if the process exits with a non-zero
// status code.
func RunAsAndWait(
	file, directory string, timeout time.Duration, parameters []string,
) error {
	// Convert our various string inputs to UTF16Ptrs
	lpVerb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return trace.Wrap(err, "converting verb to ptr")
	}
	lpFile, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return trace.Wrap(err, "converting file to ptr")
	}
	lpDirectory, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return trace.Wrap(err, "converting directory to ptr")
	}
	lpParameters, err := windows.UTF16PtrFromString(strings.Join(parameters, " "))
	if err != nil {
		return trace.Wrap(err, "converting parameters to ptr")
	}

	// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shellexecuteexw
	// Results are returned back into info :)
	info := &shellExecuteInfoW{
		fMask:        SEE_MASK_NOCLOSEPROCESS,
		lpVerb:       uintptr(unsafe.Pointer(lpVerb)),
		lpFile:       uintptr(unsafe.Pointer(lpFile)),
		lpParameters: uintptr(unsafe.Pointer(lpParameters)),
		lpDirectory:  uintptr(unsafe.Pointer(lpDirectory)),
		nShow:        windows.SW_NORMAL,
	}
	// Set the size field of info to the size of info.
	info.cbSize = uint32(unsafe.Sizeof(*info))
	err = shellExecuteExW(info)
	if err != nil {
		// Errors from this can be a little vague, and the contents of hInstApp
		// can provide additional context. We'll emit a debug log so this is a
		// little easier to investigate if a user experience issues with this.
		slog.DebugContext(context.Background(), "Encountered error calling shellExecuteExW",
			"err", err,
			"h_inst_app", info.hInstApp,
		)
		return trace.Wrap(err, "calling shellExecuteExW")
	}
	if info.hProcess == 0 {
		return trace.Errorf("unexpected null hProcess handle from shellExecuteExW call")
	}

	// Since we provided SEE_MASK_NOCLOSEPROCESS, closing info.hProcess is our
	// responsibility.
	defer windows.CloseHandle(info.hProcess)

	waitTime := windows.INFINITE
	if timeout > 0 {
		waitTime = int(timeout.Milliseconds())
	}

	// Wait for the process to finish.
	w, err := windows.WaitForSingleObject(info.hProcess, uint32(waitTime))
	if err != nil {
		return trace.Wrap(err, "could not wait for elevated process")
	}
	switch w {
	case windows.WAIT_OBJECT_0:
		// This means the process exited.
		break
	case uint32(windows.WAIT_TIMEOUT):
		return trace.Errorf("timed out waiting for elevated process to finish")
	default:
		return trace.Errorf("waiting for process resulted in unknown result: %d", w)
	}

	// Check the exit code.
	var code uint32
	if err := windows.GetExitCodeProcess(info.hProcess, &code); err != nil {
		return err
	}
	if code != 0 {
		return trace.Errorf("elevated process exited with non-zero code: %d", code)
	}

	return nil
}
