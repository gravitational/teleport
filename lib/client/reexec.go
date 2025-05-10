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
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/gravitational/trace"
)

// ForkAuthenticateParams are the parameters to RunForkAuthenticate.
type ForkAuthenticateParams struct {
	// GetArgs gets the arguments to re-exec with, excluding the executable
	// (equivalent to os.Args[1:]).
	GetArgs func(signalFd uint64) []string
	// Stdin is the child process' stdin.
	Stdin io.Reader
	// Stdout is the child process' stdout.
	Stdout io.Writer
	// Stderr is the child process' stderr.
	Stderr io.Writer
}

type forkAuthCmd struct {
	*exec.Cmd
	disownSignal io.ReadCloser
	parentStdin  io.Reader
	childStdin   io.WriteCloser
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
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmd := exec.Command(executable)
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signalFd := addSignalFdToChild(cmd, pipeW)

	cmd.Args = append(cmd.Args, params.GetArgs(signalFd)...)
	cmd.Stdout = params.Stdout
	cmd.Stderr = params.Stderr

	// Stdin needs to go through an explicit pipe so we can cut it off without
	// actually closing stdin.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if params.Stdin == nil {
		params.Stdin = os.Stdin
	}
	return &forkAuthCmd{
		Cmd:          cmd,
		disownSignal: pipeR,
		parentStdin:  params.Stdin,
		childStdin:   stdin,
	}, nil
}

func runForkAuthenticateChild(ctx context.Context, cmd *forkAuthCmd) error {
	defer cmd.disownSignal.Close()
	disownReady := make(chan error, 1)
	go func() {
		// The child process will close the pipe when it has authenticated
		// and is ready to be disowned.
		_, err := cmd.disownSignal.Read(make([]byte, 1))
		if errors.Is(err, io.EOF) {
			err = nil
		}
		disownReady <- err
	}()

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	childFinished := make(chan error, 1)
	go func() {
		childFinished <- cmd.Wait()
	}()

	// Copy stdin until the child is ready to disown.
	go io.Copy(cmd.childStdin, cmd.parentStdin)
	defer cmd.childStdin.Close()

	select {
	case err := <-childFinished:
		return trace.Wrap(err)
	case err := <-disownReady:
		return trace.Wrap(err)
	case <-ctx.Done():
		if err := cmd.Process.Kill(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(ctx.Err())
	}
}
