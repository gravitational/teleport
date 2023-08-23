//go:build bpf && !386
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
	_ "embed"
	"runtime"

	"github.com/aquasecurity/libbpfgo"
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

const (
	diskEventsBuffer = "open_events"
)

// rawOpenEvent is sent by the eBPF program that Teleport pulls off the perf
// buffer.
type rawOpenEvent struct {
	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint64

	// PID is the ID of the process.
	PID uint64

	// ReturnCode is the return code of open.
	ReturnCode int32

	// Command is name of the executable opening the file.
	Command [CommMax]byte

	// Path is the full path to the file being opened.
	Path [PathMax]byte

	// Flags are the flags passed to open.
	Flags int32
}

type cgroupRegister interface {
	startSession(cgroupID uint64) error
	endSession(cgroupID uint64) error
}

type open struct {
	session

	eventBuf *RingBuffer
	lost     *Counter
}

// startOpen will compile, load, start, and pull events off the perf buffer
// for the BPF program.
func startOpen(bufferSize int) (*open, error) {
	err := metrics.RegisterPrometheusCollectors(lostDiskEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	o := &open{}

	diskBPF, err := embedFS.ReadFile("bytecode/disk.bpf.o")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	o.session.module, err = libbpfgo.NewModuleFromBuffer(diskBPF, "disk")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Resizing the ring buffer must be done here, after the module
	// was created but before it's loaded into the kernel.
	if err = ResizeMap(o.session.module, diskEventsBuffer, uint32(bufferSize*pageSize)); err != nil {
		return nil, trace.Wrap(err)
	}

	// Load into the kernel
	if err = o.session.module.BPFLoadObject(); err != nil {
		return nil, trace.Wrap(err)
	}

	syscalls := []string{"openat", "openat2"}

	if runtime.GOARCH != "arm64" {
		// open is not implemented on arm64.
		syscalls = append(syscalls, "open")
	}

	for _, syscall := range syscalls {
		if err = AttachSyscallTracepoint(o.session.module, syscall); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	o.eventBuf, err = NewRingBuffer(o.session.module, diskEventsBuffer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	o.lost, err = NewCounter(o.session.module, "lost", lostDiskEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return o, nil
}

// close will stop reading events off the ring buffer and unload the BPF
// program. The ring buffer is closed as part of the module being closed.
func (o *open) close() {
	o.lost.Close()
	o.eventBuf.Close()
	o.session.module.Close()
}

// events contains raw events off the perf buffer.
func (o *open) events() <-chan []byte {
	return o.eventBuf.EventCh
}
