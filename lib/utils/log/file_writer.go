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
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/gravitational/trace"
)

var (
	// fileSharedWriter global file shared writer with the ability to reopen the log file
	// in the same mode and with the same flags, either by function call or by filesystem
	// notification subscription on the changes.
	fileSharedWriter *FileSharedWriter

	// initLock protects global init of the file shared writer.
	initLock sync.Mutex
)

// FileSharedWriter is similar to SharedWriter except that it requires a `os.File` instead of a `io.Writer`.
// This is to allow the File reopen required by logrotate and similar tools.
type FileSharedWriter struct {
	logFile  atomic.Pointer[os.File]
	watcher  atomic.Pointer[fsnotify.Watcher]
	fileFlag int
	fileMode fs.FileMode

	lock sync.Mutex
}

// InitFileSharedWriter wraps the provided [os.File] in a writer that is thread safe,
// with ability to enable filesystem notification watch and reopen file on specific events.
func InitFileSharedWriter(logFile *os.File, flag int, mode fs.FileMode) (io.Writer, error) {
	initLock.Lock()
	defer initLock.Unlock()

	if fileSharedWriter != nil {
		return nil, trace.BadParameter("file shared writer already initialized")
	}
	fileSharedWriter = &FileSharedWriter{fileFlag: flag, fileMode: mode}
	fileSharedWriter.logFile.Store(logFile)

	return fileSharedWriter, nil
}

// CloseFileSharedWriter closes the file shared writer and frees up resources.
func CloseFileSharedWriter() {
	initLock.Lock()
	defer initLock.Unlock()

	if fileSharedWriter != nil {
		if err := fileSharedWriter.Close(); err != nil {
			slog.ErrorContext(context.Background(), "Failed to close file shared writer", "error", err)
		}
		fileSharedWriter = nil
	}
}

// GetFileSharedWriter returns instance of the file shared writer.
func GetFileSharedWriter() *FileSharedWriter {
	initLock.Lock()
	defer initLock.Unlock()

	return fileSharedWriter
}

// Write writes len(b) bytes from b to the File.
func (s *FileSharedWriter) Write(b []byte) (int, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.logFile.Load().Write(b)
}

// Reopen closes the file and opens it again using APPEND mode.
func (s *FileSharedWriter) Reopen() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	logFile, err := os.OpenFile(s.logFile.Load().Name(), s.fileFlag, s.fileMode)
	if err != nil {
		return trace.Wrap(err)
	}
	prevLogFile := s.logFile.Swap(logFile)
	if err := prevLogFile.Close(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Close stops watcher loop and frees resources.
func (s *FileSharedWriter) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if watcher := s.watcher.Swap(nil); watcher != nil {
		return trace.Wrap(watcher.Close())
	}
	s.logFile.Swap(nil)

	return nil
}

// RunWatcherReopen runs a filesystem watcher for rename/remove events to reopen the log.
func (s *FileSharedWriter) RunWatcherReopen(ctx context.Context) error {
	return s.runWatcherFunc(ctx, s.Reopen)
}

// runWatcherFunc spawns goroutine with the watcher loop to consume events of renaming
// or removing the log file to trigger the action function when event appeared.
func (s *FileSharedWriter) runWatcherFunc(ctx context.Context, action func() error) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return trace.Wrap(err)
	}
	context.AfterFunc(ctx, func() {
		if err := watcher.Close(); err != nil {
			slog.ErrorContext(context.Background(), "Failed to close file watcher", "error", err)
		}
	})

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if s.logFile.Load().Name() == event.Name && (event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove)) {
					slog.DebugContext(ctx, "Log file was moved/removed, reopen new one", "file", event.Name)
					if err := action(); err != nil {
						slog.ErrorContext(ctx, "Failed to reopen new file", "error", err, "file", event.Name)
						continue
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.ErrorContext(ctx, "Error received on logger watcher", "error", err)
			}
		}
	}()

	logDirParent := filepath.Dir(s.logFile.Load().Name())
	if err = watcher.Add(logDirParent); err != nil {
		return trace.Wrap(err)
	}

	oldWatcher := s.watcher.Swap(watcher)
	if oldWatcher != nil {
		return trace.Wrap(oldWatcher.Close())
	}

	return nil
}
