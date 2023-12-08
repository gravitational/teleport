/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package service

import (
	"fmt"
	"io"
	"runtime"
	"runtime/pprof"
	"time"
)

// writeDebugInfo writes debugging information
// about this process
func writeDebugInfo(w io.Writer) {
	fmt.Fprintf(w, "Runtime stats\n")
	runtimeStats(w)

	fmt.Fprintf(w, "Memory stats\n")
	memStats(w)

	fmt.Fprintf(w, "Goroutines\n")
	goroutineDump(w)
}

func goroutineDump(w io.Writer) {
	pprof.Lookup("goroutine").WriteTo(w, 2)
}

func runtimeStats(w io.Writer) {
	fmt.Fprintf(w, "goroutines: %v\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "OS threads: %v\n", pprof.Lookup("threadcreate").Count())
	fmt.Fprintf(w, "GOMAXPROCS: %v\n", runtime.GOMAXPROCS(0))
	fmt.Fprintf(w, "num CPU: %v\n", runtime.NumCPU())
}

func memStats(w io.Writer) {
	var s runtime.MemStats
	runtime.ReadMemStats(&s)
	fmt.Fprintf(w, "alloc: %v\n", s.Alloc)
	fmt.Fprintf(w, "total-alloc: %v\n", s.TotalAlloc)
	fmt.Fprintf(w, "sys: %v\n", s.Sys)
	fmt.Fprintf(w, "lookups: %v\n", s.Lookups)
	fmt.Fprintf(w, "mallocs: %v\n", s.Mallocs)
	fmt.Fprintf(w, "frees: %v\n", s.Frees)
	fmt.Fprintf(w, "heap-alloc: %v\n", s.HeapAlloc)
	fmt.Fprintf(w, "heap-sys: %v\n", s.HeapSys)
	fmt.Fprintf(w, "heap-idle: %v\n", s.HeapIdle)
	fmt.Fprintf(w, "heap-in-use: %v\n", s.HeapInuse)
	fmt.Fprintf(w, "heap-released: %v\n", s.HeapReleased)
	fmt.Fprintf(w, "heap-objects: %v\n", s.HeapObjects)
	fmt.Fprintf(w, "stack-in-use: %v\n", s.StackInuse)
	fmt.Fprintf(w, "stack-sys: %v\n", s.StackSys)
	fmt.Fprintf(w, "stack-mspan-inuse: %v\n", s.MSpanInuse)
	fmt.Fprintf(w, "stack-mspan-sys: %v\n", s.MSpanSys)
	fmt.Fprintf(w, "stack-mcache-inuse: %v\n", s.MCacheInuse)
	fmt.Fprintf(w, "stack-mcache-sys: %v\n", s.MCacheSys)
	fmt.Fprintf(w, "other-sys: %v\n", s.OtherSys)
	fmt.Fprintf(w, "gc-sys: %v\n", s.GCSys)
	fmt.Fprintf(w, "next-gc: when heap-alloc >= %v\n", s.NextGC)
	lastGC := "-"
	if s.LastGC != 0 {
		lastGC = fmt.Sprint(time.Unix(0, int64(s.LastGC)))
	}
	fmt.Fprintf(w, "last-gc: %v\n", lastGC)
	fmt.Fprintf(w, "gc-pause-total: %v\n", time.Duration(s.PauseTotalNs))
	fmt.Fprintf(w, "gc-pause: %v\n", s.PauseNs[(s.NumGC+255)%256])
	fmt.Fprintf(w, "num-gc: %v\n", s.NumGC)
	fmt.Fprintf(w, "enable-gc: %v\n", s.EnableGC)
	fmt.Fprintf(w, "debug-gc: %v\n", s.DebugGC)
}
