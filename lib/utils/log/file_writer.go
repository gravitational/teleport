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
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/gravitational/trace"
)

var (
	// fileSharedWriter global file shared writer with the ability to reopen the log file
	// in the same mode and with the same flags, either by function call or by filesystem
	// notification subscription on the changes.
	fileSharedWriter *FileSharedWriter

	// mu protects global setter/getter of the file shared writer.
	mu sync.Mutex
)

// FileSharedWriter is similar to SharedWriter except that it requires a `os.File` instead of a `io.Writer`.
// This is to allow the File reopen required by logrotate and similar tools.
type FileSharedWriter struct {
	logFileName string
	fileFlag    int
	fileMode    fs.FileMode
	file        *os.File
	watcher     *fsnotify.Watcher

	lock sync.Mutex
}

// SetFileSharedWriter sets the global file shared writer, closing the current
// one (if any). The returned error is the error from the closing operation.
func SetFileSharedWriter(sharedWriter *FileSharedWriter) (err error) {
	mu.Lock()
	defer mu.Unlock()

	if fileSharedWriter != nil {
		err = fileSharedWriter.Close()
	}
	fileSharedWriter = sharedWriter

	return trace.Wrap(err)
}

// GetFileSharedWriter returns instance of the global file shared writer.
func GetFileSharedWriter() *FileSharedWriter {
	mu.Lock()
	defer mu.Unlock()

	return fileSharedWriter
}

// NewFileSharedWriter wraps the provided [os.File] in a writer that is thread safe,
// with ability to enable filesystem notification watch and reopen file on specific events.
func NewFileSharedWriter(logFileName string, flag int, mode fs.FileMode) (*FileSharedWriter, error) {
	if logFileName == "" {
		return nil, trace.BadParameter("log file name is not set")
	}
	logFile, err := os.OpenFile(logFileName, flag, mode)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create the log file")
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &FileSharedWriter{
		logFileName: logFileName,
		fileFlag:    flag,
		fileMode:    mode,
		file:        logFile,
		watcher:     watcher,
	}, nil
}

// Write writes len(b) bytes from b to the File.
func (s *FileSharedWriter) Write(b []byte) (int, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.file.Write(b)
}

// Reopen closes the file and opens it again using APPEND mode.
func (s *FileSharedWriter) Reopen() (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.file.Close(); err != nil {
		return trace.Wrap(err)
	}
	s.file, err = os.OpenFile(s.logFileName, s.fileFlag, s.fileMode)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// RunWatcherReopen runs a filesystem watcher for rename/remove events to reopen the log.
func (s *FileSharedWriter) RunWatcherReopen(ctx context.Context) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.runWatcherFunc(ctx, s.Reopen)
}

// runWatcherFunc spawns goroutine with the watcher loop to consume events of renaming
// or removing the log file to trigger the action function when event appeared.
func (s *FileSharedWriter) runWatcherFunc(ctx context.Context, action func() error) error {
	if s.watcher.WatchList() == nil {
		return trace.BadParameter("watcher is already closed")
	}

	go func() {
		for {
			select {
			case event, ok := <-s.watcher.Events:
				if !ok {
					return
				}
				if s.logFileName == event.Name && (event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove)) {
					slog.DebugContext(ctx, "Log file was moved/removed", "file", event.Name)
					if err := action(); err != nil {
						slog.ErrorContext(ctx, "Failed to take action", "error", err, "file", event.Name)
						continue
					}
				}
			case err, ok := <-s.watcher.Errors:
				if !ok {
					return
				}
				slog.ErrorContext(ctx, "Error received on logger watcher", "error", err)
			}
		}
	}()

	logDirParent := filepath.Dir(s.logFileName)
	if err := s.watcher.Add(logDirParent); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Close stops the internal watcher.
func (s *FileSharedWriter) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.watcher.Close()
}
