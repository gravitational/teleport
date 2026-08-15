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
	"errors"
	"io"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const (
	doFilpOpenName = "do_filp_open"
	doFileOpenName = "do_file_open"
)

var lostDiskEvents = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: teleport.MetricLostDiskEvents,
		Help: "Number of lost disk events.",
	},
)

type open struct {
	objs diskObjects

	bpfEvents   chan []byte
	lostCounter *Counter
	toClose     []io.Closer

	closed   bool
	flushBuf func() error

	mtx sync.Mutex
	wg  sync.WaitGroup
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

	// The disk eBPF program hooks do_file_open by default, which was
	// renamed from do_filp_open in 7.0. Check if do_file_open is available
	// and use do_filp_open otherwise; don't rely on the kernel version.
	attachTarget, err := findOpenAttachTarget()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var objs diskObjects
	spec, err := loadDisk()
	if err != nil {
		return nil, trace.Wrap(err, "reading disk objects")
	}

	p, ok := spec.Programs["do_file_open_exit"]
	if !ok {
		return nil, trace.NotFound("do_file_open_exit not found in disk objects")
	}
	p.AttachTo = attachTarget

	if err := spec.LoadAndAssign(&objs, nil); err != nil {
		return nil, trace.Wrap(err, "loading disk objects")
	}

	lostCtr, err := NewCounter(objs.LostCounter, objs.LostDoorbell, lostDiskEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var toClose []io.Closer
	tps := []struct {
		prog       *ebpf.Program
		attachType ebpf.AttachType
	}{
		{
			prog:       objs.SecurityFileOpen,
			attachType: ebpf.AttachTraceFEntry,
		},
		{
			prog:       objs.DoFileOpenExit,
			attachType: ebpf.AttachTraceFExit,
		},
	}
	for _, tp := range tps {
		lk, err := link.AttachTracing(link.TracingOptions{
			Program:    tp.prog,
			AttachType: tp.attachType,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		toClose = append(toClose, lk)
	}

	eventBuf, err := ringbuf.NewReader(objs.OpenEvents)
	if err != nil {
		return nil, trace.Wrap(err, "creating ring buffer reader: %v", err)
	}

	o := &open{
		objs:        objs,
		lostCounter: lostCtr,
		toClose:     toClose,
		flushBuf:    eventBuf.Flush,
	}

	o.bpfEvents = make(chan []byte, bufferSize)
	o.wg.Go(func() { sendEvents("disk", o.bpfEvents, eventBuf) })

	return o, nil
}

func findOpenAttachTarget() (string, error) {
	kspec, err := btf.LoadKernelSpec()
	if err != nil {
		return "", trace.Wrap(err, "loading kernel BTF spec")
	}

	for _, target := range []string{doFileOpenName, doFilpOpenName} {
		var fn *btf.Func
		if err := kspec.TypeByName(target, &fn); err != nil {
			if errors.Is(err, btf.ErrNotFound) {
				continue
			}
			return "", trace.Wrap(err, "finding %s", target)
		}

		return target, nil
	}

	return "", trace.NotFound("do_file_open and do_filp_open not found in kernel BTF spec")
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

	if o.closed {
		o.mtx.Unlock()
		return
	}

	o.closed = true

	if err := o.flushBuf(); err != nil {
		logger.WarnContext(context.Background(), "failed to flush disk ring buffer", "error", err)
	} else {
		logger.DebugContext(context.Background(), "Flushed disk ring buffer, waiting for pending events to be processed")
	}

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

	// Unlock before waiting for the goroutines to finish to avoid
	// startSession/endSession blocking for potentially a long time.
	o.mtx.Unlock()
	o.wg.Wait()

	logger.DebugContext(context.Background(), "Closed disk BPF module")
}

// events contains raw events off the perf buffer.
func (o *open) events() <-chan []byte {
	return o.bpfEvents
}
