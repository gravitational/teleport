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

package client

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/gravitational/trace"
)

// ForkAuthenticateParams are the parameters to RunForkAuthenticate.
type ForkAuthenticateParams struct {
	// GetArgs gets the arguments to re-exec with, excluding the executable
	// (equivalent to os.Args[1:]).
	GetArgs func(signalFd, killFd uint64) []string
	// Stdin is the child process' stdin.
	Stdin io.Reader
	// Stdout is the child process' stdout.
	Stdout io.Writer
	// Stderr is the child process' stderr.
	Stderr io.Writer
}

type forkAuthCmd struct {
	*exec.Cmd
	signalR, signalW, killR, killW *os.File
}

// RunForkAuthenticate re-execs the current executable and waits for any of
// the following:
//   - The child process exits (usually in error).
//   - The child process signals the parent that it is ready to be disowned.
//   - The context is canceled.
func RunForkAuthenticate(ctx context.Context, params ForkAuthenticateParams) error {
	cmd, err := buildForkAuthenticateCommand(params)
	if err != nil {
		return trace.Wrap(err)
	}
	return runForkAuthenticateChild(ctx, cmd)
}

func buildForkAuthenticateCommand(params ForkAuthenticateParams) (*forkAuthCmd, error) {
	executable, err := getExecutable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmd := exec.Command(executable)
	// Set up disown signal.
	signalR, signalW, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	killR, killW, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signalFd, killFd := configureReexecForOS(cmd, signalW, killR)

	cmd.Args = append(cmd.Args, params.GetArgs(signalFd, killFd)...)
	cmd.Args[0] = os.Args[0]
	cmd.Stdin = params.Stdin
	cmd.Stdout = params.Stdout
	cmd.Stderr = params.Stderr

	return &forkAuthCmd{
		Cmd: cmd,
		// Keep all pipes around to prevent them from being garbage collected.
		signalR: signalR,
		signalW: signalW,
		killR:   killR,
		killW:   killW,
	}, nil
}

func runForkAuthenticateChild(ctx context.Context, cmd *forkAuthCmd) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer cmd.signalR.Close()
	defer cmd.killW.Close()
	defer cmd.killW.Write([]byte{0x00})

	disownReady := make(chan error, 1)
	go func() {
		// The child process will write to the pipe when it has authenticated
		// and is ready to be disowned.
		_, err := cmd.signalR.Read(make([]byte, 1))
		if err == nil {
			disownReady <- nil
		}
		// Error was likely caused by the child process exiting. Wait for Wait() to
		// return the exit status if possible.
		select {
		case <-runCtx.Done():
		case <-time.After(3 * time.Second):
			disownReady <- err
		}
	}()

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	for _, file := range cmd.ExtraFiles {
		if err := file.Close(); err != nil {
			return trace.Wrap(err)
		}
	}
	childFinished := make(chan error, 1)
	go func() {
		childFinished <- cmd.Wait()
	}()

	select {
	case err := <-childFinished:
		return trace.Wrap(err)
	case err := <-disownReady:
		// Check if child finished at the same time.
		if finishedErr, ok := <-childFinished; ok {
			return trace.Wrap(finishedErr)
		}
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(cmd.Process.Release())
	case <-runCtx.Done():
		if err := cmd.Process.Kill(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(runCtx.Err())
	}
}
