//go:build windows
// +build windows

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

package common

import (
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

var (
	k32           = syscall.NewLazyDLL("kernel32.dll")
	freeConsole   = k32.NewProc("FreeConsole")
	attachConsole = k32.NewProc("AttachConsole")
)

// onDaemonStop implements the "tsh daemon stop" command. It handles graceful shutdown of the daemon.
//
// teleterm.Serve listens to os.Interrupt and syscall.SIGTERM and stops the deamon when one of the
// signals is received. The Electron app starts the daemon and kills it with SIGTERM before the GUI
// app itself is closed.
//
// The problem is that Windows doesn't implement Unix signals. In Node.js, calling `subprocess.kill`
// on Windows is the equivalent of using SIGKILL to kill a process, giving it no chance to perform
// any kind of cleanup before exiting. [1]
//
// However, there is a way around this problem, as found on SO [2]. A process can attach itself to
// the console of another process. Then it can send a signal to the console which sends a signal to
// all processes attached to that console to shut down. In Go, that signal is represented as
// os.Interrupt. [3]
//
// Why do it in a separate tsh command instead of in the main JavaScript process which starts the
// tsh daemon? First, calling Windows APIs in Go is much easier. Second, sending CTRL_BREAK_EVENT
// sends os.Interrupt to _all_ processes attached to a particular console. We'd have to add special
// calls in Node.js to make the main process ignore that signal. Instead, we can just spawn a new
// process and not care about ignoring the signal.
//
// [1] https://nodejs.org/docs/latest-v16.x/api/child_process.html#subprocesskillsignal
// [2] https://stackoverflow.com/a/50020028/742872
// [3] https://pkg.go.dev/os/signal#hdr-Windows
//
// Related:
// - https://blog.codetitans.pl/post/sending-ctrl-c-signal-to-another-application-on-windows/
func onDaemonStop(cf *CLIConf) error {
	// A process can be attached to at most one console, so we have to deattach the current process
	// from its console first.
	//
	// returnValue is used for syscall.Proc.Call-specific error handling, see:
	// https://github.com/golang/go/blob/go1.19.5/src/syscall/dll_windows.go#L175-L188
	// https://learn.microsoft.com/en-us/windows/console/freeconsole
	returnValue, _, err := freeConsole.Call()
	if returnValue == 0 {
		return trace.Wrap(err)
	}

	// https://learn.microsoft.com/en-us/windows/console/attachconsole
	returnValue, _, err = attachConsole.Call(uintptr(cf.DaemonPid))
	if returnValue == 0 {
		return trace.Wrap(err)
	}

	// We have to use CTRL_BREAK_EVENT instead of CTRL_C_EVENT here. For some reason, CTRL_C_EVENT
	// doesn't make the other process to receive os.Interrupt.
	//
	// It might be because CTRL_C_EVENT is received as a signal by default, but processes can use
	// SetConsoleCtrlHandler to receive CTRL_C_EVENT as keyboard input. [1] We didn't investigate if
	// this is what Electron does under the hood when using spawn, but rest assured we know that
	// CTRL_C_EVENT works here - as the docs say, it's always sent as a signal.
	//
	// [1] https://learn.microsoft.com/en-us/windows/console/ctrl-c-and-ctrl-break-signals
	// https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
	err = windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, 0)
	return trace.Wrap(err)
}
