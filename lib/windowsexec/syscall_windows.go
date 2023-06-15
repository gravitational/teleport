// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package windowsexec

// this package is called windowsexec rather than just `windows` because
// calling the package `windows` causes `mkwinsyscall` to generate code
// without the appropriate package name.

import (
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go syscall_windows.go
//sys	shellExecuteExW(info *shellExecuteInfoW) (wasSuccess bool) = shell32.ShellExecuteExW

// shellExecuteInfoW is the input/output struct for ShellExecuteExW.
// See the docs for information about the fields:
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shellexecuteinfow
type shellExecuteInfoW struct {
	cbSize         uint32
	fMask          uint32
	hwnd           syscall.Handle
	lpVerb         uintptr
	lpFile         uintptr
	lpParameters   uintptr
	lpDirectory    uintptr
	nShow          int
	hInstApp       syscall.Handle
	lpIDList       uintptr
	lpClass        uintptr
	hkeyClass      syscall.Handle
	dwHotKey       uint32
	hIconOrMonitor syscall.Handle
	hProcess       syscall.Handle
}

// These consts are copied verbatim from
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shellexecuteinfow
const (
	// SE_ERR_ACCESSDENIED (5):
	// 	Access denied.
	SE_ERR_ACCESSDENIED = 0x05
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
	file string, directory string, timeout time.Duration, parameters ...string,
) error {
	// Convert our various string inputs to UTF16Ptrs
	lpVerb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return trace.Wrap(err, "converting verb to ptr")
	}
	lpFile, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return trace.Wrap(err, "converting file to ptr")
	}
	lpDirectory, err := syscall.UTF16PtrFromString(directory)
	if err != nil {
		return trace.Wrap(err, "converting directory to ptr")
	}
	lpParameters, err := syscall.UTF16PtrFromString(strings.Join(parameters, " "))
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

	success := shellExecuteExW(info)
	if !success {
		err := syscall.GetLastError()
		// The above err can still be nil in certain types of failure
		// if it is returned, then it is much more descriptive, so we should
		// return that.
		if err != nil {
			return trace.Wrap(err, "calling shellExecuteExW")
		}
		if info.hInstApp == SE_ERR_ACCESSDENIED {
			err := trace.BadParameter("shellExecuteExW failed with ACCESSDENIED")
			return trace.WithUserMessage(
				err,
				"This error can occur if the UAC dialogue is rejected or if it is not possible to open a UAC dialogue due to system configuration.",
			)
		}
		// If GetLastError is nil, the only thing we can do is push out the
		// value of hInstApp.
		return trace.Errorf("shellExecuteExW failed and did not call SetLastError. hInstApp=%d", info.hInstApp)

	}
	// Since we provided SEE_MASK_NOCLOSEPROCESS, closing info.hProcess is our
	// responsibility.
	defer syscall.CloseHandle(info.hProcess)

	waitTime := syscall.INFINITE
	if timeout != time.Duration(0) {
		waitTime = int(timeout.Milliseconds())
	}

	// Wait for the process to finish.
	w, err := syscall.WaitForSingleObject(info.hProcess, uint32(waitTime))
	switch w {
	case syscall.WAIT_OBJECT_0:
		// This means the process exited.
		break
	case syscall.WAIT_FAILED:
		return trace.Wrap(err, "could not wait for elevated process")
	case syscall.WAIT_TIMEOUT:
		return trace.Errorf("timed out waiting for elevated process to finish")
	default:
		return trace.Errorf("could not wait for process, unknown state")
	}

	// Check the exit code.
	var code uint32
	if err := syscall.GetExitCodeProcess(info.hProcess, &code); err != nil {
		return err
	}
	if code != 0 {
		return trace.Errorf("elevated process exited with non-zero code: %d", code)
	}

	return nil
}
