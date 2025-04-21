package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/gravitational/trace"
)

type buildForkAuthenticateCommandParams struct {
	getArgs func(signalFd uintptr) []string
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

func buildForkAuthenticateCommand(ctx context.Context, params buildForkAuthenticateCommandParams) (cmd *exec.Cmd, disownSignal *os.File, err error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cmd = exec.CommandContext(ctx, executable)
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signalFd := addSignalFdToChild(cmd, pipeW)
	cmd.Args = append(cmd.Args, params.getArgs(signalFd)...)
	cmd.Stdin = params.stdin
	cmd.Stdout = params.stdout
	cmd.Stderr = params.stderr

	return cmd, pipeR, nil
}

func RunForkAuthenticateChild(ctx context.Context, cmd *exec.Cmd, disownSignal *os.File) (err error) {
	childFinished := make(chan error, 1)
	defer func() {
		if err != nil {
			cmd.Process.Kill()
			select {
			case <-childFinished:
			case <-time.After(5 * time.Second):
				slog.WarnContext(ctx, "timed out waiting for child to finish")
			}
		}
	}()
	disownReady := make(chan error, 1)
	go func() {
		_, err := disownSignal.Read(make([]byte, 1))
		if errors.Is(err, io.EOF) {
			err = nil
		}
		disownReady <- err
	}()

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	go func() {
		childFinished <- cmd.Wait()
	}()

	select {
	case err := <-childFinished:
		return trace.Wrap(err)
	case err := <-disownReady:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}
