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

// RunForkAuthenticate re-execs the current executable and waits for any of
// the following:
//   - The child process exits (usually in error).
//   - The child process signals the parent that it is ready to be disowned.
//   - The context is canceled.
func RunForkAuthenticate(ctx context.Context, params ForkAuthenticateParams) error {
	cmd, disownSignal, err := buildForkAuthenticateCommand(params)
	if err != nil {
		return trace.Wrap(err)
	}
	return runForkAuthenticateChild(ctx, cmd, disownSignal)
}

func buildForkAuthenticateCommand(params ForkAuthenticateParams) (cmd *exec.Cmd, disownSignal io.ReadCloser, err error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cmd = exec.Command(executable)
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signalFd := addSignalFdToChild(cmd, pipeW)
	cmd.Args = append(cmd.Args, params.GetArgs(signalFd)...)
	cmd.Stdin = params.Stdin
	cmd.Stdout = params.Stdout
	cmd.Stderr = params.Stderr

	return cmd, pipeR, nil
}

func runForkAuthenticateChild(ctx context.Context, cmd *exec.Cmd, disownSignal io.ReadCloser) error {
	defer disownSignal.Close()
	disownReady := make(chan error, 1)
	go func() {
		// The child process will close the pipe when it has authenticated
		// and is ready to be disowned.
		_, err := disownSignal.Read(make([]byte, 1))
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
