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

		reexecCmd, stdin, stdout := newTestReexecCommand(t)

		// Start the command and go through the ready signal flow.
		require.NoError(t, reexecCmd.Start())
		require.NoError(t, reexecCmd.WaitReady(ctx))
		reexecCmd.Continue()

		// The reexec process should echo back anything written to it.
		echoString := "hello world"
		stdin.Write([]byte(echoString))

		// Close stdin to end the cmd with success.
		stdin.Close()
		require.NoError(t, reexecCmd.Wait())
		require.Equal(t, echoString, stdout.String())

		require.Zero(t, reexecCmd.ExitCode())
	})

	t.Run("continue", func(t *testing.T) {
		t.Parallel()

		reexecCmd, stdin, stdout := newTestReexecCommand(t)

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
		require.NoError(t, reexecCmd.Wait())
		require.Equal(t, echoString, stdout.String())

		require.Zero(t, reexecCmd.ExitCode())
	})

	t.Run("never ready", func(t *testing.T) {
		t.Parallel()

		reexecCmd, _, _ := newTestReexecCommand(t, "REEXEC_SKIP_READY=1")

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

		reexecCmd, _, stdout := newTestReexecCommand(t)

		// Start the command and go through the ready signal flow.
		require.NoError(t, reexecCmd.Start())
		require.NoError(t, reexecCmd.WaitReady(ctx))
		reexecCmd.Continue()

		// Terminate the command prematurely.
		err := reexecCmd.stop(3 * time.Second)
		require.NoError(t, err)
		require.NoError(t, reexecCmd.Wait())
		require.Empty(t, stdout.Bytes())

		require.Zero(t, reexecCmd.ExitCode())
	})

	t.Run("kill", func(t *testing.T) {
		t.Parallel()

		// Purposely don't handle the terminate signal since graceful termination
		// is always attempted first.
		reexecCmd, _, stdout := newTestReexecCommand(t, "REEXEC_IGNORE_TERMINATE=1")

		// Start the command and go through the ready signal flow.
		require.NoError(t, reexecCmd.Start())
		require.NoError(t, reexecCmd.WaitReady(ctx))
		reexecCmd.Continue()

		// Kill the command prematurely.
		err := reexecCmd.stop(500 * time.Millisecond)
		require.NoError(t, err)
		require.Error(t, reexecCmd.Wait())
		require.Empty(t, stdout.Bytes())

		require.NotZero(t, reexecCmd.ExitCode())
	})

	t.Run("extra pipe", func(t *testing.T) {
		t.Parallel()

		reexecCmd, stdin, stdout := newTestReexecCommand(t, "REEXEC_USE_EXTRA=1")

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
		require.NoError(t, reexecCmd.Wait())
		require.Equal(t, echoString, stdout.String())

		require.Zero(t, reexecCmd.ExitCode())
	})
}

func TestCommandCloseIdempotent(t *testing.T) {
	t.Parallel()

	reexecCmd, err := NewCommand(newBasicConfig(t))
	require.NoError(t, err)

	require.NoError(t, reexecCmd.Close())
	require.NoError(t, reexecCmd.Close())
}

func newTestReexecCommand(t *testing.T, env ...string) (cmd *Command, stdin io.WriteCloser, stdout *safeBuffer) {
	t.Helper()

	reexecCmd, err := NewCommand(newBasicConfig(t))
	require.NoError(t, err)

	reexecCmd.cmd.Env = append(reexecCmd.cmd.Env, "GO_WANT_HELPER_PROCESS=1")
	reexecCmd.cmd.Env = append(reexecCmd.cmd.Env, env...)

	stdout = &safeBuffer{}
	reexecCmd.cmd.Stdout = stdout
	reexecCmd.cmd.Stderr = io.Discard

	stdin, err = reexecCmd.cmd.StdinPipe()
	require.NoError(t, err)
	t.Cleanup(func() { stdin.Close() })

	return reexecCmd, stdin, stdout
}

func newBasicConfig(t *testing.T) *Config {
	t.Helper()

	return &Config{
		ReexecCommand: "-test.run=TestReexecEchoProcess",
		LogConfig: LogConfig{
			ExtraFields: []string{
				teleport.ComponentKey, "echo-process",
			},
		},
		LogWriter: os.Stderr,
	}
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
