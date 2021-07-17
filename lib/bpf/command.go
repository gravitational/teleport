// +build bpf,!386

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bpf

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/aquasecurity/libbpfgo"
	"github.com/prometheus/client_golang/prometheus"

	_ "embed"
)

//go:embed bytecode/command.bpf.o
var commandBPF []byte

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
	module *libbpfgo.Module

	eventBuf *RingBuffer
	lost     *Counter
}

// startExec will load, start, and pull events off the ring buffer
// for the BPF program.
func startExec(bufferSize int) (*exec, error) {
	err := utils.RegisterPrometheusCollectors(lostCommandEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e := &exec{}

	e.module, err = libbpfgo.NewModuleFromBuffer(commandBPF, "command")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Resizing the ring buffer must be done here, after the module
	// was created but before it's loaded into the kernel.
	if err = ResizeMap(e.module, commandEventsBuffer, uint32(bufferSize*pageSize)); err != nil {
		return nil, trace.Wrap(err)
	}

	// Load into the kernel
	if err = e.module.BPFLoadObject(); err != nil {
		return nil, trace.Wrap(err)
	}

	syscalls := []string{"execve", "execveat"}

	for _, syscall := range syscalls {
		if err = AttachSyscallTracepoint(e.module, syscall); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	e.eventBuf, err = NewRingBuffer(e.module, commandEventsBuffer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.lost, err = NewCounter(e.module, "lost", lostCommandEvents)
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
	e.module.Close()
}

// events contains raw events off the perf buffer.
func (e *exec) events() <-chan []byte {
	return e.eventBuf.EventCh
}
