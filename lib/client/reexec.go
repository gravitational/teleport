package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/gravitational/trace"
)

type ForkAuthenticateCommand struct {
	*exec.Cmd
	disownSignal io.ReadCloser
}

type BuildForkAuthenticateCommandParams struct {
	GetArgs func(signalFd uint64) []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

func BuildForkAuthenticateCommand(params BuildForkAuthenticateCommandParams) (*ForkAuthenticateCommand, error) {
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
	cmd.Stdin = params.Stdin
	cmd.Stdout = params.Stdout
	cmd.Stderr = params.Stderr

	return &ForkAuthenticateCommand{
		Cmd:          cmd,
		disownSignal: pipeR,
	}, nil
}

func RunForkAuthenticateChild(ctx context.Context, cmd *ForkAuthenticateCommand) (err error) {
	fmt.Printf("fork auth: %v\n", cmd.Args)
	defer cmd.disownSignal.Close()
	disownReady := make(chan error, 1)
	go func() {
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
