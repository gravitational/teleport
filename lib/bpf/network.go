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
	lostNetworkEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricLostNetworkEvents,
			Help: "Number of lost network events.",
		},
	)
)

const (
	network4EventsBuffer = "ipv4_events"
	network6EventsBuffer = "ipv6_events"
)

// rawConn4Event is sent by the eBPF program that Teleport pulls off the perf
// buffer.
type rawConn4Event struct {
	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint64

	// Version is the version of TCP (4 or 6).
	Version uint64

	// PID is the process ID.
	PID uint32

	// SrcAddr is the source IP address.
	SrcAddr uint32

	// DstAddr is the destination IP address.
	DstAddr uint32

	// DstPort is the port the connection is being made to.
	DstPort uint16

	// Command is name of the executable making the connection.
	Command [CommMax]byte
}

// rawConn6Event is sent by the eBPF program that Teleport pulls off the perf
// buffer.
type rawConn6Event struct {
	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint64

	// Version is the version of TCP (4 or 6).
	Version uint64

	// PID is the process ID.
	PID uint32

	// SrcAddr is the source IP address.
	SrcAddr [4]uint32

	// DstAddr is the destination IP address.
	DstAddr [4]uint32

	// DstPort is the port the connection is being made to.
	DstPort uint16

	// Command is name of the executable making the connection.
	Command [CommMax]byte
}

type conn struct {
	session

	event4Buf *RingBuffer
	event6Buf *RingBuffer

	lost *Counter
}

func startConn(bufferSize int) (*conn, error) {
	err := metrics.RegisterPrometheusCollectors(lostNetworkEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c := &conn{}

	networkBPF, err := embedFS.ReadFile("bytecode/network.bpf.o")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.session.module, err = libbpfgo.NewModuleFromBuffer(networkBPF, "network")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Resizing the ring buffer must be done here, after the module
	// was created but before it's loaded into the kernel.
	if err = ResizeMap(c.session.module, network4EventsBuffer, uint32(bufferSize*pageSize)); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = ResizeMap(c.session.module, network6EventsBuffer, uint32(bufferSize*pageSize)); err != nil {
		return nil, trace.Wrap(err)
	}

	// Load into the kernel
	if err = c.session.module.BPFLoadObject(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = AttachKprobe(c.session.module, "tcp_v4_connect"); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = AttachKprobe(c.session.module, "tcp_v6_connect"); err != nil {
		return nil, trace.Wrap(err)
	}

	c.event4Buf, err = NewRingBuffer(c.session.module, network4EventsBuffer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.event6Buf, err = NewRingBuffer(c.session.module, network6EventsBuffer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.lost, err = NewCounter(c.session.module, "lost", lostNetworkEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return c, nil
}

// close will stop reading events off the ring buffer and unload the BPF
// program. The ring buffer is closed as part of the module being closed.
func (c *conn) close() {
	c.lost.Close()
	c.event4Buf.Close()
	c.event6Buf.Close()
	c.session.module.Close()
}

// v4Events contains raw events off the perf buffer.
func (c *conn) v4Events() <-chan []byte {
	return c.event4Buf.EventCh
}

// v6Events contains raw events off the perf buffer.
func (c *conn) v6Events() <-chan []byte {
	return c.event6Buf.EventCh
}
