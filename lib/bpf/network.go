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
	"encoding/binary"
	"net"

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

	// SockType is the socket type, either SOCK_STREAM or SOCK_DGRAM.
	SockType uint16

	// SockInode is the inode backing the socket.
	// Used to identify repeated sends by the same socket.
	SockInode uint64
}

func (e *rawConn4Event) SrcIP() net.IP {
	return ipv4HostToIP(e.SrcAddr)
}

func (e *rawConn4Event) DstIP() net.IP {
	return ipv4HostToIP(e.DstAddr)
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

	// SockType is the socket type, either SOCK_STREAM or SOCK_DGRAM.
	SockType uint16

	// SockInode is the inode backing the socket.
	// Used to identify repeated sends by the same socket.
	SockInode uint64
}

func (e *rawConn6Event) SrcIP() net.IP {
	return ipv6HostToIP(e.SrcAddr)
}

func (e *rawConn6Event) DstIP() net.IP {
	return ipv6HostToIP(e.DstAddr)
}

func ipv4HostToIP(addr uint32) net.IP {
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, addr)
	return net.IP(val)
}

func ipv6HostToIP(addr [4]uint32) net.IP {
	val := make([]byte, 16)
	binary.LittleEndian.PutUint32(val[0:], addr[0])
	binary.LittleEndian.PutUint32(val[4:], addr[1])
	binary.LittleEndian.PutUint32(val[8:], addr[2])
	binary.LittleEndian.PutUint32(val[12:], addr[3])
	return net.IP(val)
}

type conn struct {
	session

	event4Buf *RingBuffer
	event6Buf *RingBuffer

	lost *Counter
}

func startConn(bufferSize int, udpEnabled bool) (*conn, error) {
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

	var udpEnabledVal = uint8(0)
	if udpEnabled {
		udpEnabledVal = 1
	}
	if err := c.session.module.InitGlobalVariable("udp_enabled", udpEnabledVal); err != nil {
		return nil, trace.Wrap(err, "setting udp_disable global")
	}

	// Load into the kernel
	if err = c.session.module.BPFLoadObject(); err != nil {
		return nil, trace.Wrap(err)
	}

	for _, name := range []string{
		"tcp_v4_connect",
		"tcp_v6_connect",
		"udp_sendmsg",
		"udpv6_sendmsg",
	} {
		if err = AttachKprobe(c.session.module, name); err != nil {
			return nil, trace.Wrap(err, "%v probe", name)
		}
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
