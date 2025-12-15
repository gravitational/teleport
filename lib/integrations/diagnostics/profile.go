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

package diagnostics

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	runtimetrace "runtime/trace"
	"strconv"
	"time"

	"github.com/gravitational/trace"
)

// Profile captures various Go pprof profiles and writes
// them to the profivided directory. All profiles are prefixed
// with the same epoch time so that profiles can easily be associated
// as being captured from the same call.
func Profile(dir string) error {
	ctx := context.Background()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return trace.Wrap(err, "creating profile directory %v", dir)
	}

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	traceFile, err := os.Create(filepath.Join(dir, timestamp+"-trace.profile"))
	if err != nil {
		return trace.Wrap(err, "creating trace profile file")
	}
	defer traceFile.Close()

	cpuFile, err := os.Create(filepath.Join(dir, timestamp+"-cpu.profile"))
	if err != nil {
		return trace.Wrap(err, "creating cpu profile file")
	}
	defer cpuFile.Close()

	heapFile, err := os.Create(filepath.Join(dir, timestamp+"-heap.profile"))
	if err != nil {
		return trace.Wrap(err, "creating heap profile file")
	}
	defer heapFile.Close()

	goroutineFile, err := os.Create(filepath.Join(dir, timestamp+"-goroutine.profile"))
	if err != nil {
		return trace.Wrap(err, "creating goroutine profile file")
	}
	defer goroutineFile.Close()

	blockFile, err := os.Create(filepath.Join(dir, timestamp+"-block.profile"))
	if err != nil {
		return trace.Wrap(err, "creating block profile file")
	}
	defer blockFile.Close()

	slog.DebugContext(ctx, "capturing trace profile", "file", traceFile.Name())

	if err := runtimetrace.Start(traceFile); err != nil {
		return trace.Wrap(err, "capturing trace profile")
	}

	slog.DebugContext(ctx, "capturing cpu profile", "file", cpuFile.Name())

	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		return trace.Wrap(err, "capturing cpu profile")
	}

	defer func() {
		slog.DebugContext(ctx, "capturing goroutine profile", "file", cpuFile.Name())

		if err := pprof.Lookup("goroutine").WriteTo(goroutineFile, 0); err != nil {
			slog.WarnContext(ctx, "failed to capture goroutine profile", "error", err)
		}

		slog.DebugContext(ctx, "capturing block profile", "file", cpuFile.Name())

		if err := pprof.Lookup("block").WriteTo(blockFile, 0); err != nil {
			slog.WarnContext(ctx, "failed to capture block profile", "error", err)
		}

		runtime.GC()

		slog.DebugContext(ctx, "capturing heap profile", "file", cpuFile.Name())

		if err := pprof.WriteHeapProfile(heapFile); err != nil {
			slog.WarnContext(ctx, "failed to capture heap profile", "error", err)
		}

		pprof.StopCPUProfile()
		runtimetrace.Stop()
	}()

	<-time.After(30 * time.Second)
	return nil
}
