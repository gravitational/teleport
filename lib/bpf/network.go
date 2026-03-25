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

var lostNetworkEvents = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: teleport.MetricLostNetworkEvents,
		Help: "Number of lost network events.",
	},
)

type conn struct {
	objs *networkObjects

	event4Chan chan []byte
	event6Chan chan []byte
	toClose    []io.Closer

	closed bool
	mtx    sync.Mutex

	lostCounter *Counter
}

func startConn(bufferSize int) (*conn, error) {
	err := metrics.RegisterPrometheusCollectors(lostNetworkEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove resource limits for kernels <5.11.
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, trace.WrapWithMessage(err, "Removing memlock")
	}

	var objs networkObjects
	if err := loadNetworkObjects(&objs, nil); err != nil {
		return nil, trace.Wrap(err)
	}

	lostCtr, err := NewCounter(objs.LostCounter, objs.LostDoorbell, lostNetworkEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kprobes := []struct {
		symbol string
		prog   *ebpf.Program
	}{
		{
			symbol: "tcp_v4_connect",
			prog:   objs.KprobeTcpV4Connect,
		},
		{
			symbol: "tcp_v6_connect",
			prog:   objs.KprobeTcpV6Connect,
		},
	}

	kretProbes := []struct {
		symbol string
		prog   *ebpf.Program
	}{
		{
			symbol: "tcp_v4_connect",
			prog:   objs.KretprobeTcpV4Connect,
		},
		{
			symbol: "tcp_v6_connect",
			prog:   objs.KretprobeTcpV6Connect,
		},
	}

	toClose := make([]io.Closer, 0)
	for _, kprobe := range kprobes {
		kp, err := link.Kprobe(kprobe.symbol, kprobe.prog, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		toClose = append(toClose, kp)
	}

	for _, kretprobe := range kretProbes {
		kret, err := link.Kretprobe(kretprobe.symbol, kretprobe.prog, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		toClose = append(toClose, kret)
	}

	eventBufV4, err := ringbuf.NewReader(objs.Ipv4Events)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	eventBufV6, err := ringbuf.NewReader(objs.Ipv6Events)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bpfv4Events := make(chan []byte, bufferSize)
	go sendEvents(bpfv4Events, eventBufV4)

	bpfv6Events := make(chan []byte, bufferSize)
	go sendEvents(bpfv6Events, eventBufV6)

	return &conn{
		objs:        &objs,
		event4Chan:  bpfv4Events,
		event6Chan:  bpfv6Events,
		toClose:     toClose,
		lostCounter: lostCtr,
	}, nil
}

func (c *conn) startSession(auditSessionID uint32) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.closed {
		return trace.BadParameter("connection is closed")
	}

	if err := c.objs.MonitoredSessionids.Put(auditSessionID, uint8(0)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *conn) endSession(auditSessionID uint32) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.closed {
		return nil
	}

	if err := c.objs.MonitoredSessionids.Delete(&auditSessionID); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// close will stop reading events off the ring buffer and unload the BPF
// program. The ring buffer is closed as part of the module being closed.
func (c *conn) close() {
	// c.lost.Close()
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.closed {
		return
	}

	c.closed = true

	for _, link := range c.toClose {
		if err := link.Close(); err != nil {
			logger.WarnContext(context.Background(), "failed to close link", "error", err)
		}
	}

	if err := c.objs.Close(); err != nil {
		logger.WarnContext(context.Background(), "failed to close network objects", "error", err)
	}

	if err := c.lostCounter.Close(); err != nil {
		logger.WarnContext(context.Background(), "failed to close network lost counter", "error", err)
	}

	logger.DebugContext(context.Background(), "Closed network BPF module")
}

// v4Events contains raw events off the perf buffer.
func (c *conn) v4Events() <-chan []byte {
	return c.event4Chan
}

// v6Events contains raw events off the perf buffer.
func (c *conn) v6Events() <-chan []byte {
	return c.event6Chan
}
