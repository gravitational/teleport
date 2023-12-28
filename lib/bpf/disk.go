//go:build bpf && !386
// +build bpf,!386

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

package bpf

import (
	_ "embed"
	"io"
	"runtime"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

var (
	lostDiskEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricLostDiskEvents,
			Help: "Number of lost disk events.",
		},
	)
)

type cgroupRegister interface {
	startSession(cgroupID uint64) error
	endSession(cgroupID uint64) error
}

type open struct {
	objs diskObjects

	eventBuf chan []byte
	toClose  []io.Closer

	closed bool
	mtx    sync.Mutex
}

// startOpen will compile, load, start, and pull events off the perf buffer
// for the BPF program.
func startOpen(bufferSize int) (*open, error) {
	err := metrics.RegisterPrometheusCollectors(lostDiskEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove resource limits for kernels <5.11.
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal("Removing memlock:", err)
	}

	var objs diskObjects
	if err := loadDiskObjects(&objs, nil); err != nil {
		return nil, trace.Wrap(err)
	}

	trs := []struct {
		name string
		prog *ebpf.Program
	}{
		{
			name: "sys_enter_creat",
			prog: objs.TracepointSyscallsSysEnterCreat,
		},
		{
			name: "sys_enter_open",
			prog: objs.TracepointSyscallsSysEnterOpen,
		},
		{
			name: "sys_enter_openat2",
			prog: objs.TracepointSyscallsSysEnterOpenat2,
		},
		{
			name: "sys_exit_creat",
			prog: objs.TracepointSyscallsSysExitCreat,
		},
		{
			name: "sys_exit_open",
			prog: objs.TracepointSyscallsSysExitOpen,
		},
		{
			name: "sys_exit_openat2",
			prog: objs.TracepointSyscallsSysExitOpenat2,
		},
	}

	if runtime.GOARCH != "arm64" {
		// openat is not implemented on arm64.
		trs = append(trs, []struct {
			name string
			prog *ebpf.Program
		}{
			{
				name: "sys_enter_openat",
				prog: objs.TracepointSyscallsSysEnterOpenat,
			},
			{
				name: "sys_exit_openat",
				prog: objs.TracepointSyscallsSysExitOpenat,
			},
		}...)
	}

	toClose := make([]io.Closer, 0, len(trs))
	for _, tr := range trs {
		tp, err := link.Tracepoint("syscalls", tr.name, tr.prog, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		toClose = append(toClose, tp)
	}

	eventBuf, err := ringbuf.NewReader(objs.OpenEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bpfEvents := make(chan []byte, 100)
	go sendEvents(bpfEvents, eventBuf)

	return &open{
		objs:     objs,
		eventBuf: bpfEvents,
		toClose:  toClose,
	}, nil
}

func (o *open) startSession(cgroupID uint64) error {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	if o.closed {
		return trace.BadParameter("open session already closed")
	}

	if err := o.objs.MonitoredCgroups.Put(cgroupID, int64(0)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (o *open) endSession(cgroupID uint64) error {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	if o.closed {
		return nil // Ignore. If the session is closed, the cgroup is no longer monitored.
	}

	if err := o.objs.MonitoredCgroups.Delete(&cgroupID); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// close will stop reading events off the ring buffer and unload the BPF
// program. The ring buffer is closed as part of the module being closed.
func (o *open) close() {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	if o.closed {
		return
	}

	o.closed = true

	for _, toClose := range o.toClose {
		if err := toClose.Close(); err != nil {
			log.Warn(err)
		}
	}

	if err := o.objs.Close(); err != nil {
		log.Warn(err)
	}
}

// events contains raw events off the perf buffer.
func (o *open) events() <-chan []byte {
	return o.eventBuf
}
