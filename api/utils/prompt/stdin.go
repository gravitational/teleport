/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package prompt

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"sync"
)

var (
	stdinMU               sync.Mutex
	stdin                 StdinReader
	stdinTerminalFallback bool
)

// StdinReader contains ContextReader methods applicable to stdin.
type StdinReader interface {
	IsTerminal() bool
	Reader
	SecureReader
}

// Stdin returns a singleton ContextReader wrapped around os.Stdin.
//
// os.Stdin should not be used directly after the first call to this function
// to avoid losing data.
func Stdin() StdinReader {
	stdinMU.Lock()
	defer stdinMU.Unlock()
	if stdin == nil {
		cr := NewContextReader(os.Stdin)
		cr = maybeUseStdinTerminalFallbackLocked(cr)
		go cr.handleInterrupt()
		stdin = cr
	}
	return stdin
}

// SetStdin allows callers to change the Stdin reader.
// Useful to replace Stdin for tests, but should be avoided in production code.
func SetStdin(rd StdinReader) {
	stdinMU.Lock()
	defer stdinMU.Unlock()
	stdin = rd
}

// NotifyExit notifies prompt singletons, such as Stdin, that the program is
// about to exit. This allows singletons to perform actions such as restoring
// terminal state.
// Once NotifyExit is called the singletons will be closed.
func NotifyExit() {
	// Note: don't call methods such as Stdin() here, we don't want to
	// inadvertently hijack the prompts on exit.
	stdinMU.Lock()
	if cr, ok := stdin.(*ContextReader); ok {
		_ = cr.Close()
	}
	stdinMU.Unlock()
}

// EnableStdinTerminalFallback attempts to use a fallback like /dev/tty when stdin is
// not the terminal. This can be useful when tsh is called by another terminal
// tool (e.g. git).
func EnableStdinTerminalFallback() {
	stdinMU.Lock()
	defer stdinMU.Unlock()
	stdinTerminalFallback = true
}

func maybeUseStdinTerminalFallbackLocked(original *ContextReader) *ContextReader {
	if !stdinTerminalFallback || runtime.GOOS == "windows" || original.IsTerminal() {
		return original
	}

	// File /dev/tty is the controlling tty of the current terminal. Not
	// available on Windows.
	// https://tldp.org/HOWTO/Text-Terminal-HOWTO-7.html
	devTTY, err := os.Open("/dev/tty")
	if err != nil {
		slog.DebugContext(context.Background(), "Failed to open /dev/tty", "error", err)
		return original
	}

	fallback := NewContextReader(devTTY)
	if !fallback.IsTerminal() {
		slog.DebugContext(context.Background(), "/dev/tty is not a terminal")
		return original
	}

	slog.DebugContext(context.Background(), "Using /dev/tty for prompt")
	return fallback
}
