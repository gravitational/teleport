/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package reexec

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

func TestCommand(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		reexecCmd, stdin, stdout := newTestReexecCommand(t, "-test.run=TestReexecEchoProcess")

		// Start the command and go through the ready signal flow.
		require.NoError(t, reexecCmd.Start())
		require.NoError(t, reexecCmd.WaitReady(ctx))
		reexecCmd.Continue()

		// The reexec process should echo back anything written to it.
		echoString := "hello world"
		stdin.Write([]byte(echoString))

		// Close stdin to end the cmd with success.
		stdin.Close()
		exitCode, exitErr := reexecCmd.Wait()
		require.NoError(t, exitErr)
		require.Zero(t, exitCode)
		require.Equal(t, echoString, stdout.String())
	})

	t.Run("continue", func(t *testing.T) {
		t.Parallel()

		reexecCmd, stdin, stdout := newTestReexecCommand(t, "-test.run=TestReexecEchoProcess")

		// Start the command.
		require.NoError(t, reexecCmd.Start())

		// The child process should not echo writes until the parent signals to continue.
		echoString := "hello world"
		stdin.Write([]byte(echoString))

		require.Never(t, func() bool {
			return stdout.Len() != 0
		}, 100*time.Millisecond, 10*time.Millisecond)

		// Signal continue.
		reexecCmd.Continue()

		// Close stdin to end the cmd with success.
		stdin.Close()
		exitCode, exitErr := reexecCmd.Wait()
		require.NoError(t, exitErr)
		require.Zero(t, exitCode)
		require.Equal(t, echoString, stdout.String())
	})

	t.Run("never ready", func(t *testing.T) {
		t.Parallel()

		reexecCmd, _, _ := newTestReexecCommand(t, "-test.run=TestReexecEchoProcess", "REEXEC_SKIP_READY=1")

		// Start the command.
		require.NoError(t, reexecCmd.Start())

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		// If the child never signals ready, we should timeout waiting.
		err := reexecCmd.WaitReady(ctx)
		require.ErrorIs(t, err, ctx.Err())
	})

	t.Run("graceful termination", func(t *testing.T) {
		t.Parallel()

		reexecCmd, _, stdout := newTestReexecCommand(t, "-test.run=TestReexecEchoProcess")

		// Start the command and go through the ready signal flow.
		require.NoError(t, reexecCmd.Start())
		require.NoError(t, reexecCmd.WaitReady(ctx))
		reexecCmd.Continue()

		// Terminate the command prematurely.
		err := reexecCmd.stop(3 * time.Second)
		require.NoError(t, err)
		exitCode, exitErr := reexecCmd.Wait()
		require.NoError(t, exitErr)
		require.Zero(t, exitCode)
		require.Empty(t, stdout.Bytes())
	})

	t.Run("kill", func(t *testing.T) {
		t.Parallel()

		// Purposely don't handle the terminate signal since graceful termination
		// is always attempted first.
		reexecCmd, _, stdout := newTestReexecCommand(t, "-test.run=TestReexecEchoProcess", "REEXEC_IGNORE_TERMINATE=1")

		// Start the command and go through the ready signal flow.
		require.NoError(t, reexecCmd.Start())
		require.NoError(t, reexecCmd.WaitReady(ctx))
		reexecCmd.Continue()

		// Kill the command prematurely.
		err := reexecCmd.stop(500 * time.Millisecond)
		require.NoError(t, err)
		exitCode, exitErr := reexecCmd.Wait()
		require.Error(t, exitErr)
		require.NotZero(t, exitCode)
		require.Empty(t, stdout.Bytes())
	})

	t.Run("extra pipe", func(t *testing.T) {
		t.Parallel()

		reexecCmd, stdin, stdout := newTestReexecCommand(t, "-test.run=TestReexecEchoProcess", "REEXEC_USE_EXTRA=1")

		echoPipe, err := reexecCmd.AddParentToChildPipe()
		require.NoError(t, err)

		require.NoError(t, reexecCmd.Start())
		require.NoError(t, reexecCmd.WaitReady(ctx))
		reexecCmd.Continue()

		// The reexec process should echo back anything written to it.
		echoString := "hello world"
		echoPipe.Write([]byte(echoString))

		// Close the pipe and stdin to end the cmd with success.
		echoPipe.Close()
		stdin.Close()
		exitCode, exitErr := reexecCmd.Wait()
		require.NoError(t, exitErr)
		require.Zero(t, exitCode)
		require.Equal(t, echoString, stdout.String())
	})
}

func TestReexecEchoProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	cfg := os.NewFile(ConfigFile, "config")
	cont := os.NewFile(ContinueFile, "continue")
	ready := os.NewFile(ReadyFile, "ready")
	term := os.NewFile(TerminateFile, "terminate")
	var echo *os.File
	if os.Getenv("REEXEC_USE_EXTRA") == "1" {
		echo = os.NewFile(FirstExtraFile, "echo")
	}

	var cfgPayload Config
	_ = json.NewDecoder(cfg).Decode(&cfgPayload)
	_ = cfg.Close()

	InitLogger("echo-reexec", cfgPayload.LogConfig)

	// Handle graceful termination by ending the copy loop on our side.
	if os.Getenv("REEXEC_IGNORE_TERMINATE") != "1" {
		termReader := term
		go func() {
			if termReader == nil {
				return
			}
			_, _ = termReader.Read(make([]byte, 1))
			slog.DebugContext(t.Context(), "terminate signal received")
			os.Exit(0)
		}()
	}

	if os.Getenv("REEXEC_SKIP_READY") != "1" {
		_ = ready.Close()
		slog.DebugContext(t.Context(), "ready signal sent")
	}

	<-waitForClose(cont)
	slog.DebugContext(t.Context(), "continue signal received")

	var in io.Reader = os.Stdin
	if echo != nil {
		in = io.MultiReader(os.Stdin, echo)
	}

	_, _ = io.Copy(os.Stdout, in)

	slog.DebugContext(t.Context(), "stdin closed, exiting")
	os.Exit(0)
}
func TestCommandCloseIdempotent(t *testing.T) {
	t.Parallel()

	reexecCmd, err := NewCommand(newConfigForReexec(t, ""))
	require.NoError(t, err)

	require.NoError(t, reexecCmd.Close())
	require.NoError(t, reexecCmd.Close())
}

func TestCommandStderrHandling(t *testing.T) {
	t.Parallel()

	t.Run("normal stderr", func(t *testing.T) {
		t.Parallel()

		reexecCmd, _, _ := newTestReexecCommand(t, "-test.run=TestReexecErrorProcess")
		stderrEchoPipe, err := reexecCmd.AddParentToChildPipe()
		require.NoError(t, err)
		require.NoError(t, reexecCmd.Start())

		errMsg := "Failed to launch: big bad bug\n"
		_, err = io.WriteString(stderrEchoPipe, errMsg)
		require.NoError(t, err)
		stderrEchoPipe.Close()

		exitCode, exitErr := reexecCmd.Wait()
		require.Equal(t, 7, exitCode)
		require.Error(t, exitErr)
		require.ErrorContains(t, exitErr, errMsg)
	})

	t.Run("empty stderr", func(t *testing.T) {
		t.Parallel()

		reexecCmd, _, _ := newTestReexecCommand(t, "-test.run=TestReexecErrorProcess")
		stderrEchoPipe, err := reexecCmd.AddParentToChildPipe()
		require.NoError(t, err)
		require.NoError(t, reexecCmd.Start())

		// End the process without any stderr.
		stderrEchoPipe.Close()

		exitCode, err := reexecCmd.Wait()
		require.Equal(t, 7, exitCode)
		require.Error(t, err)
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
	})

	t.Run("malformed stderr", func(t *testing.T) {
		t.Parallel()

		reexecCmd, _, _ := newTestReexecCommand(t, "-test.run=TestReexecErrorProcess")
		stderrEchoPipe, err := reexecCmd.AddParentToChildPipe()
		require.NoError(t, err)
		require.NoError(t, reexecCmd.Start())

		errMsg := "malformed stderr\n"
		_, err = io.WriteString(stderrEchoPipe, errMsg)
		require.NoError(t, err)
		stderrEchoPipe.Close()

		exitCode, err := reexecCmd.Wait()
		require.Equal(t, 7, exitCode)
		require.Error(t, err)
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
	})
}

func TestReexecErrorProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	cfg := os.NewFile(ConfigFile, "config")
	echoStderr := os.NewFile(FirstExtraFile, "stderr")

	var cfgPayload Config
	_ = json.NewDecoder(cfg).Decode(&cfgPayload)
	_ = cfg.Close()

	InitLogger("echo-reexec", cfgPayload.LogConfig)

	if _, err := io.Copy(os.Stderr, echoStderr); err != nil {
		slog.DebugContext(t.Context(), "Stderr copy loop ended with error", "err", err)
	}

	os.Exit(7)
}

func newTestReexecCommand(t *testing.T, reexecCommand string, env ...string) (cmd *Command, stdin io.WriteCloser, stdout *safeBuffer) {
	t.Helper()

	reexecCmd, err := NewCommand(newConfigForReexec(t, reexecCommand))
	require.NoError(t, err)

	reexecCmd.cmd.Env = append(reexecCmd.cmd.Env, "GO_WANT_HELPER_PROCESS=1")
	reexecCmd.cmd.Env = append(reexecCmd.cmd.Env, env...)

	stdout = &safeBuffer{}
	reexecCmd.cmd.Stdout = stdout

	stdin, err = reexecCmd.cmd.StdinPipe()
	require.NoError(t, err)
	t.Cleanup(func() { stdin.Close() })

	return reexecCmd, stdin, stdout
}

func newConfigForReexec(t *testing.T, reexecCommand string) *Config {
	t.Helper()

	return &Config{
		ReexecCommand: reexecCommand,
		LogConfig: LogConfig{
			ExtraFields: []string{
				teleport.ComponentKey, "reexec-logs",
			},
		},
		LogWriter: os.Stderr,
	}
}

func waitForClose(r io.Reader) <-chan struct{} {
	if r == nil {
		return nil
	}

	done := make(chan struct{})
	go func() {
		_, _ = r.Read(make([]byte, 1))
		close(done)
	}()

	return done
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *safeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}

func (b *safeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}
