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

package reexec

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

// NotifyFileSignal signals on the returned channel when the provided file
// receives a signal (a one-byte read).
func NotifyFileSignal(f *os.File) <-chan error {
	errorCh := make(chan error, 1)
	go func() {
		n, err := f.Read(make([]byte, 1))
		if n > 0 {
			errorCh <- nil
		} else if err == nil {
			// this should be impossible according to the io.Reader contract
			errorCh <- io.ErrUnexpectedEOF
		} else {
			errorCh <- err
		}
	}()
	return errorCh
}

// SignalAndClose writes a byte to the provided file (to signal a caller of
// NotifyFileSignal) and closes it.
func SignalAndClose(f *os.File) error {
	_, err := f.Write([]byte{0x00})
	return trace.NewAggregate(err, f.Close())
}

// ForkAuthenticateParams are the parameters to RunForkAuthenticate.
type ForkAuthenticateParams struct {
	// GetArgs gets the arguments to re-exec with, excluding the executable
	// (equivalent to os.Args[1:]).
	GetArgs func(signalFd, killFd uint64) []string
	// executable is the executable to run while re-execing. Overridden in tests.
	executable string
	// Stdin is the child process' stdin.
	Stdin io.Reader
	// Stdout is the child process' stdout.
	Stdout io.Writer
	// Stderr is the child process' stderr.
	Stderr io.Writer
}

// RunForkAuthenticate re-execs the current executable and waits for any of
// the following:
//   - The child process exits (usually in error).
//   - The child process signals the parent that it is ready to be disowned.
//   - The context is canceled.
func RunForkAuthenticate(ctx context.Context, params ForkAuthenticateParams) error {
	if params.executable == "" {
		executable, err := getExecutable()
		if err != nil {
			return trace.Wrap(err)
		}
		params.executable = executable
	}
	cmd := exec.Command(params.executable)
	// Set up signal pipes.
	disownR, disownW, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err)
	}
	killR, killW, err := os.Pipe()
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		// If the child is still listening, kill it. If the child successfully
		// disowned, this will do nothing.
		SignalAndClose(killW)
		killR.Close()
		disownW.Close()
		disownR.Close()
	}()

	signalFd, killFd := configureReexecForOS(cmd, disownW, killR)
	cmd.Args = append(cmd.Args, params.GetArgs(signalFd, killFd)...)
	cmd.Args[0] = os.Args[0]
	cmd.Stdin = params.Stdin
	cmd.Stdout = params.Stdout
	cmd.Stderr = params.Stderr

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	// Clean up parent end of pipes.
	if err := disownW.Close(); err != nil {
		return trace.NewAggregate(err, killAndWaitProcess(cmd))
	}
	if err := killR.Close(); err != nil {
		return trace.NewAggregate(err, killAndWaitProcess(cmd))
	}

	select {
	case err := <-NotifyFileSignal(disownR):
		if err == nil {
			return trace.Wrap(cmd.Process.Release())
		} else if errors.Is(err, io.EOF) {
			// EOF means the child process exited, no need to report it on top of kill/wait.
			return trace.Wrap(killAndWaitProcess(cmd))
		}
		return trace.NewAggregate(err, killAndWaitProcess(cmd))
	case <-ctx.Done():
		return trace.NewAggregate(ctx.Err(), killAndWaitProcess(cmd))
	}
}

func killAndWaitProcess(cmd *exec.Cmd) error {
	if err := cmd.Process.Kill(); err != nil {
		return trace.Wrap(err)
	}
	err := cmd.Wait()
	var execErr *exec.ExitError
	if errors.As(err, &execErr) && execErr.ExitCode() != 0 {
		return trace.Wrap(err)
	} else if err != nil && strings.Contains(err.Error(), "signal: killed") {
		// If the process was successfully killed, there is no issue.
		return nil
	}
	return trace.Wrap(err)
}
