// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gravitational/trace"
)

func withPidfileCancellation(ctx context.Context, pidFilePath string) (context.Context, error) {
	ctx, cancel := context.WithCancel(ctx)

	pid, running, err := checkProcessRunning(pidFilePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !running {
		cancel()
		return ctx, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, trace.Wrap(err, "creating PID file watcher")
	}
	if err := watcher.Add(pidFilePath); err != nil {
		return nil, trace.Wrap(err, "watching PID file")
	}

	go func() {
		defer cancel()
		defer watcher.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Remove != 0 {
					// The file was removed, return and cancel the context.
					return
				}
				if event.Op&fsnotify.Write != 0 {
					newPID, running, err := checkProcessRunning(pidFilePath)
					if err != nil {
						slog.Warn("Error checking if parent process is running.", "error", err)
						return
					}
					if newPID != pid || !running {
						return
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Warn("PID file watcher error", "error", err)
				return
			case <-time.After(500 * time.Millisecond):
				newPID, running, err := checkProcessRunning(pidFilePath)
				if err != nil {
					slog.Warn("Error checking if parent process is running.", "error", err)
					return
				}
				if newPID != pid || !running {
					return
				}
			}
		}
	}()

	return ctx, nil
}

func checkProcessRunning(pidFilePath string) (int, bool, error) {
	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil {
		return 0, false, trace.Wrap(err, "reading PID file")
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return 0, false, trace.Wrap(err, "parsing PID file")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false, trace.Wrap(err)
	}

	err = process.Signal(syscall.Signal(0))
	return pid, err == nil, trace.Wrap(err, "sending signal to parent")
}
