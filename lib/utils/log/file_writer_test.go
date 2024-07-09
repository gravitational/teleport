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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFileSharedWriterNotify checks that if we create the file with shared writer and enable
// watcher functionality, we should expect the file to be reopened after renaming the original one.
func TestFileSharedWriterNotify(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	logDir := t.TempDir()
	testFileMode := os.FileMode(0o600)
	testFileFlag := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	logFile, err := os.OpenFile(filepath.Join(logDir, "test.log"), testFileFlag, testFileMode)
	require.NoError(t, err, "failed to open log file")

	t.Cleanup(func() {
		cancel()
	})

	logWriter, err := NewFileSharedWriter(logFile, testFileFlag, testFileMode)
	require.NoError(t, err, "failed to init the file shared writer")

	signal := make(chan struct{})
	err = logWriter.runWatcherFunc(ctx, func() error {
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

	data, err := os.ReadFile(logFile.Name())
	require.NoError(t, err, "cannot read log file")
	require.Equal(t, firstPhrase, string(data), "first written phrase does not match")

	// Move the original file to a new location to simulate the logrotate operation.
	err = os.Rename(logFile.Name(), fmt.Sprintf("%s.1", logFile.Name()))
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

	data, err = os.ReadFile(logFile.Name())
	require.NoError(t, err, "cannot read log file")
	require.Equal(t, secondPhrase, string(data), "second written phrase does not match")
}
