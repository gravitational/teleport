// +build bpf,linux

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

import "C"

import (
	"context"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	lostNetworkEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricLostNetworkEvents,
			Help: "Number of lost network events.",
		},
	)
)

func init() {
	prometheus.MustRegister(lostNetworkEvents)
}

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
	Command [commMax]byte
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
	Command [commMax]byte
}

type conn struct {
	closeContext context.Context

	// v{4,6}EventCh are the channels upon which the perf buffer places
	// events.
	v4EventCh <-chan []byte
	v6EventCh <-chan []byte

	// v{4,6}LostCh are the channels upon which the perf buffer places lost
	// event count.
	v4LostCh <-chan uint64
	v6LostCh <-chan uint64

	module   *bcc.Module
	perfMaps []*bcc.PerfMap
}

func startConn(closeContext context.Context, pageCount int) (*conn, error) {
	var err error

	e := &conn{
		closeContext: closeContext,
	}

	e.module = bcc.NewModule(connSource, []string{})
	if e.module == nil {
		return nil, trace.BadParameter("failed to load libbcc")
	}

	// Hook IPv4 connection attempts.
	err = attachProbe(e.module, "tcp_v4_connect", "trace_connect_entry")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = attachRetProbe(e.module, "tcp_v4_connect", "trace_connect_v4_return")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Hook IPv6 connection attempts.
	err = attachProbe(e.module, "tcp_v6_connect", "trace_connect_entry")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = attachRetProbe(e.module, "tcp_v6_connect", "trace_connect_v6_return")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Open perf buffer and start processing IPv4 events.
	e.v4EventCh, e.v4LostCh, err = openPerfBuffer(e.module, e.perfMaps, pageCount, "ipv4_events")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Open perf buffer and start processing IPv6 events.
	e.v6EventCh, e.v6LostCh, err = openPerfBuffer(e.module, e.perfMaps, pageCount, "ipv6_events")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Start a loop that will emit lost events to prometheus.
	go e.lostLoop()

	return e, nil
}

// close will stop reading events off the perf buffer and unload the BPF
// program.
func (e *conn) close() {
	for _, perfMap := range e.perfMaps {
		perfMap.Stop()
	}
	e.module.Close()
}

// lostLoop keeps emitting the number of lost events to prometheus.
func (e *conn) lostLoop() {
	for {
		select {
		case n := <-e.v4LostCh:
			log.Debugf("Lost %v IPv4 events.", n)
			lostNetworkEvents.Add(float64(n))
		case n := <-e.v6LostCh:
			log.Debugf("Lost %v IPv6 events.", n)
			lostNetworkEvents.Add(float64(n))
		case <-e.closeContext.Done():
			return
		}
	}
}

// v4Events contains raw events off the perf buffer.
func (e *conn) v4Events() <-chan []byte {
	return e.v4EventCh
}

// v6Events contains raw events off the perf buffer.
func (e *conn) v6Events() <-chan []byte {
	return e.v6EventCh
}

const connSource string = `
#include <uapi/linux/ptrace.h>
#include <net/sock.h>
#include <bcc/proto.h>

BPF_HASH(currsock, u32, struct sock *);

// separate data structs for ipv4 and ipv6
struct ipv4_data_t {
    u64 cgroup;
    u64 ip;
    u32 pid;
    u32 saddr;
    u32 daddr;
    u16 dport;
    char task[TASK_COMM_LEN];
};
BPF_PERF_OUTPUT(ipv4_events);

struct ipv6_data_t {
    u64 cgroup;
    u64 ip;
    u32 pid;
    u32 saddr[4];
    u32 daddr[4];
    u16 dport;
    char task[TASK_COMM_LEN];
};
BPF_PERF_OUTPUT(ipv6_events);

int trace_connect_entry(struct pt_regs *ctx, struct sock *sk)
{
    u32 pid = bpf_get_current_pid_tgid();

    // Stash the sock ptr for lookup on return.
    currsock.update(&pid, &sk);

    return 0;
};

static int trace_connect_return(struct pt_regs *ctx, short ipver)
{
    int ret = PT_REGS_RC(ctx);
    u32 pid = bpf_get_current_pid_tgid();

    struct sock **skpp;
    skpp = currsock.lookup(&pid);
    if (skpp == 0) {
        return 0;   // missed entry
    }

    if (ret != 0) {
        // failed to send SYNC packet, may not have populated
        // socket __sk_common.{skc_rcv_saddr, ...}
        currsock.delete(&pid);
        return 0;
    }

    // pull in details
    struct sock *skp = *skpp;
    u16 dport = skp->__sk_common.skc_dport;

    if (ipver == 4) {
        struct ipv4_data_t data4 = {.pid = pid, .ip = ipver};
        data4.saddr = skp->__sk_common.skc_rcv_saddr;
        data4.daddr = skp->__sk_common.skc_daddr;
        data4.dport = ntohs(dport);
        data4.cgroup = bpf_get_current_cgroup_id();
        bpf_get_current_comm(&data4.task, sizeof(data4.task));
        ipv4_events.perf_submit(ctx, &data4, sizeof(data4));

    } else /* 6 */ {
        struct ipv6_data_t data6 = {.pid = pid, .ip = ipver};

		// Source.
        bpf_probe_read(&data6.saddr[0], sizeof(data6.saddr[0]),
            &skp->__sk_common.skc_v6_rcv_saddr.in6_u.u6_addr32[0]);
        bpf_probe_read(&data6.saddr[1], sizeof(data6.saddr[1]),
            &skp->__sk_common.skc_v6_rcv_saddr.in6_u.u6_addr32[1]);
        bpf_probe_read(&data6.saddr[2], sizeof(data6.saddr[2]),
            &skp->__sk_common.skc_v6_rcv_saddr.in6_u.u6_addr32[2]);
        bpf_probe_read(&data6.saddr[3], sizeof(data6.saddr[3]),
            &skp->__sk_common.skc_v6_rcv_saddr.in6_u.u6_addr32[3]);

		// Destination.
        bpf_probe_read(&data6.daddr[0], sizeof(data6.daddr[0]),
            &skp->__sk_common.skc_v6_daddr.in6_u.u6_addr32[0]);
        bpf_probe_read(&data6.daddr[1], sizeof(data6.daddr[1]),
            &skp->__sk_common.skc_v6_daddr.in6_u.u6_addr32[1]);
        bpf_probe_read(&data6.daddr[2], sizeof(data6.daddr[2]),
            &skp->__sk_common.skc_v6_daddr.in6_u.u6_addr32[2]);
        bpf_probe_read(&data6.daddr[3], sizeof(data6.daddr[3]),
            &skp->__sk_common.skc_v6_daddr.in6_u.u6_addr32[3]);

        data6.dport = ntohs(dport);
        data6.cgroup = bpf_get_current_cgroup_id();
        bpf_get_current_comm(&data6.task, sizeof(data6.task));
        ipv6_events.perf_submit(ctx, &data6, sizeof(data6));
    }

    currsock.delete(&pid);

    return 0;
}

int trace_connect_v4_return(struct pt_regs *ctx)
{
    return trace_connect_return(ctx, 4);
}

int trace_connect_v6_return(struct pt_regs *ctx)
{
    return trace_connect_return(ctx, 6);
}`
