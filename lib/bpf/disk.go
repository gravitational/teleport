//go:build bpf && !386

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
	"context"
	"io"
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

var lostDiskEvents = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: teleport.MetricLostDiskEvents,
		Help: "Number of lost disk events.",
	},
)

type open struct {
	objs diskObjects

	eventBuf chan []byte
	toClose  []io.Closer

	closed bool
	mtx    sync.Mutex

	lostCounter *Counter
}

// startOpen will compile, load, start, and pull events off the perf buffer
// for the BPF program.
func startOpen(bufferSize int) (*open, error) {
	err := metrics.RegisterPrometheusCollectors(lostDiskEvents)
	if err != nil {
		return nil, trace.Wrap(err, "registering prometheus collectors: %v", err)
	}

	// Remove resource limits for kernels <5.11.
	if err := rlimit.RemoveMemlock(); err != nil {
		logger.ErrorContext(context.Background(), "Removing memlock failed", "error", err)
		return nil, trace.Wrap(err)
	}

	var objs diskObjects
	if err := loadDiskObjects(&objs, nil); err != nil {
		return nil, trace.Wrap(err, "loading disk objects: %v", err)
	}

	lostCtr, err := NewCounter(objs.LostCounter, objs.LostDoorbell, lostDiskEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tp, err := link.AttachTracing(link.TracingOptions{
		Program:    objs.SecurityFileOpen,
		AttachType: ebpf.AttachTraceFEntry,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	eventBuf, err := ringbuf.NewReader(objs.OpenEvents)
	if err != nil {
		return nil, trace.Wrap(err, "creating ring buffer reader: %v", err)
	}

	bpfEvents := make(chan []byte, bufferSize)
	go sendEvents(bpfEvents, eventBuf)

	return &open{
		objs:        objs,
		eventBuf:    bpfEvents,
		toClose:     []io.Closer{tp},
		lostCounter: lostCtr,
	}, nil
}

func (o *open) startSession(auditSessionID uint32) error {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	if o.closed {
		return trace.BadParameter("open session already closed")
	}

	if err := o.objs.MonitoredSessionids.Put(auditSessionID, uint8(0)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (o *open) endSession(auditSessionID uint32) error {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	if o.closed {
		return nil
	}

	if err := o.objs.MonitoredSessionids.Delete(&auditSessionID); err != nil {
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
			logger.WarnContext(context.Background(), "failed to close link", "error", err)
		}
	}

	if err := o.objs.Close(); err != nil {
		logger.WarnContext(context.Background(), "failed to close disk objects", "error", err)
	}

	if err := o.lostCounter.Close(); err != nil {
		logger.WarnContext(context.Background(), "failed to close disk lost counter", "error", err)
	}

	logger.DebugContext(context.Background(), "Closed disk BPF module")
}

// events contains raw events off the perf buffer.
func (o *open) events() <-chan []byte {
	return o.eventBuf
}
