package common

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/trace"
)

func buildForkAuthenticateCommand(ctx context.Context, origArgs []string, cf *CLIConf) (cmd *exec.Cmd, disownSignal *os.File, err error) {
	if origArgs[0] != "ssh" {
		return nil, nil, trace.BadParameter("cannot fork authenticate for non-ssh command")
	}
	cmd = exec.CommandContext(ctx, cf.executablePath)

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signalFd := addSignalFdToChild(cmd, pipeW)
	cmd.Args = append([]string{cf.executablePath, "ssh", "--fork-signal-fd", strconv.FormatUint(uint64(signalFd), 16)}, origArgs[1:]...)
	cmd.Stdin = cf.Stdin()
	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()

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
		var execErr *exec.ExitError
		if errors.As(err, &execErr) {
			return &common.ExitCodeError{Code: execErr.ExitCode()}
		}
		return trace.Wrap(err)
	case err := <-disownReady:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}
