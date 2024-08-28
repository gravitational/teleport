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
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/gravitational/trace"
)

var (
	// ErrFileSharedWriterClosed is returned when file shared writer is already closed.
	ErrFileSharedWriterClosed = errors.New("file shared writer is closed")
)

// FileSharedWriter is similar to SharedWriter except that it requires a `os.File` instead of a `io.Writer`.
// This is to allow the File reopen required by logrotate and similar tools.
type FileSharedWriter struct {
	logFileName string
	fileFlag    int
	fileMode    fs.FileMode
	file        *os.File
	watcher     *fsnotify.Watcher
	closed      bool

	lock sync.Mutex
}

// NewFileSharedWriter wraps the provided [os.File] in a writer that is thread safe,
// with ability to enable filesystem notification watch and reopen file on specific events.
func NewFileSharedWriter(logFileName string, flag int, mode fs.FileMode) (*FileSharedWriter, error) {
	if logFileName == "" {
		return nil, trace.BadParameter("log file name is not set")
	}
	logFile, err := os.OpenFile(logFileName, flag, mode)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		_ = logFile.Close()
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
	if s.closed {
		return 0, trace.Wrap(ErrFileSharedWriterClosed)
	}

	return s.file.Write(b)
}

// Reopen closes the file and opens it again using APPEND mode.
func (s *FileSharedWriter) Reopen() error {
	// If opening the file is locked we should not acquire a lock and block write.
	file, err := os.OpenFile(s.logFileName, s.fileFlag, s.fileMode)
	if err != nil {
		return trace.Wrap(err)
	}

	s.lock.Lock()
	if s.closed {
		s.lock.Unlock()
		_ = file.Close()
		return trace.Wrap(ErrFileSharedWriterClosed)
	}
	oldLogFile := s.file
	s.file = file
	s.lock.Unlock()

	return trace.Wrap(oldLogFile.Close())
}

// RunWatcherReopen runs a filesystem watcher for rename/remove events to reopen the log.
func (s *FileSharedWriter) RunWatcherReopen(ctx context.Context) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.closed {
		return trace.Wrap(ErrFileSharedWriterClosed)
	}

	return s.runWatcherFunc(ctx, s.Reopen)
}

// runWatcherFunc spawns goroutine with the watcher loop to consume events of renaming
// or removing the log file to trigger the action function when event appeared.
func (s *FileSharedWriter) runWatcherFunc(ctx context.Context, action func() error) error {
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
						slog.ErrorContext(ctx, "Failed to reopen file", "error", err, "file", event.Name)
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

// Close stops the internal watcher and close the log file.
func (s *FileSharedWriter) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.closed {
		return trace.Wrap(ErrFileSharedWriterClosed)
	}
	s.closed = true

	return trace.NewAggregate(s.watcher.Close(), s.file.Close())
}
