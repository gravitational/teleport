/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFileSharedWriterNotify checks that if we create the file with shared writer and enable
// watcher functionality, we should expect the file to be reopened after renaming the original one.
func TestFileSharedWriterNotify(t *testing.T) {
	logDir := t.TempDir()
	testFileMode := os.FileMode(0o600)
	testFileFlag := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	logFileName := filepath.Join(logDir, "test.log")

	logWriter, err := NewFileSharedWriter(logFileName, testFileFlag, testFileMode)
	require.NoError(t, err, "failed to init the file shared writer")

	t.Cleanup(func() {
		require.NoError(t, logWriter.Close())
	})

	signal := make(chan struct{})
	err = logWriter.runWatcherFunc(context.Background(), func() error {
		err := logWriter.Reopen()
		signal <- struct{}{}
		return err
	})
	require.NoError(t, err, "failed to run reopen watcher")

	// Write a custom phrase to ensure that the original file was written to before the rotation.
	firstPhrase := "first-write"
	n, err := logWriter.Write([]byte(firstPhrase))
	require.NoError(t, err)
	require.Equal(t, len(firstPhrase), n, "failed to write first phrase")

	data, err := os.ReadFile(logFileName)
	require.NoError(t, err, "cannot read log file")
	require.Equal(t, firstPhrase, string(data), "first written phrase does not match")

	// Move the original file to a new location to simulate the logrotate operation.
	err = os.Rename(logFileName, fmt.Sprintf("%s.1", logFileName))
	require.NoError(t, err, "can't rename log file")

	select {
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for file reopen")
	case <-signal:
	}

	// Write a second custom phrase to ensure the previous one is not in the file.
	secondPhrase := "second-write"
	n, err = logWriter.Write([]byte(secondPhrase))
	require.NoError(t, err)
	require.Equal(t, len(secondPhrase), n, "failed to write second phrase")

	data, err = os.ReadFile(logFileName)
	require.NoError(t, err, "cannot read log file")
	require.Equal(t, secondPhrase, string(data), "second written phrase does not match")
}

// TestFileSharedWriterFinalizer verifies the logic with closing file shared writer
// after overriding it in `logger.SetOutput`.
func TestFileSharedWriterFinalizer(t *testing.T) {
	// output simulates setting the file shared writer to logger.
	var output io.WriteCloser

	logDir := t.TempDir()
	testFileMode := os.FileMode(0o600)
	testFileFlag := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	logFileName := filepath.Join(logDir, "test.log")

	// Initiate the first file shared writer and set it to output.
	var firstWatcherTriggered atomic.Bool
	firstLogWriter, err := NewFileSharedWriter(logFileName, testFileFlag, testFileMode)
	require.NoError(t, err, "failed to init the file shared writer")

	err = firstLogWriter.runWatcherFunc(context.Background(), func() error {
		firstWatcherTriggered.Store(true)
		return nil
	})
	require.NoError(t, err, "failed to run reopen watcher")

	// Set wrapped file shared writer to fake logger output variable.
	output = NewWriterFinalizer(firstLogWriter)
	_, err = output.Write([]byte("test"))
	require.NoError(t, err)

	// Initiate the second file shared writer and override it in common output,
	// previous must be closed automatically by finalizer and stop reacting on events.
	secondLogWriter, err := NewFileSharedWriter(logFileName, testFileFlag, testFileMode)
	require.NoError(t, err, "failed to init the file shared writer")

	signal := make(chan struct{})
	err = secondLogWriter.runWatcherFunc(context.Background(), func() error {
		err := secondLogWriter.Reopen()
		signal <- struct{}{}
		return err
	})
	require.NoError(t, err, "failed to run reopen watcher")

	// Overriding second file shared writer to free resources of the first one
	// and trigger finalizing logic. We have to run GC twice to ensure that
	// it was executed for the firstLogWriter.
	output = secondLogWriter
	runtime.GC()
	runtime.GC()

	// Move the original file to a new location to simulate the logrotate operation.
	err = os.Rename(logFileName, fmt.Sprintf("%s.1", logFileName))
	require.NoError(t, err, "can't rename log file")

	select {
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for file reopen")
	case <-signal:
	}

	// Check that if we set new global file shared writer we close first one.
	require.False(t, firstWatcherTriggered.Load())

	// Check that we receive the error if we are going to try to run watcher
	// again for closed one.
	err = firstLogWriter.RunWatcherReopen(context.Background())
	require.ErrorIs(t, err, ErrFileSharedWriterClosed)

	// First file shared writer must be already closed and produce error after
	// trying to close it second time.
	err = firstLogWriter.Close()
	require.ErrorIs(t, err, ErrFileSharedWriterClosed)

	// Write must not fail after override.
	_, err = output.Write([]byte("test"))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, output.Close())
	})
}
