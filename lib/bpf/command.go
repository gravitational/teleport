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

	"github.com/aquasecurity/libbpfgo"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

var (
	lostCommandEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricLostCommandEvents,
			Help: "Number of lost command events.",
		},
	)
)

const (
	commandEventsBuffer = "execve_events"
)

// rawExecEvent is sent by the eBPF program that Teleport pulls off the perf
// buffer.
type rawExecEvent struct {
	// PID is the ID of the process.
	PID uint64

	// PPID is the PID of the parent process.
	PPID uint64

	// Command is the executable.
	Command [CommMax]byte

	// Type is the type of event.
	Type int32

	// Argv is the list of arguments to the program.
	Argv [ArgvMax]byte

	// ReturnCode is the return code of execve.
	ReturnCode int32

	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint64
}

type exec struct {
	session

	eventBuf *RingBuffer
	lost     *Counter
}

// startExec will load, start, and pull events off the ring buffer
// for the BPF program.
func startExec(bufferSize int) (*exec, error) {
	err := metrics.RegisterPrometheusCollectors(lostCommandEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e := &exec{}

	commandBPF, err := embedFS.ReadFile("bytecode/command.bpf.o")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.session.module, err = libbpfgo.NewModuleFromBuffer(commandBPF, "command")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Resizing the ring buffer must be done here, after the module
	// was created but before it's loaded into the kernel.
	if err = ResizeMap(e.session.module, commandEventsBuffer, uint32(bufferSize*pageSize)); err != nil {
		return nil, trace.Wrap(err)
	}

	// Load into the kernel
	if err = e.session.module.BPFLoadObject(); err != nil {
		return nil, trace.Wrap(err)
	}

	syscalls := []string{"execve", "execveat"}

	for _, syscall := range syscalls {
		if err = AttachSyscallTracepoint(e.session.module, syscall); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	e.eventBuf, err = NewRingBuffer(e.session.module, commandEventsBuffer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.lost, err = NewCounter(e.session.module, "lost", lostCommandEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return e, nil
}

// close will stop reading events off the ring buffer and unload the BPF
// program. The ring buffer is closed as part of the module being closed.
func (e *exec) close() {
	e.lost.Close()
	e.eventBuf.Close()
	e.session.module.Close()
}

// events contains raw events off the perf buffer.
func (e *exec) events() <-chan []byte {
	return e.eventBuf.EventCh
}
